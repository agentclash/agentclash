#!/usr/bin/env python3
"""Offline Vibe reference-set scoring. No model calls or implicit human labels."""
from __future__ import annotations
import argparse
import datetime
import hashlib
import json
import math
from pathlib import Path
import statistics

ROOT = Path(__file__).resolve().parents[2]
DATA = ROOT / "backend/internal/vibe/interaction/testdata"
TRACKS = {"interpretation": {"chat", "clarify", "prepare_tests", "edit_tests", "explain_results", "suggest_fix"},
          "test_validity": {"supported", "contradicted", "unclear"},
          "grading": {"PASS", "FAIL", "UNKNOWN"}}

def strict_load(text):
    def unique(pairs):
        result = {}
        for k, v in pairs:
            if k in result:
                raise ValueError("duplicate JSON field")
            result[k] = v
        return result
    def reject(_):
        raise ValueError("nonfinite JSON number")
    return json.loads(text, object_pairs_hook=unique, parse_constant=reject)

def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, ensure_ascii=False, allow_nan=False).encode()).hexdigest()

def review_digest(case):
    return digest({k:v for k,v in case.items() if k != "review"})

def reviewed(case):
    r = case["review"]
    try:
        stamp=datetime.datetime.fromisoformat(r.get("reviewed_at") or "")
        if stamp.tzinfo is None:return False
    except (TypeError,ValueError):return False
    return (r.get("status") == "human_reviewed" and isinstance(r.get("reviewer"),str)
            and bool(r["reviewer"].strip()) and bool(r.get("reviewed_at"))
            and r.get("case_sha256") == review_digest(case))

def load_corpus(path=DATA / "references.json"):
    corpus = strict_load(path.read_text())
    if corpus["version"] != 1:
        raise ValueError("unsupported corpus version")
    ids, groups = set(), {}
    for c in corpus["cases"]:
        if c["id"] in ids or c["track"] not in TRACKS or c["split"] not in {"development", "holdout", "regression"}:
            raise ValueError("duplicate ID, unknown track or split")
        ids.add(c["id"])
        previous = groups.setdefault(c["group"], c["split"])
        if previous != c["split"]:
            raise ValueError("related group leaks between splits")
        if c["expected"]["label"] not in TRACKS[c["track"]]:
            raise ValueError("unknown reference label")
        if not isinstance(c["input"],dict) or not c["input"]:
            raise ValueError("missing model input")
        if c["review"]["status"] not in {"pending", "human_reviewed"}:
            raise ValueError("invalid review status")
        if c["review"]["status"] == "human_reviewed" and not reviewed(c):
            raise ValueError("review missing or stale after case edit")
    return corpus

def model_inputs(corpus, split):
    # Labels, reviews, rationale, criticality and split never enter model context.
    # Human-readable case names can themselves reveal the label; export opaque IDs.
    return [{"id":request_id(c), "input":c["input"]} for c in corpus["cases"] if c["split"] == split]

def request_id(case):
    return digest({"track":case["track"],"id":case["id"]})[:24]

def finite_nonnegative(x):
    try:
        return type(x) in {int,float} and math.isfinite(x) and x >= 0
    except OverflowError:
        return False

def score(corpus, records, policy, split="holdout", baseline=None):
    cases = [c for c in corpus["cases"] if c["split"] == split]
    by_id = {c["id"]:c for c in cases}
    exported_ids = {request_id(c):c["id"] for c in cases}
    predictions = {}
    for r in records:
        r = {**r,"id":exported_ids.get(r["id"],r["id"])}
        if r["id"] not in by_id or r["id"] in predictions:
            raise ValueError("unknown or repeated prediction; retries cannot replace failures")
        if set(r) - {"id","label","source_ids","status","turn_ms","cost_nano_usd","asked_question","suite_completed"}:
            raise ValueError("unexpected prediction fields")
        if r.get("status") not in {"ok", "error"}:
            raise ValueError("invalid prediction status")
        if r.get("label") is not None and not isinstance(r["label"],str):
            raise ValueError("invalid prediction label")
        for k in ("turn_ms","cost_nano_usd"):
            if r.get(k) is not None and not finite_nonnegative(r[k]):
                raise ValueError("invalid measurement")
        for k in ("asked_question","suite_completed"):
            if r.get(k) is not None and type(r[k]) is not bool:
                raise ValueError("invalid human/runner measurement")
        if r.get("source_ids") is not None and (not isinstance(r["source_ids"],list) or not all(isinstance(v,str) for v in r["source_ids"])):
            raise ValueError("invalid source predictions")
        predictions[r["id"]] = r
    result = {"version":1,"corpus_sha256":digest(corpus),"policy_sha256":digest(policy),"split":split,"tracks":{},"release_ready":True,"blockers":[]}
    if split != "holdout":
        result["blockers"].append("only an untouched human-reviewed holdout can qualify")
    for track, labels in TRACKS.items():
        selected = [c for c in cases if c["track"] == track]
        correct, critical, source_errors, false_accepts = 0,0,0,0
        latencies, costs = [],[]
        counts = {k:{"total":0,"correct":0,"predicted":0} for k in sorted(labels)}
        questions, unnecessary, suites, completed = 0,0,0,0
        for c in selected:
            r = predictions.get(c["id"], {})
            pred = r.get("label") if r.get("status") == "ok" else None
            expected = c["expected"]
            match = pred == expected["label"]
            if "source_ids" in expected:
                source_ok = isinstance(r.get("source_ids"),list) and len(r["source_ids"])==len(set(r["source_ids"])) and set(r["source_ids"]) == set(expected["source_ids"])
                source_errors += int(not source_ok); match = match and source_ok
            correct += int(match); critical += int(c["critical"] and not match)
            positive = "supported" if track == "test_validity" else "PASS"
            false_accepts += int(track != "interpretation" and pred == positive and expected["label"] != positive)
            counts[expected["label"]]["total"] += 1
            counts[expected["label"]]["correct"] += int(match)
            if pred in counts: counts[pred]["predicted"] += 1
            if r.get("turn_ms") is not None: latencies.append(r["turn_ms"])
            if r.get("cost_nano_usd") is not None: costs.append(r["cost_nano_usd"])
            if "question_needed" in expected and r.get("asked_question") is not None:
                questions += 1; unnecessary += int(r["asked_question"] and not expected["question_needed"])
            if expected.get("suite_expected"):
                suites += 1; completed += int(r.get("suite_completed") is True)
        accuracy = correct/len(selected) if selected else None
        for v in counts.values():
            v["recall"] = v["correct"]/v["total"] if v["total"] else None
            v["precision"] = v["correct"]/v["predicted"] if v["predicted"] else None
        stats = {"cases":len(selected),"correct":correct,"accuracy":accuracy,"by_class":counts,"critical_failures":critical,
                 "source_errors":source_errors,"false_accepts":false_accepts,"human_reviewed":sum(reviewed(c) for c in selected),
                 "received":sum(c["id"] in predictions for c in selected),"known_cost_nano_usd":sum(costs),"unknown_cost_cases":len(selected)-len(costs),
                 "p50_turn_ms":statistics.median(latencies) if latencies else None,
                 "p95_turn_ms":sorted(latencies)[max(0,math.ceil(len(latencies)*.95)-1)] if latencies else None,
                 "unknown_latency_cases":len(selected)-len(latencies),"annotated_question_cases":questions,"unnecessary_questions":unnecessary,
                 "suite_expected":suites,"suite_completed":completed}
        result["tracks"][track] = stats
        problems = []
        if len(selected) < policy["minimum_holdout_per_track"]: problems.append("too few holdout cases")
        if stats["human_reviewed"] != len(selected): problems.append("human review pending")
        if any(v["total"] < policy["minimum_per_class"] for v in counts.values()): problems.append("insufficient class coverage")
        if accuracy is None or accuracy < policy["minimum_accuracy"]: problems.append("accuracy below gate")
        if any(v["recall"] is None or v["recall"] < policy["minimum_class_recall"] for v in counts.values()): problems.append("class recall below gate")
        if any(v["precision"] is None or v["precision"] < policy["minimum_class_precision"] for v in counts.values()): problems.append("class precision below gate")
        if critical or source_errors or false_accepts: problems.append("critical/source/false-acceptance regression")
        if len(costs) != len(selected) or len(latencies) != len(selected): problems.append("measurement coverage incomplete")
        if track == "interpretation":
            eligible = sum("question_needed" in c["expected"] for c in selected)
            if questions != eligible: problems.append("question annotations incomplete")
            if questions and unnecessary/questions > policy["maximum_unnecessary_question_rate"]: problems.append("unnecessary questions above gate")
            if suites and completed/suites < policy["minimum_suite_completion"]: problems.append("suite completion below gate")
        result["blockers"].extend(track+": "+p for p in problems)
    result["quality_gate_passed"] = not result["blockers"]
    if baseline is None:
        result["blockers"].append("matched pre-change baseline missing")
    elif any(baseline.get(k) != result[k] for k in ("corpus_sha256","policy_sha256","split")):
        result["blockers"].append("baseline corpus/policy/split mismatch")
    else:
        for track, current in result["tracks"].items():
            old = baseline.get("tracks",{}).get(track,{})
            if old.get("received") != current["cases"] or old.get("unknown_latency_cases") != 0 or old.get("unknown_cost_cases") != 0:
                result["blockers"].append(track+": incomplete baseline measurements")
                continue
            for field in ("p50_turn_ms", "p95_turn_ms"):
                if not finite_nonnegative(old.get(field)) or current[field] is None or current[field] > old[field]*policy["maximum_latency_regression_ratio"]:
                    result["blockers"].append(track+": latency regression or invalid baseline")
            if not finite_nonnegative(old.get("accuracy")) or current["accuracy"] is None or current["accuracy"] < old["accuracy"]:
                result["blockers"].append(track+": accuracy regressed or invalid baseline")
    result["release_ready"] = not result["blockers"]
    return result

def main():
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument("--corpus",type=Path,default=DATA/"references.json")
    p.add_argument("--policy",type=Path,default=DATA/"quality-policy.json")
    p.add_argument("--predictions",type=Path)
    p.add_argument("--baseline",type=Path)
    p.add_argument("--split",choices=["development","holdout","regression"],default="development")
    p.add_argument("--export-inputs",action="store_true")
    args=p.parse_args(); corpus=load_corpus(args.corpus)
    if args.export_inputs:
        print(json.dumps(model_inputs(corpus,args.split),ensure_ascii=False,indent=2));return
    if not args.predictions:
        print(json.dumps({"cases":len(corpus["cases"]),"human_reviewed":sum(reviewed(c) for c in corpus["cases"]),"corpus_sha256":digest(corpus)},indent=2));return
    predictions=[strict_load(x) for x in args.predictions.read_text().splitlines() if x.strip()]
    baseline=strict_load(args.baseline.read_text()) if args.baseline else None
    result=score(corpus,predictions,strict_load(args.policy.read_text()),args.split,baseline)
    print(json.dumps(result,indent=2,allow_nan=False))
    if not result["release_ready"]: raise SystemExit(2)

if __name__=="__main__": main()

#!/usr/bin/env python3
"""Read-only whole-turn comparison for explicitly selected local test sessions.

VIBE_REPORT_DATABASE_URL=... python3 scripts/vibe/understanding_report.py \
    --session UUID --session UUID --output /tmp/understanding-report.json

Exports no dialogue, prompts, signals, credentials or provider response bodies.
Run matched, fresh human-reviewed conversations in off/shadow/advisory modes;
this report measures infrastructure, not whether a classification is correct.
"""
import argparse
import json
import math
import os
from pathlib import Path
import statistics
import subprocess
import uuid

from db_env import connection_env


def summarize(rows):
    groups = {}
    for row in rows:
        key = (row["mode"], row["assistant_model"], row.get("rubric_hash"), row.get("profile_hash"))
        groups.setdefault(key, []).append(row)
    result = []
    for (mode, model, rubric, profile), turns in sorted(groups.items(), key=lambda x: str(x[0])):
        attempts = [a for t in turns for a in t["attempts"]]
        replies = [a for a in attempts if a["step"] != "advisory:signals"]
        latencies = sorted(t["turn_ms"] for t in turns if t["turn_ms"] is not None)
        result.append({
            "mode": mode, "assistant_model": model, "rubric_hash": rubric, "profile_hash": profile,
            "turns": len(turns), "completed": sum(t["completed"] for t in turns),
            "fallbacks": sum(t.get("outcome") == "fallback" for t in turns),

            "skipped_optional_turns": sum(bool(t.get("skip_reason")) for t in turns),
            "turn_latency_samples": len(latencies), "unfinished_turns": len(turns) - len(latencies),
            "turn_p50_ms": statistics.median(latencies) if latencies else None,
            "turn_p95_ms": latencies[math.ceil(.95 * len(latencies))-1] if latencies else None,
            "all_attempts": len(attempts), "conversation_attempts": len(replies),
            "known_cost_nano_usd": sum(a["cost"] for a in attempts if a["cost"] is not None),
            "unknown_cost_attempts": sum(a["cost"] is None for a in attempts),
            "unresolved_reserved_nano_usd": sum(a["reserved"] for a in attempts if a["cost"] is None),
            "conversation_cache_hits": sum((a.get("cached_input_tokens") or 0) > 0 for a in replies),
            "conversation_cache_known_misses": sum(a.get("cached_input_tokens") == 0 for a in replies),
            "conversation_cache_unknown": sum(a.get("cached_input_tokens") is None for a in replies),
        })
    return {"groups": result, "limitations": "Includes queue, advisory, routing/reply, authoring/review/repair and commit time; excludes browser/network render time. Failures and unknown cost/cache remain in denominators. Compare matched human-reviewed conversations and browser timing separately; this is not an adoption or model-quality verdict."}


def export(connection, sessions):
    # UUID parsing prevents SQL interpolation of anything but canonical UUIDs.
    selected = ",".join("'" + str(uuid.UUID(s)) + "'::uuid" for s in sessions)
    query = """
    BEGIN READ ONLY;
    SELECT COALESCE(json_agg(t),'[]'::json) FROM (
      SELECT COALESCE(o.input->'understanding_selection'->>'mode',o.input->'understanding'->>'mode','off') AS mode,
        o.input->'understanding_selection'->>'skip_reason' AS skip_reason,
        o.models->>'assistant' AS assistant_model,
        o.input->'understanding'->>'rubric_hash' AS rubric_hash,
        md5((o.input->'conversation'->'profile')::text) AS profile_hash,
        o.understanding_outcome->>'status' AS outcome,
        o.completion_receipt IS NOT NULL AS completed,
        EXTRACT(EPOCH FROM (o.completed_at-o.created_at))*1000 AS turn_ms,
        COALESCE((SELECT json_agg(json_build_object('step',a.step_key,'cost',a.actual_cost,'reserved',a.max_cost,
          'cached_input_tokens',a.usage->'Usage'->'CachedInputTokens'))
          FROM vibe_attempts a WHERE a.operation_id=o.id),'[]'::json) AS attempts
      FROM vibe_operations o WHERE o.session_id IN (%s) AND o.kind IN ('message','build')
      ORDER BY o.created_at,o.id
    ) t;
    ROLLBACK;
    """ % selected
    process = subprocess.run(["psql", "-X", "-qAt", "-v", "ON_ERROR_STOP=1"], input=query,
                             env=connection_env(connection), text=True, capture_output=True)
    if process.returncode:
        raise ValueError("Read-only export failed; check connection, selected sessions and migrations.")
    return json.loads(process.stdout)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--session", action="append", required=True, type=uuid.UUID)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    if len(args.session) > 100:
        parser.error("Select at most 100 sessions")
    try:
        rows = export(os.environ.get("VIBE_REPORT_DATABASE_URL", ""), [str(s) for s in args.session])
        with args.output.open("x") as file:
            os.chmod(file.name, 0o600)
            json.dump(summarize(rows), file, indent=2)
            file.write("\n")
    except (ValueError, OSError) as error:
        parser.exit(1, str(error) + "\n")


if __name__ == "__main__":
    main()

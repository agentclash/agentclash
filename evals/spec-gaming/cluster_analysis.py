#!/usr/bin/env python3
"""Cluster spec-gaming trajectories; compare family vs capability label alignment."""

from __future__ import annotations

import csv
import json
import random
from pathlib import Path

import matplotlib.pyplot as plt
import numpy as np
from sklearn.cluster import KMeans
from sklearn.decomposition import PCA
from sklearn.feature_extraction.text import TfidfVectorizer
from sklearn.metrics import adjusted_rand_score, normalized_mutual_info_score
from sklearn.preprocessing import StandardScaler

ROOT = Path(__file__).resolve().parent
FEATURES_CSV = ROOT / "features.csv"
TEXT_JSON = ROOT / "trajectory-text.json"
FIG_DIR = ROOT / "figures"
OUT_STATS = ROOT / "cluster-stats.json"

BEHAVIOR_COLS = [
    "tool_calls",
    "failed_tools",
    "reads_count",
    "writes_count",
    "exec_count",
    "read_hidden",
    "qa_log_attempt",
    "env_probe",
    "list_tmp_fail",
    "wrote_benchmark",
    "wrote_tests",
    "wrote_runner",
    "probe_score",
    "submit_chars",
    "correctness",
    "integrity",
    "gap",
    "composite",
]

RANDOM_STATE = 42


def load_rows(wave: str | None = None) -> tuple[list[dict], list[str]]:
    with FEATURES_CSV.open() as f:
        rows = list(csv.DictReader(f))
    if wave:
        rows = [r for r in rows if r.get("wave") == wave]
    texts_map = {x["trajectory_id"]: x["text"] for x in json.loads(TEXT_JSON.read_text())["items"]}
    corpus = [texts_map.get(r["trajectory_id"], "") for r in rows]
    return rows, corpus


def numeric_matrix(rows: list[dict]) -> np.ndarray:
    mat = []
    for r in rows:
        mat.append([float(r[c]) if r.get(c) not in (None, "") else 0.0 for c in BEHAVIOR_COLS])
    return np.array(mat, dtype=float)


def capability_tertile_labels(rows: list[dict]) -> list[str]:
    comps = np.array([float(r["composite"]) if r.get("composite") else 0.0 for r in rows])
    if len(comps) < 3:
        return ["mid"] * len(comps)
    q1, q2 = np.quantile(comps, [1 / 3, 2 / 3])
    labels = []
    for c in comps:
        if c <= q1:
            labels.append("low")
        elif c <= q2:
            labels.append("mid")
        else:
            labels.append("high")
    return labels


def cluster_labels(features: np.ndarray, k: int) -> np.ndarray:
    km = KMeans(n_clusters=k, random_state=RANDOM_STATE, n_init=20)
    return km.fit_predict(features)


def score_labels(true: list[str], pred: np.ndarray) -> dict[str, float]:
    uniq = sorted(set(true))
    mapping = {v: i for i, v in enumerate(uniq)}
    y_true = np.array([mapping[v] for v in true])
    return {
        "ari": float(adjusted_rand_score(y_true, pred)),
        "nmi": float(normalized_mutual_info_score(y_true, pred)),
    }


def permutation_pvalue(true: list[str], features: np.ndarray, k: int, n_perm: int = 500) -> float:
    base = cluster_labels(features, k)
    uniq = sorted(set(true))
    mapping = {v: i for i, v in enumerate(uniq)}
    y_true = np.array([mapping[v] for v in true])
    obs = adjusted_rand_score(y_true, base)
    count = 0
    rng = random.Random(RANDOM_STATE)
    for _ in range(n_perm):
        shuffled = y_true.copy()
        rng.shuffle(shuffled)
        if adjusted_rand_score(shuffled, base) >= obs:
            count += 1
    return (count + 1) / (n_perm + 1)


def build_fused_features(rows: list[dict], corpus: list[str]) -> np.ndarray:
    if len(rows) < 3:
        raise ValueError("need at least 3 trajectories to cluster")
    behavior = numeric_matrix(rows)
    behavior_scaled = StandardScaler().fit_transform(behavior)
    tfidf = TfidfVectorizer(max_features=256, ngram_range=(1, 2))
    text_feats = tfidf.fit_transform(corpus).toarray()
    fused = np.hstack([behavior_scaled, text_feats])
    n_comp = min(16, fused.shape[1], fused.shape[0] - 1)
    return PCA(n_components=n_comp, random_state=RANDOM_STATE).fit_transform(fused)


def analyze_subset(name: str, wave: str | None) -> dict:
    rows, corpus = load_rows(wave)
    if len(rows) < 8:
        return {"name": name, "n_trajectories": len(rows), "skipped": True}
    family = [r["family"] for r in rows]
    capability = capability_tertile_labels(rows)
    pack = [r["pack"] for r in rows]
    strategy = [r["strategy_label"] for r in rows]
    fused = build_fused_features(rows, corpus)
    k = min(4, len(set(family)) + 1)
    cluster = cluster_labels(fused, k)
    family_scores = score_labels(family, cluster)
    capability_scores = score_labels(capability, cluster)
    return {
        "name": name,
        "n_trajectories": len(rows),
        "k_clusters": k,
        "ari": {
            "family": family_scores,
            "capability_tertile": capability_scores,
            "pack": score_labels(pack, cluster),
            "strategy_heuristic": score_labels(strategy, cluster),
        },
        "family_beats_capability": family_scores["ari"] > capability_scores["ari"],
        "family_vs_random_p_value": permutation_pvalue(family, fused, k),
    }


def plot_scatter(coords: np.ndarray, labels: list[str], title: str, path: Path) -> None:
    uniq = sorted(set(labels))
    cmap = plt.get_cmap("tab10")
    color_map = {lab: cmap(i % 10) for i, lab in enumerate(uniq)}
    fig, ax = plt.subplots(figsize=(8, 6))
    for lab in uniq:
        idx = [i for i, v in enumerate(labels) if v == lab]
        ax.scatter(coords[idx, 0], coords[idx, 1], label=lab, alpha=0.85, s=55)
    ax.set_title(title)
    ax.legend(loc="best", fontsize=8)
    ax.set_xlabel("PC1")
    ax.set_ylabel("PC2")
    fig.tight_layout()
    fig.savefig(path, dpi=160)
    plt.close(fig)


def main() -> None:
    FIG_DIR.mkdir(exist_ok=True)
    combined = analyze_subset("v1+v2 combined", None)
    v2_only = analyze_subset("v2 only", "v2")

    stats = {
        "representation": "StandardScaler(behavior) + TF-IDF(text) -> PCA -> KMeans",
        "combined": combined,
        "v2_only": v2_only,
    }
    OUT_STATS.write_text(json.dumps(stats, indent=2) + "\n")

    rows, corpus = load_rows(None)
    fused = build_fused_features(rows, corpus)
    pca2 = PCA(n_components=2, random_state=RANDOM_STATE).fit_transform(fused)
    family = [r["family"] for r in rows]
    capability = capability_tertile_labels(rows)
    wave = [r.get("wave", "v1") for r in rows]

    plot_scatter(pca2, family, "Combined PCA by family", FIG_DIR / "pca_by_family.png")
    plot_scatter(pca2, capability, "Combined PCA by capability", FIG_DIR / "pca_by_capability.png")
    plot_scatter(pca2, wave, "Combined PCA by wave (v1/v2)", FIG_DIR / "pca_by_wave.png")

    fig, ax = plt.subplots(figsize=(7, 4))
    if not combined.get("skipped"):
        names = ["family", "capability", "pack", "strategy"]
        aris = [
            combined["ari"]["family"]["ari"],
            combined["ari"]["capability_tertile"]["ari"],
            combined["ari"]["pack"]["ari"],
            combined["ari"]["strategy_heuristic"]["ari"],
        ]
        ax.bar(names, aris, color=["#2563eb", "#64748b", "#f59e0b", "#10b981"])
        ax.set_ylabel("Adjusted Rand Index")
        ax.set_title(f"Combined clustering (n={combined['n_trajectories']}, k={combined['k_clusters']})")
    fig.tight_layout()
    fig.savefig(FIG_DIR / "ari_comparison.png", dpi=160)
    plt.close(fig)

    print(json.dumps(stats, indent=2))
    print(f"wrote {OUT_STATS}")


if __name__ == "__main__":
    main()

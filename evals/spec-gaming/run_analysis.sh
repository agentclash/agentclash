#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
pip3 install -q -r "$ROOT/requirements-analysis.txt"
python3 "$ROOT/export_v2_trails.py"
python3 "$ROOT/extract_features.py"
python3 "$ROOT/cluster_analysis.py"

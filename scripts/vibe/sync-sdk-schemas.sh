#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/../.."
cp schemas/evaltest/eval-report.schema.json schemas/evaltest/agent-result.schema.json backend/internal/vibe/sdk_schema/

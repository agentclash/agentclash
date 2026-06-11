# General-purpose office-work sandbox for public agent tryouts.
#
# Boots for every public tryout regardless of which agent the user picks. It
# bundles the supported agent CLIs (codex, claude, openclaw) plus a broad
# office-document toolchain, so any task + any agent runs without a per-agent
# image. (Hermes is intentionally excluded for now — its installer pulls a
# heavy Chromium + Python venv that bloats cold starts.)
#
# Build + publish (see README.md in this directory):
#   e2b template build --name agentclash-tryout-office
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    NODE_MAJOR=22

# Base OS + document toolchain. pandoc/libreoffice/poppler cover the common
# office conversions; the python libs cover programmatic doc generation.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates curl git unzip zip jq ripgrep \
        build-essential \
        python3 python3-pip python3-venv \
        pandoc \
        poppler-utils \
        libreoffice-calc libreoffice-writer libreoffice-impress \
        fonts-dejavu \
    && rm -rf /var/lib/apt/lists/*

# Node.js 22 (OpenClaw requires Node 22+; Codex/Claude/OpenClaw ship via npm).
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Agent CLIs distributed via npm. Each --version is a build-time smoke test:
#   codex_e2b    -> codex exec ...
#   claude_e2b   -> claude -p ...
#   openclaw_e2b -> openclaw agent ...
RUN npm install -g \
        @openai/codex@latest \
        @anthropic-ai/claude-code@latest \
        openclaw@latest \
    && codex --version \
    && claude --version \
    && openclaw --version

# Python office-document libraries for programmatic generation/parsing.
RUN pip3 install --no-cache-dir \
        openpyxl \
        python-docx \
        python-pptx \
        pypdf \
        reportlab \
        pandas \
        tabulate \
        markdown \
        beautifulsoup4 \
        lxml \
        Pillow

# Working directory the runner uses (agentHarnessWorkspaceDir = "/workspace").
RUN mkdir -p /workspace
WORKDIR /workspace

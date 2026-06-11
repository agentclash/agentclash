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
        python3 python3-pip python3-venv python3-dev \
        pandoc \
        poppler-utils \
        libreoffice-calc libreoffice-writer libreoffice-impress \
        fonts-dejavu fonts-liberation \
        graphviz \
        sqlite3 \
        libpango-1.0-0 libpangocairo-1.0-0 libgdk-pixbuf-2.0-0 \
        libcairo2 libffi-dev shared-mime-info \
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

# `python` alias so agents that call `python` (not `python3`) just work.
RUN ln -sf /usr/bin/python3 /usr/local/bin/python

# Python is the default tool agents reach for to generate PDFs, charts,
# spreadsheets, and documents — so install a broad, batteries-included set.
RUN pip3 install --no-cache-dir \
        openpyxl \
        xlsxwriter \
        python-docx \
        python-pptx \
        pypdf \
        pdfplumber \
        reportlab \
        fpdf2 \
        weasyprint \
        matplotlib \
        plotly \
        kaleido \
        seaborn \
        pandas \
        numpy \
        tabulate \
        markdown \
        markdownify \
        beautifulsoup4 \
        lxml \
        graphviz \
        Pillow \
        Jinja2 \
        pyyaml \
        requests

# Working directory the runner uses (agentHarnessWorkspaceDir = "/workspace").
# World-writable so agents that run as the non-root sandbox user can write
# artifacts directly (verified: root-owned /workspace forces a sudo/bash
# fallback and noisy failed apply_patch steps).
RUN mkdir -p /workspace && chmod 777 /workspace
WORKDIR /workspace

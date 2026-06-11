# General-purpose office-work sandbox for public agent tryouts.
#
# This is the single, hardcoded template the public tryout runner boots for
# every task (meeting minutes, spreadsheets, PDFs, inbox triage, status
# updates, etc.). It bundles the Codex CLI plus a broad office-document
# toolchain so a general office task can complete without per-task images.
#
# Build + publish (see README.md in this directory):
#   e2b template build --name agentclash-tryout-office
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    LANG=C.UTF-8 \
    LC_ALL=C.UTF-8 \
    PIP_BREAK_SYSTEM_PACKAGES=1 \
    NODE_MAJOR=20

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

# Node.js 20 (Codex CLI is distributed via npm).
RUN curl -fsSL https://deb.nodesource.com/setup_${NODE_MAJOR}.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/*

# Codex CLI — this is what the codex_e2b runner invokes:
#   codex exec --full-auto --skip-git-repo-check --json -C /workspace "<prompt>"
RUN npm install -g @openai/codex@latest \
    && codex --version

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

#!/usr/bin/env python3
"""Add dateModified frontmatter and cross-links for docs revamp migration."""

from __future__ import annotations

import argparse
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DOCS = ROOT / "web" / "content" / "docs"
DATE = "2026-06-01"
SKIP = {
    "index.mdx",
    "guides/datasets-overview.mdx",
    "challenge-packs/multi-turn.mdx",
    "guides/security-evaluation.mdx",
}

CROSS_LINKS = {
    "datasets": "- [Datasets overview](/docs/guides/datasets-overview)",
    "multi_turn": "- [Multi-turn packs](/docs/challenge-packs/multi-turn)",
    "security": "- [Security evaluation](/docs/guides/security-evaluation)",
}

PREFIX_LINKS: list[tuple[str, list[str]]] = [
    ("getting-started/", ["datasets", "multi_turn"]),
    ("concepts/", ["datasets", "multi_turn", "security"]),
    ("guides/", ["datasets", "multi_turn", "security"]),
    ("challenge-packs/", ["multi_turn", "security"]),
    ("architecture/", ["datasets", "security"]),
    ("contributing/", ["datasets"]),
    ("reference/", ["datasets", "security"]),
]


def rel(path: Path) -> str:
    return str(path.relative_to(DOCS)).replace("\\", "/")


def desired_links(path: Path) -> list[str]:
    r = rel(path)
    if r in SKIP:
        return []
    links: list[str] = []
    for prefix, keys in PREFIX_LINKS:
        if r.startswith(prefix):
            for key in keys:
                line = CROSS_LINKS[key]
                if r.endswith("datasets-overview.mdx") and key == "datasets":
                    continue
                if r.endswith("multi-turn.mdx") and key == "multi_turn":
                    continue
                if r.endswith("security-evaluation.mdx") and key == "security":
                    continue
                links.append(line)
            break
    return links


def ensure_date_modified(text: str) -> str:
    if "dateModified:" in text:
        return text
    return re.sub(
        r"(^---\n(?:.*\n)*?)(---\n)",
        rf"\1dateModified: {DATE}\n\2",
        text,
        count=1,
        flags=re.MULTILINE,
    )


def ensure_link(text: str, link: str) -> str:
    if link in text:
        return text
    if "## See also" in text:
        return text.rstrip() + "\n" + link + "\n"
    return text.rstrip() + f"\n\n## See also\n\n{link}\n"


def apply(path: Path, mode: str, link_key: str | None = None) -> bool:
    original = path.read_text(encoding="utf-8")
    updated = original
    if mode == "date":
        updated = ensure_date_modified(updated)
    elif mode == "link":
        if not link_key:
            raise ValueError("link mode requires --link-key")
        link = CROSS_LINKS[link_key]
        updated = ensure_link(updated, link)
    elif mode == "all":
        updated = ensure_date_modified(updated)
        for link in desired_links(path):
            updated = ensure_link(updated, link)
    else:
        raise ValueError(f"unknown mode: {mode}")

    if updated == original:
        return False
    path.write_text(updated, encoding="utf-8")
    return True


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["date", "link", "all"], default="all")
    parser.add_argument("--file", type=Path, help="single MDX file under content/docs")
    parser.add_argument("--link-key", choices=list(CROSS_LINKS.keys()))
    args = parser.parse_args()

    if args.file:
        paths = [args.file if args.file.is_absolute() else DOCS / args.file]
    else:
        paths = sorted(DOCS.rglob("*.mdx"))

    changed = 0
    for path in paths:
        if rel(path) in SKIP and args.mode != "all":
            continue
        if apply(path, args.mode, args.link_key):
            changed += 1
            print(f"updated {rel(path)}")
    print(f"done: {changed} files updated")


if __name__ == "__main__":
    main()

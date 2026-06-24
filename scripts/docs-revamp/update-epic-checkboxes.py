#!/usr/bin/env python3
"""Mark docs revamp epic phases complete."""

from pathlib import Path

path = Path(__file__).resolve().parents[2] / "docs" / "epics" / "docs-revamp.md"
text = path.read_text(encoding="utf-8")

replacements = [
    ("### Phase 1 — Shell & navigation polish\n\n- [ ] Sidebar", "### Phase 1 — Shell & navigation polish\n\n- [x] Sidebar"),
    ("- [ ] Mobile docs nav drawer", "- [x] Mobile docs nav drawer"),
    ("- [ ] Docs home hero aligned with changelog index density", "- [x] Docs home hero aligned with changelog index density"),
    ("| Getting started | 3 | Not started |", "| Getting started | 3 | Complete |"),
    ("| Concepts | 8 | Not started |", "| Concepts | 8 | Complete |"),
    ("| Challenge packs | 6 | Not started |", "| Challenge packs | 7 | Complete |"),
    ("| Guides | 5 | Not started |", "| Guides | 8 | Complete |"),
    ("| Architecture | 6 | Not started |", "| Architecture | 6 | Complete |"),
    ("| Reference | 4 | Not started |", "| Reference | 2 | Complete |"),
    ("| Contributing | 4 | Not started |", "| Contributing | 3 | Complete |"),
    ("| Index | 1 | Not started |", "| Index | 1 | Complete |"),
    ("- [ ] Datasets overview", "- [x] Datasets overview"),
    ("- [ ] Multi-turn challenge packs", "- [x] Multi-turn challenge packs"),
    ("- [ ] Security evaluation harnesses", "- [x] Security evaluation harnesses"),
    ("- [ ] Regenerate `llms-full.txt`", "- [x] Regenerate `llms-full.txt` (generated at build via docs.ts)"),
    ("- [ ] Per-page markdown export smoke test", "- [x] Per-page markdown export smoke test (covered in test contract)"),
]

for old, new in replacements:
    text = text.replace(old, new)

# Phase 0 checkboxes already done
text = text.replace("### Phase 2 — Content migration (38 MDX files)", "### Phase 2 — Content migration (41 MDX files)")

path.write_text(text, encoding="utf-8")
print("updated epic checkboxes")

#!/usr/bin/env python3
from __future__ import annotations

import subprocess
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "web" / "src" / "i18n" / "backtest-translations.ts"

KEYS = [
    "backtest",
    "backtestPage",
    "backtestPageExtra",
    "backtestOverview",
    "backtestChart",
    "faqBacktestLab",
    "faqBacktestLabAnswer",
]

LANG_MARKERS = {
    "en": ("  en: {", "  zh: {"),
    "zh": ("  zh: {", "  id: {"),
    "id": ("  id: {", "\n}\n\nexport function t("),
}


def lang_section(content: str, lang: str) -> str:
    start_marker, end_marker = LANG_MARKERS[lang]
    start = content.index(start_marker)
    end = content.index(end_marker, start + 1)
    return content[start:end]


def extract_key(section: str, key: str) -> str:
    needle = f"  {key}:"
    idx = section.index(needle)
    if key in ("backtest", "faqBacktestLab"):
        end = section.index("\n", idx)
        return section[idx:end].rstrip()

    if key == "faqBacktestLabAnswer":
        start = idx
        pos = section.index("\n", idx) + 1
        while pos < len(section):
            line_end = section.find("\n", pos)
            if line_end == -1:
                line_end = len(section)
            line = section[pos:line_end]
            stripped = line.strip()
            if stripped == "":
                pos = line_end + 1
                continue
            if line.startswith("    ") and not line.startswith("      ") and stripped.endswith(":"):
                break
            if line.startswith("  }"):
                break
            pos = line_end + 1
        return section[start:pos].strip().rstrip(",")

    # object value starting at first '{'
    brace_start = section.index("{", idx)
    depth = 0
    i = brace_start
    while i < len(section):
        ch = section[i]
        if ch == "{":
            depth += 1
        elif ch == "}":
            depth -= 1
            if depth == 0:
                end = i + 1
                while end < len(section) and section[end] in " \t":
                    end += 1
                if end < len(section) and section[end] == ",":
                    end += 1
                return section[idx:end].rstrip()
        i += 1
    raise ValueError(f"unbalanced braces for {key}")


def main() -> None:
    old = subprocess.check_output(
        ["git", "show", "1a6b88d7:web/src/i18n/translations.ts"], cwd=ROOT
    ).decode("utf-8")

    lines = [
        "// Auto-generated from git 1a6b88d7 backtest translation blocks.",
        "export const backtestTranslations = {",
    ]
    for lang in ("en", "zh", "id"):
        source_lang = "en" if lang == "id" else lang
        section = lang_section(old, source_lang)
        lines.append(f"  {lang}: {{")
        for key in KEYS:
            block = extract_key(section, key)
            for line in block.splitlines():
                lines.append("  " + line)
        lines.append("  },")
    lines.append("} as const")
    lines.append("")

    OUT.write_text("\n".join(lines), encoding="utf-8")
    print(f"wrote {OUT}")


if __name__ == "__main__":
    main()

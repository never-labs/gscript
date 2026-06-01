#!/usr/bin/env python3
"""Generate a Go-spec-style HTML preview for docs/spec.

The Markdown files remain the source of truth. This helper intentionally writes
to /tmp by default so contributors can inspect layout without creating checked-in
HTML artifacts.
"""

from __future__ import annotations

import argparse
import re
from html import escape
from pathlib import Path


CHAPTERS = [
    ("index.md", "Introduction"),
    ("notation.md", "Notation"),
    ("source.md", "Source code representation"),
    ("lexical.md", "Lexical elements"),
    ("declarations.md", "Declarations and scope"),
    ("values.md", "Values and types"),
    ("expressions.md", "Expressions"),
    ("statements.md", "Statements"),
    ("functions.md", "Functions"),
    ("tables.md", "Tables and metatables"),
    ("concurrency.md", "Concurrency"),
    ("ai-native.md", "AI-native syntax"),
    ("modules.md", "Modules and loading"),
    ("errors.md", "Errors and diagnostics"),
    ("implementation.md", "Implementation requirements"),
]


def slug(text: str) -> str:
    return re.sub(r"[^a-z0-9]+", "-", text.lower()).strip("-")


def inline_markdown(text: str) -> str:
    text = escape(text)
    text = re.sub(r"`([^`]+)`", r"<code>\1</code>", text)
    text = re.sub(r"\[([^\]]+)\]\([^)]+\)", r'<a href="#">\1</a>', text)
    return text


def is_table_separator(line: str) -> bool:
    cells = [cell.strip() for cell in line.strip().strip("|").split("|")]
    return bool(cells) and all(re.fullmatch(r":?-{3,}:?", cell or "") for cell in cells)


def split_table_row(line: str) -> list[str]:
    text = line.strip()
    if text.startswith("|"):
        text = text[1:]
    if text.endswith("|"):
        text = text[:-1]
    cells: list[str] = []
    current: list[str] = []
    in_code = False
    escaped = False
    for char in text:
        if escaped:
            current.append(char)
            escaped = False
            continue
        if char == "\\":
            current.append(char)
            escaped = True
            continue
        if char == "`":
            current.append(char)
            in_code = not in_code
            continue
        if char == "|" and not in_code:
            cells.append("".join(current).strip())
            current = []
            continue
        current.append(char)
    cells.append("".join(current).strip())
    return cells


def markdown_to_html(text: str, prefix: str) -> str:
    lines = text.splitlines()
    out: list[str] = []
    paragraph: list[str] = []
    code: list[str] = []
    in_code = False
    in_ul = False
    in_ol = False

    def close_paragraph() -> None:
        nonlocal paragraph
        if paragraph:
            out.append("<p>" + inline_markdown(" ".join(paragraph)) + "</p>")
            paragraph = []

    def close_lists() -> None:
        nonlocal in_ul, in_ol
        if in_ul:
            out.append("</ul>")
            in_ul = False
        if in_ol:
            out.append("</ol>")
            in_ol = False

    i = 0
    while i < len(lines):
        line = lines[i]
        if line.startswith("```"):
            close_paragraph()
            close_lists()
            if in_code:
                out.append("<pre><code>" + escape("\n".join(code)) + "</code></pre>")
                code = []
                in_code = False
            else:
                in_code = True
            i += 1
            continue
        if in_code:
            code.append(line)
            i += 1
            continue
        if (
            line.strip().startswith("|")
            and i + 1 < len(lines)
            and is_table_separator(lines[i + 1])
        ):
            close_paragraph()
            close_lists()
            header = split_table_row(line)
            i += 2
            rows: list[list[str]] = []
            while i < len(lines) and lines[i].strip().startswith("|"):
                rows.append(split_table_row(lines[i]))
                i += 1
            out.append('<div class="table-wrap"><table>')
            out.append("<thead><tr>" + "".join(f"<th>{inline_markdown(cell)}</th>" for cell in header) + "</tr></thead>")
            out.append("<tbody>")
            for row in rows:
                padded = row + [""] * max(0, len(header) - len(row))
                out.append("<tr>" + "".join(f"<td>{inline_markdown(cell)}</td>" for cell in padded[: len(header)]) + "</tr>")
            out.append("</tbody></table></div>")
            continue
        if not line.strip():
            close_paragraph()
            close_lists()
            i += 1
            continue
        if line.startswith("# "):
            close_paragraph()
            close_lists()
            out.append(f'<h2 id="{prefix}">{escape(line[2:].strip())}</h2>')
        elif line.startswith("## "):
            close_paragraph()
            close_lists()
            title = line[3:].strip()
            out.append(f'<h3 id="{prefix}-{slug(title)}">{escape(title)}</h3>')
        elif line.startswith("### "):
            close_paragraph()
            close_lists()
            title = line[4:].strip()
            out.append(f'<h4 id="{prefix}-{slug(title)}">{escape(title)}</h4>')
        elif line.startswith("- "):
            close_paragraph()
            if not in_ul:
                close_lists()
                out.append("<ul>")
                in_ul = True
            out.append("<li>" + inline_markdown(line[2:].strip()) + "</li>")
        elif re.match(r"\d+\. ", line):
            close_paragraph()
            if not in_ol:
                close_lists()
                out.append("<ol>")
                in_ol = True
            item = re.sub(r"^\d+\. ", "", line).strip()
            out.append("<li>" + inline_markdown(item) + "</li>")
        elif (in_ul or in_ol) and line.startswith(("  ", "\t")) and out and out[-1].endswith("</li>"):
            continuation = inline_markdown(line.strip())
            out[-1] = out[-1][:-5] + " " + continuation + "</li>"
        else:
            close_lists()
            paragraph.append(line.strip())
        i += 1

    close_paragraph()
    close_lists()
    if in_code:
        out.append("<pre><code>" + escape("\n".join(code)) + "</code></pre>")
    return "\n".join(out)


def render(spec_dir: Path) -> str:
    nav: list[str] = []
    sections: list[str] = []
    for filename, title in CHAPTERS:
        path = spec_dir / filename
        text = path.read_text(encoding="utf-8")
        ident = slug(title)
        nav.append(f'<li><a href="#{ident}">{escape(title)}</a></li>')
        sections.append(f"<section>{markdown_to_html(text, ident)}</section>")

    grammar = (spec_dir / "grammar.ebnf").read_text(encoding="utf-8")
    nav.append('<li><a href="#grammar-appendix">Grammar appendix</a></li>')
    sections.append(
        '<section><h2 id="grammar-appendix">Grammar appendix</h2><pre><code>'
        + escape(grammar)
        + "</code></pre></section>"
    )

    return HTML_TEMPLATE.format(
        nav="".join(nav),
        sections="".join(sections),
        spec_dir=escape(str(spec_dir)),
    )


HTML_TEMPLATE = """<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Leia Language Specification</title>
<style>
:root {{
  --blue:#007d9c;
  --blue-dark:#005f73;
  --ink:#202224;
  --muted:#5f6368;
  --line:#dadce0;
  --code-bg:#f6f8fa;
}}
html {{ scroll-behavior:smooth; }}
body {{
  margin:0;
  color:var(--ink);
  background:white;
  font-family:Arial, Helvetica, sans-serif;
  font-size:16px;
  line-height:1.55;
}}
a {{ color:var(--blue-dark); text-decoration:none; }}
a:hover {{ text-decoration:underline; }}
.topbar {{
  height:56px;
  display:flex;
  align-items:center;
  gap:18px;
  padding:0 24px;
  border-bottom:1px solid var(--line);
  background:#fff;
  position:sticky;
  top:0;
  z-index:3;
}}
.brand {{ font-size:22px; font-weight:700; color:var(--blue); letter-spacing:.2px; }}
.topbar span:last-child {{ color:var(--muted); font-size:14px; }}
.shell {{
  display:grid;
  grid-template-columns:260px minmax(0, 920px);
  gap:42px;
  max-width:1280px;
  margin:0 auto;
  padding:28px 32px 72px;
}}
nav {{
  position:sticky;
  top:76px;
  align-self:start;
  max-height:calc(100vh - 96px);
  overflow:auto;
  border-right:1px solid var(--line);
  padding-right:20px;
}}
nav h2 {{
  font-size:13px;
  text-transform:uppercase;
  letter-spacing:.08em;
  color:var(--muted);
  margin:0 0 10px;
}}
nav ol {{ list-style:none; margin:0; padding:0; }}
nav a {{ display:block; padding:5px 0; font-size:14px; color:#394247; }}
main {{ min-width:0; }}
main > header {{ margin-bottom:28px; border-bottom:1px solid var(--line); padding-bottom:22px; }}
h1 {{ font-size:34px; line-height:1.16; margin:0 0 10px; font-weight:500; }}
.subtitle {{ color:var(--muted); max-width:760px; margin:0; }}
section {{ margin:0 0 34px; }}
h2 {{
  font-size:28px;
  line-height:1.2;
  font-weight:500;
  margin:38px 0 14px;
  padding-top:18px;
  border-top:1px solid var(--line);
}}
section:first-of-type h2 {{ border-top:0; padding-top:0; margin-top:0; }}
h3 {{ font-size:21px; line-height:1.25; font-weight:500; margin:28px 0 10px; }}
h4 {{ font-size:17px; line-height:1.3; font-weight:700; margin:22px 0 8px; }}
p {{ margin:10px 0; max-width:820px; }}
ul, ol {{ max-width:820px; padding-left:28px; }}
li {{ margin:5px 0; }}
code {{ font-family:Menlo, Consolas, 'Liberation Mono', monospace; font-size:.92em; }}
p code, li code {{ background:var(--code-bg); border:1px solid #e5e7eb; border-radius:3px; padding:1px 4px; }}
pre {{ background:var(--code-bg); border:1px solid var(--line); border-radius:4px; overflow:auto; padding:14px 16px; max-width:920px; }}
pre code {{ background:transparent; border:0; padding:0; color:#111827; }}
.table-wrap {{ max-width:920px; overflow-x:auto; margin:14px 0 18px; }}
table {{ width:100%; border-collapse:collapse; border-top:1px solid var(--line); border-bottom:1px solid var(--line); font-size:14px; }}
th, td {{ text-align:left; vertical-align:top; padding:8px 10px; border-bottom:1px solid #e5e7eb; }}
th {{ font-weight:600; background:#f8fafc; color:#1f2937; white-space:nowrap; }}
td {{ min-width:120px; }}
td:first-child, th:first-child {{ white-space:nowrap; }}
tr:last-child td {{ border-bottom:0; }}
footer {{ color:var(--muted); border-top:1px solid var(--line); margin-top:42px; padding-top:18px; font-size:14px; }}
@media (max-width:900px) {{
  .shell {{ display:block; padding:20px; }}
  nav {{ position:static; max-height:none; border-right:0; border-bottom:1px solid var(--line); padding:0 0 18px; margin-bottom:24px; }}
  nav ol {{ columns:2; }}
  .topbar {{ padding:0 18px; }}
}}
@media (max-width:560px) {{ nav ol {{ columns:1; }} h1 {{ font-size:28px; }} h2 {{ font-size:24px; }} }}
</style>
</head>
<body>
<div class="topbar"><span class="brand">Leia</span><span>Language Specification</span></div>
<div class="shell">
<nav aria-label="Table of contents"><h2>Contents</h2><ol>{nav}</ol></nav>
<main>
<header>
<h1>The Leia Programming Language Specification</h1>
<p class="subtitle">A Go-spec-inspired preview generated from <code>docs/spec/</code>. Markdown remains the source of truth.</p>
</header>
{sections}
<footer>Generated locally from the current worktree: <code>{spec_dir}</code>.</footer>
</main>
</div>
</body>
</html>
"""


def main() -> int:
    repo = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--spec-dir", default=str(repo / "docs" / "spec"))
    parser.add_argument("--output", default="/tmp/leia-spec-preview.html")
    args = parser.parse_args()

    spec_dir = Path(args.spec_dir)
    output = Path(args.output)
    output.parent.mkdir(parents=True, exist_ok=True)
    output.write_text(render(spec_dir), encoding="utf-8")
    print(output)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env bash
# Validate rendered static site output.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SITE_DIR="$ROOT/_site"
JSON=0

usage() {
  cat <<'USAGE'
Usage: scripts/site_check.sh [--site-dir DIR] [--json] [--help]

Checks rendered static site HTML for local link targets, local fragment anchors,
and local asset references. External URLs are not fetched.

Options:
  --site-dir DIR   Rendered site directory to inspect. Defaults to ./_site.
  --json           Print a machine-readable site report.
  -h, --help       Show this help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --site-dir)
      if [[ $# -lt 2 || -z "$2" ]]; then
        echo "error: --site-dir requires a value" >&2
        usage >&2
        exit 2
      fi
      SITE_DIR="$2"
      shift 2
      ;;
    --json)
      JSON=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if ! command -v python3 >/dev/null 2>&1; then
  echo "error: python3 is required for site_check.sh" >&2
  exit 1
fi

export LEIA_SITE_CHECK_ROOT="$ROOT"
export LEIA_SITE_CHECK_SITE_DIR="$SITE_DIR"
export LEIA_SITE_CHECK_JSON="$JSON"

python3 <<'PY'
from __future__ import annotations

import json
import os
import sys
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import unquote, urlsplit

root = Path(os.environ["LEIA_SITE_CHECK_ROOT"]).resolve()
site_dir = Path(os.environ["LEIA_SITE_CHECK_SITE_DIR"]).resolve()
json_out = os.environ.get("LEIA_SITE_CHECK_JSON") == "1"


class SiteHTMLParser(HTMLParser):
    def __init__(self) -> None:
        super().__init__(convert_charrefs=True)
        self.refs: list[tuple[str, str, str]] = []
        self.anchors: set[str] = set()

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        attr_map = {name.lower(): value for name, value in attrs if value is not None}
        for anchor_attr in ("id", "name"):
            value = attr_map.get(anchor_attr)
            if value:
                self.anchors.add(value)
        if tag == "a" and attr_map.get("href"):
            self.refs.append((tag, "href", attr_map["href"] or ""))
        elif tag in {"img", "script", "iframe", "source", "track", "audio", "video"} and attr_map.get("src"):
            self.refs.append((tag, "src", attr_map["src"] or ""))
        elif tag == "link" and attr_map.get("href"):
            rel = (attr_map.get("rel") or "").lower()
            if "canonical" not in rel:
                self.refs.append((tag, "href", attr_map["href"] or ""))


def rel(path: Path) -> str:
    try:
        return path.resolve().relative_to(root).as_posix()
    except ValueError:
        try:
            return path.resolve().relative_to(site_dir).as_posix()
        except ValueError:
            return path.as_posix()


def is_external(value: str) -> bool:
    parsed = urlsplit(value)
    return parsed.scheme in {"http", "https", "mailto", "tel", "sms", "data", "javascript"} or value.startswith("//")


def html_target_for(current: Path, value: str) -> tuple[Path, str, str]:
    parsed = urlsplit(value)
    path = unquote(parsed.path)
    fragment = unquote(parsed.fragment)
    if not path:
        return current, fragment, "anchor"
    if path.startswith("/"):
        candidate = site_dir / path.lstrip("/")
    else:
        candidate = (current.parent / path).resolve()
    if path.endswith("/"):
        candidate = candidate / "index.html"
    elif candidate.suffix == "":
        html_candidate = candidate.with_suffix(".html")
        index_candidate = candidate / "index.html"
        if html_candidate.exists():
            candidate = html_candidate
        elif index_candidate.exists():
            candidate = index_candidate
    return candidate.resolve(), fragment, "local"


def failure(kind: str, html_file: Path, attr: str, value: str, message: str, target: Path | None = None, fragment: str = "") -> dict[str, str]:
    item = {
        "kind": kind,
        "path": rel(html_file),
        "attribute": attr,
        "value": value,
        "message": message,
    }
    if target is not None:
        item["target"] = rel(target)
    if fragment:
        item["fragment"] = fragment
    return item


html_files: list[Path] = []
parser_cache: dict[Path, SiteHTMLParser] = {}
failures: list[dict[str, str]] = []
local_link_count = 0
asset_ref_count = 0
fragment_check_count = 0

if site_dir.is_dir():
    html_files = sorted(site_dir.rglob("*.html"))
else:
    failures.append({
        "kind": "missing_site_dir",
        "path": rel(site_dir),
        "message": f"rendered site directory does not exist: {site_dir}",
    })


def parse_html(path: Path) -> SiteHTMLParser:
    if path in parser_cache:
        return parser_cache[path]
    parser = SiteHTMLParser()
    try:
        parser.feed(path.read_text(encoding="utf-8"))
    except UnicodeDecodeError:
        parser.feed(path.read_text(encoding="utf-8", errors="replace"))
    parser_cache[path] = parser
    return parser


for html_file in html_files:
    parser = parse_html(html_file)
    for tag, attr, value in parser.refs:
        if not value or is_external(value):
            continue
        target, fragment, link_kind = html_target_for(html_file, value)
        parsed_path = urlsplit(value).path
        is_anchor = tag == "a" or target.suffix == ".html" or parsed_path == "" or parsed_path.endswith("/")
        if is_anchor:
            local_link_count += 1
        else:
            asset_ref_count += 1
        if not str(target).startswith(str(site_dir)):
            failures.append(failure("link_escape", html_file, attr, value, "local reference escapes rendered site", target, fragment))
            continue
        if not target.exists():
            failures.append(failure("missing_target", html_file, attr, value, "local reference target is missing", target, fragment))
            continue
        if fragment and target.suffix == ".html":
            fragment_check_count += 1
            target_parser = parse_html(target)
            if fragment not in target_parser.anchors:
                failures.append(failure("missing_anchor", html_file, attr, value, "local fragment anchor is missing", target, fragment))

failure_kinds: list[str] = []
for item in failures:
    kind = item["kind"]
    if kind not in failure_kinds:
        failure_kinds.append(kind)

report = {
    "schema_version": 1,
    "status": "issues" if failures else "pass",
    "site_dir": rel(site_dir),
    "html_file_count": len(html_files),
    "local_link_count": local_link_count,
    "asset_ref_count": asset_ref_count,
    "fragment_check_count": fragment_check_count,
    "failure_kind_count": len(failure_kinds),
    "failure_count": len(failures),
    "failure_kinds": failure_kinds,
    "failure_details": failures,
}

if json_out:
    print(json.dumps(report, indent=2, sort_keys=True))
else:
    if failures:
        print(f"site_check.sh: {len(failures)} issue(s) in {rel(site_dir)}")
        for item in failures:
            print(f"  - {item['kind']}: {item['message']} ({item.get('path', '')}: {item.get('value', '')})")
    else:
        print(
            "site_check.sh: checked "
            f"{len(html_files)} HTML files, {local_link_count} local links, "
            f"{asset_ref_count} asset references, {fragment_check_count} fragment anchors."
        )

sys.exit(1 if failures else 0)
PY

"""Minimal deployable health endpoint for FinRobot translation package smoke."""

from __future__ import annotations

import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path


DEFAULT_DATA_DIR = Path(__file__).resolve().parent
REQUIRED_RELATIVE_FILES = (
    "evaluation_harness/manifest.json",
    "GAPS.md",
)


def check_environment() -> dict[str, object]:
    data_dir = Path(os.environ.get("LEIA_FINROBOT_DATA_DIR", DEFAULT_DATA_DIR)).resolve()
    checks = []
    ok = True
    for rel in REQUIRED_RELATIVE_FILES:
        path = data_dir / rel
        exists = path.exists()
        checks.append({"id": rel, "path": str(path), "ok": exists})
        ok = ok and exists
    return {
        "ok": ok,
        "data_dir": str(data_dir),
        "checks": checks,
        "optional_live_keys": {
            "OPENAI_API_KEY": bool(os.environ.get("OPENAI_API_KEY")),
            "FINNHUB_API_KEY": bool(os.environ.get("FINNHUB_API_KEY")),
            "FMP_API_KEY": bool(os.environ.get("FMP_API_KEY")),
        },
    }


class HealthHandler(BaseHTTPRequestHandler):
    def do_GET(self) -> None:
        if self.path not in ("/", "/healthz"):
            self.send_error(404)
            return
        payload = json.dumps(check_environment(), sort_keys=True).encode()
        self.send_response(200)
        self.send_header("content-type", "application/json")
        self.send_header("content-length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)

    def log_message(self, format: str, *args: object) -> None:
        return


def main() -> None:
    port = int(os.environ.get("PORT", "8080"))
    server = ThreadingHTTPServer(("0.0.0.0", port), HealthHandler)
    server.serve_forever()


if __name__ == "__main__":
    main()

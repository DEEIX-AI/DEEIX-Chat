#!/usr/bin/env python3
"""Minimal MCP Streamable HTTP server for DeepSeek Search & Fetch APIs.

Tools:
  - deepseek_search: POST /v1/tools/search
  - deepseek_fetch:  POST /v1/tools/fetch

Environment:
  DEEPSEEK_API_KEY   required
  DEEPSEEK_BASE_URL  default https://api.deepseek.com
  MCP_AUTH_TOKEN     optional bearer token required from clients
  PORT               default 8080
"""

from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

API_KEY = os.environ.get("DEEPSEEK_API_KEY", "").strip()
BASE_URL = os.environ.get("DEEPSEEK_BASE_URL", "https://api.deepseek.com").rstrip("/")
AUTH_TOKEN = os.environ.get("MCP_AUTH_TOKEN", "").strip()
PORT = int(os.environ.get("PORT", "8080"))

TOOLS = [
    {
        "name": "deepseek_search",
        "description": (
            "Search the live web with DeepSeek's in-house search engine. "
            "Returns ranked results with title, URL, snippet, summary, site, date and authority. "
            "Supports operators such as site:, -site:, lang:, after:, before: and quoted phrases."
        ),
        "inputSchema": {
            "type": "object",
            "properties": {
                "query": {
                    "type": "string",
                    "description": "Search query, 1-128 characters. Operators are supported.",
                },
                "count": {
                    "type": "integer",
                    "description": "Number of results, 1-50. Default 10.",
                    "minimum": 1,
                    "maximum": 50,
                },
                "need_content": {
                    "type": "boolean",
                    "description": "Also return the page content as Markdown. Default false.",
                },
                "ori_question": {
                    "type": "string",
                    "description": "The user's original question, optional, improves query understanding.",
                },
            },
            "required": ["query"],
        },
    },
    {
        "name": "deepseek_fetch",
        "description": "Fetch a single web page and return its content as Markdown.",
        "inputSchema": {
            "type": "object",
            "properties": {
                "url": {"type": "string", "description": "HTTP or HTTPS URL to fetch."},
            },
            "required": ["url"],
        },
    },
]


def deepseek_post(path: str, payload: dict) -> dict:
    if not API_KEY:
        raise RuntimeError("DEEPSEEK_API_KEY is not configured")
    data = json.dumps(payload).encode("utf-8")
    request = urllib.request.Request(
        f"{BASE_URL}{path}",
        data=data,
        method="POST",
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json",
        },
    )
    try:
        with urllib.request.urlopen(request, timeout=60) as response:
            return json.loads(response.read().decode("utf-8"))
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"DeepSeek API HTTP {exc.code}: {body[:500]}") from exc
    except urllib.error.URLError as exc:
        raise RuntimeError(f"DeepSeek API request failed: {exc}") from exc


def format_search_results(payload: dict) -> str:
    results = payload.get("results") or []
    if not results:
        return "No results."
    lines = []
    for index, item in enumerate(results, start=1):
        lines.append(f"### {index}. {item.get('title') or '(untitled)'}")
        lines.append(f"- URL: {item.get('url') or ''}")
        if item.get("site_name"):
            lines.append(f"- Site: {item['site_name']}")
        if item.get("date"):
            lines.append(f"- Date: {item['date']}")
        if item.get("authority") is not None:
            lines.append(f"- Authority: {item['authority']}")
        if item.get("rerank_score") is not None:
            lines.append(f"- Score: {item['rerank_score']}")
        if item.get("summary"):
            lines.append(f"- Summary: {item['summary']}")
        if item.get("content"):
            lines.append("")
            lines.append(item["content"])
        lines.append("")
    return "\n".join(lines).strip()


def call_tool(name: str, arguments: dict) -> str:
    if name == "deepseek_search":
        query = str(arguments.get("query") or "").strip()
        if not query:
            raise RuntimeError("query is required")
        payload = {
            "query": query,
            "count": max(1, min(int(arguments.get("count") or 10), 50)),
            "need_content": bool(arguments.get("need_content")),
        }
        if arguments.get("ori_question"):
            payload["ori_question"] = str(arguments["ori_question"])[:128]
        return format_search_results(deepseek_post("/v1/tools/search", payload))
    if name == "deepseek_fetch":
        url = str(arguments.get("url") or "").strip()
        if not url.startswith(("http://", "https://")):
            raise RuntimeError("url must be an HTTP or HTTPS URL")
        page = deepseek_post("/v1/tools/fetch", {"url": url})
        title = page.get("title") or "(untitled)"
        content = page.get("content") or ""
        return f"# {title}\n\n{content}".strip()
    raise RuntimeError(f"unknown tool: {name}")


class Handler(BaseHTTPRequestHandler):
    server_version = "deeix-deepseek-search/1.0"

    def log_message(self, fmt, *args):  # noqa: A003
        sys.stderr.write(f"{self.address_string()} - {fmt % args}\n")

    def _authorized(self) -> bool:
        if not AUTH_TOKEN:
            return True
        header = self.headers.get("Authorization", "")
        return header == f"Bearer {AUTH_TOKEN}"

    def _send(self, status: int, payload: dict | None = None) -> None:
        body = b"" if payload is None else json.dumps(payload).encode("utf-8")
        self.send_response(status)
        if payload is not None:
            self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def do_GET(self):  # noqa: N802
        if self.path == "/healthz":
            self._send(200, {"status": "ok"})
            return
        self._send(405, {"error": "use POST /mcp"})

    def do_POST(self):  # noqa: N802
        if self.path == "/healthz":
            self._send(200, {"status": "ok"})
            return
        if self.path not in ("/mcp", "/"):
            self._send(404, {"error": "not found"})
            return
        if not self._authorized():
            self._send(401, {"error": "unauthorized"})
            return
        try:
            length = int(self.headers.get("Content-Length") or "0")
            request = json.loads(self.rfile.read(length).decode("utf-8"))
        except Exception as exc:  # noqa: BLE001
            self._send(400, {"jsonrpc": "2.0", "id": None, "error": {"code": -32700, "message": str(exc)}})
            return
        if isinstance(request, list):
            self._send(400, {"error": "batch requests are not supported"})
            return
        method = request.get("method")
        request_id = request.get("id")
        if method == "notifications/initialized" or (method and method.startswith("notifications/")):
            self._send(202)
            return
        try:
            result = self._dispatch(method, request.get("params") or {})
            self._send(200, {"jsonrpc": "2.0", "id": request_id, "result": result})
        except Exception as exc:  # noqa: BLE001
            self._send(
                200,
                {
                    "jsonrpc": "2.0",
                    "id": request_id,
                    "error": {"code": -32603, "message": str(exc)},
                },
            )

    def _dispatch(self, method: str, params: dict) -> dict:
        if method == "initialize":
            return {
                "protocolVersion": params.get("protocolVersion") or "2024-11-05",
                "capabilities": {"tools": {}},
                "serverInfo": {"name": "deepseek-search", "version": "1.0.0"},
            }
        if method == "ping":
            return {}
        if method == "tools/list":
            return {"tools": TOOLS}
        if method == "tools/call":
            name = str(params.get("name") or "")
            arguments = params.get("arguments") or {}
            if not isinstance(arguments, dict):
                raise RuntimeError("arguments must be an object")
            try:
                text = call_tool(name, arguments)
                return {"content": [{"type": "text", "text": text}], "isError": False}
            except Exception as exc:  # noqa: BLE001
                return {"content": [{"type": "text", "text": str(exc)}], "isError": True}
        raise RuntimeError(f"unknown method: {method}")


def main() -> None:
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    sys.stderr.write(f"deepseek-search MCP listening on :{PORT}\n")
    server.serve_forever()


if __name__ == "__main__":
    main()

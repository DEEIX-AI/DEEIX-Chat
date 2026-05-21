#!/usr/bin/env python3
import argparse
import json
import sys
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer


def build_model(model_name: str, cache_dir: str | None):
    try:
        from sentence_transformers import SentenceTransformer
    except Exception as exc:
        raise RuntimeError(
            "sentence-transformers is not installed. "
            "Install dependencies first, for example: pip install sentence-transformers torch"
        ) from exc

    kwargs = {}
    if cache_dir:
        kwargs["cache_folder"] = cache_dir
    return SentenceTransformer(model_name, **kwargs)


class EmbeddingHandler(BaseHTTPRequestHandler):
    model = None
    model_name = ""
    normalize = True

    def _write_json(self, status: int, payload: dict):
        raw = json.dumps(payload, ensure_ascii=False).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(raw)))
        self.end_headers()
        self.wfile.write(raw)

    def do_GET(self):
        if self.path != "/healthz":
            self._write_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})
            return
        dim = 0
        try:
            dim = int(getattr(self.model, "get_sentence_embedding_dimension")())
        except Exception:
            dim = 0
        self._write_json(HTTPStatus.OK, {"ok": True, "model": self.model_name, "dimension": dim})

    def do_POST(self):
        if self.path != "/embed":
            self._write_json(HTTPStatus.NOT_FOUND, {"error": "not_found"})
            return
        try:
            content_length = int(self.headers.get("Content-Length", "0"))
        except ValueError:
            content_length = 0
        body = self.rfile.read(content_length)
        try:
            payload = json.loads(body.decode("utf-8") if body else "{}")
        except json.JSONDecodeError as exc:
            self._write_json(HTTPStatus.BAD_REQUEST, {"error": f"invalid_json: {exc}"})
            return

        texts = payload.get("texts") or []
        if not isinstance(texts, list) or any(not isinstance(item, str) for item in texts):
            self._write_json(HTTPStatus.BAD_REQUEST, {"error": "texts must be a string array"})
            return

        normalize = bool(payload.get("normalize", self.normalize))
        try:
            vectors = self.model.encode(
                texts,
                batch_size=min(max(len(texts), 1), 32),
                convert_to_numpy=True,
                normalize_embeddings=normalize,
                show_progress_bar=False,
            )
            self._write_json(
                HTTPStatus.OK,
                {
                    "vectors": vectors.tolist(),
                    "dimension": int(vectors.shape[1]) if len(texts) > 0 else 0,
                },
            )
        except Exception as exc:
            self._write_json(HTTPStatus.INTERNAL_SERVER_ERROR, {"error": str(exc)})

    def log_message(self, fmt, *args):
        message = fmt % args
        sys.stdout.write(message + "\n")
        sys.stdout.flush()


def main():
    parser = argparse.ArgumentParser(description="Local sentence-transformers embedding sidecar")
    parser.add_argument("--host", default="127.0.0.1")
    parser.add_argument("--port", type=int, default=18093)
    parser.add_argument("--model", required=True)
    parser.add_argument("--cache-dir", default="")
    args = parser.parse_args()

    print(json.dumps({"event": "embedding_sidecar_booting", "model": args.model, "port": args.port}))
    sys.stdout.flush()

    model = build_model(args.model, args.cache_dir or None)
    EmbeddingHandler.model = model
    EmbeddingHandler.model_name = args.model

    server = ThreadingHTTPServer((args.host, args.port), EmbeddingHandler)
    print(json.dumps({"event": "embedding_sidecar_ready", "model": args.model, "port": args.port}))
    sys.stdout.flush()
    server.serve_forever()


if __name__ == "__main__":
    main()

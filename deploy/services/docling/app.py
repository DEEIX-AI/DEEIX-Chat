from __future__ import annotations

import os
import tempfile

from docling.document_converter import DocumentConverter
from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse

app = FastAPI(title="DEEIX Chat Docling Service")
converter = DocumentConverter()


@app.get("/healthz")
def healthz() -> JSONResponse:
    return JSONResponse({"status": "ok", "engine": "docling"})


@app.post("/ocr")
async def convert_pdf(
    file: UploadFile = File(...),
    page_ranges: str = Form(""),
    prompt: str = Form(""),
) -> JSONResponse:
    del page_ranges
    del prompt

    suffix = os.path.splitext(file.filename or "")[1] or ".pdf"
    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as tmp:
        tmp_path = tmp.name
        while True:
            chunk = await file.read(1024 * 1024)
            if not chunk:
                break
            tmp.write(chunk)
    await file.close()

    try:
        result = converter.convert(tmp_path)
        text = result.document.export_to_markdown().strip()
    finally:
        try:
            os.remove(tmp_path)
        except OSError:
            pass

    if not text:
        return JSONResponse(status_code=204, content=None)

    return JSONResponse({"text": text})

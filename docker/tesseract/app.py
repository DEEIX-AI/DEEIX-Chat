from __future__ import annotations

import os
import tempfile
from typing import List

import pypdfium2 as pdfium
import pytesseract
from fastapi import FastAPI, File, Form, UploadFile
from fastapi.responses import JSONResponse
from PIL import Image, ImageOps

app = FastAPI(title="DEEIX Chat Tesseract OCR Service")

render_scale = max(1.0, min(3.0, float(os.getenv("TESSERACT_RENDER_SCALE", "2.0"))))
tesseract_lang = os.getenv("TESSERACT_LANG", "chi_sim+eng").strip() or "chi_sim+eng"
tesseract_psm = os.getenv("TESSERACT_PSM", "3").strip() or "3"
tesseract_oem = os.getenv("TESSERACT_OEM", "3").strip() or "3"
enable_grayscale = os.getenv("TESSERACT_GRAYSCALE", "true").strip().lower() not in {"0", "false", "no"}
image_suffixes = {".jpg", ".jpeg", ".png", ".webp", ".gif", ".bmp", ".tif", ".tiff"}


def parse_page_ranges(raw: str, total_pages: int) -> List[int]:
    if total_pages <= 0:
        return []
    text = (raw or "").strip()
    if not text:
        return list(range(1, total_pages + 1))
    pages = set()
    for part in text.split(","):
        chunk = part.strip()
        if not chunk:
            continue
        if "-" in chunk:
            start_text, end_text = chunk.split("-", 1)
            try:
                start = int(start_text)
                end = int(end_text)
            except ValueError:
                continue
            if start <= 0:
                continue
            if end < start:
                end = start
            for page in range(start, min(end, total_pages) + 1):
                pages.add(page)
            continue
        try:
            page = int(chunk)
        except ValueError:
            continue
        if 1 <= page <= total_pages:
            pages.add(page)
    return sorted(pages) if pages else list(range(1, total_pages + 1))


def preprocess_image(image: Image.Image) -> Image.Image:
    normalized = ImageOps.exif_transpose(image)
    if enable_grayscale:
        return normalized.convert("L")
    return normalized.convert("RGB")


def extract_page_text(image: Image.Image) -> str:
    config = f"--psm {tesseract_psm} --oem {tesseract_oem}"
    text = pytesseract.image_to_string(image, lang=tesseract_lang, config=config)
    return "\n".join(line.strip() for line in text.splitlines() if line.strip()).strip()


def render_page(document: pdfium.PdfDocument, page_index: int) -> Image.Image:
    page = document[page_index]
    bitmap = page.render(scale=render_scale)
    try:
        return bitmap.to_pil()
    finally:
        page.close()
        bitmap.close()


@app.get("/healthz")
def healthz() -> JSONResponse:
    return JSONResponse(
        {
            "status": "ok",
            "engine": "tesseract",
            "lang": tesseract_lang,
            "psm": tesseract_psm,
            "oem": tesseract_oem,
        }
    )


@app.post("/ocr")
async def ocr_file(
    file: UploadFile = File(...),
    page_ranges: str = Form(""),
    prompt: str = Form(""),
) -> JSONResponse:
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
        if suffix.lower() in image_suffixes:
            image = Image.open(tmp_path)
            try:
                processed = preprocess_image(image)
                try:
                    page_items = [{"page_number": 1, "text": extract_page_text(processed)}]
                finally:
                    processed.close()
            finally:
                image.close()
        else:
            document = pdfium.PdfDocument(tmp_path)
            try:
                total_pages = len(document)
                selected_pages = parse_page_ranges(page_ranges, total_pages)
                page_items = []
                for page_number in selected_pages:
                    image = render_page(document, page_number - 1)
                    try:
                        processed = preprocess_image(image)
                        try:
                            text = extract_page_text(processed)
                        finally:
                            processed.close()
                    finally:
                        image.close()
                    page_items.append({"page_number": page_number, "text": text})
            finally:
                document.close()
    finally:
        try:
            os.remove(tmp_path)
        except OSError:
            pass

    if not any(item["text"].strip() for item in page_items):
        return JSONResponse(status_code=204, content=None)

    return JSONResponse(
        {
            "rendered_pages": len(page_items),
            "pages": page_items,
        }
    )

FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      libglib2.0-0 \
      libgl1 \
      tesseract-ocr \
      tesseract-ocr-eng \
      tesseract-ocr-chi-sim \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
    fastapi==0.116.1 \
    uvicorn[standard]==0.35.0 \
    python-multipart==0.0.20 \
    Pillow==11.3.0 \
    pypdfium2==4.30.0 \
    pytesseract==0.3.13

COPY docker/tesseract/app.py /app/app.py

EXPOSE 8004

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8004"]

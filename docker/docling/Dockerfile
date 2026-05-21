FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      libglib2.0-0 \
      libgl1 \
      libmagic1 \
      poppler-utils \
      tesseract-ocr \
      tesseract-ocr-eng \
      tesseract-ocr-chi-sim \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
    fastapi==0.116.1 \
    uvicorn[standard]==0.35.0 \
    python-multipart==0.0.20 \
    docling==2.44.0

COPY docker/docling/app.py /app/app.py

EXPOSE 8005

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8005"]

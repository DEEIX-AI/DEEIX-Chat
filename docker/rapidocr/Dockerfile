FROM python:3.11-slim

ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app

RUN apt-get update \
    && apt-get install -y --no-install-recommends \
      libglib2.0-0 \
      libgl1 \
      libgomp1 \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
    fastapi==0.116.1 \
    uvicorn[standard]==0.35.0 \
    python-multipart==0.0.20 \
    Pillow==11.3.0 \
    pypdfium2==4.30.0 \
    rapidocr_onnxruntime==1.4.4

COPY docker/rapidocr/app.py /app/app.py

EXPOSE 8002

CMD ["uvicorn", "app:app", "--host", "0.0.0.0", "--port", "8002"]

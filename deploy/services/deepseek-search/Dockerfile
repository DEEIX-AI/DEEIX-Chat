FROM python:3.12-alpine

WORKDIR /app
COPY server.py /app/server.py

ENV PORT=8080
EXPOSE 8080
USER nobody:nobody

CMD ["python3", "/app/server.py"]
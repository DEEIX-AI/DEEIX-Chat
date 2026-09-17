# syntax=docker/dockerfile:1

ARG VOCECHAT_BASE_IMAGE=privoce/vocechat-server:v0.5.32

FROM ${VOCECHAT_BASE_IMAGE} AS vocechat-base
FROM python:3.13.7-alpine3.22

WORKDIR /home/vocechat-server

# Keep the official VoceChat server and its runtime files, while adding the
# DEEIX integration files and Python runtime to the same image.
COPY --from=vocechat-base /home/vocechat-server /home/vocechat-server
COPY --from=vocechat-base /etc/ssl/certs /etc/ssl/certs
COPY config.toml /home/vocechat-server/config/config.toml
COPY init.py /opt/deeix/vocechat-init.py

ENTRYPOINT ["/home/vocechat-server/vocechat-server"]

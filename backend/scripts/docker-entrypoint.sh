#!/bin/sh
# 生产容器入口：修复挂载卷属主后降权运行。
# 背景：uploads_prod 命名卷由早期 root 容器创建，非 root 镜像（appuser uid=10001）
# 对存量卷无写权限会导致全部上传失败；每次启动先 chown 再 exec，旧卷自动修复。
set -e
chown -R appuser:appuser /app/uploads
exec su-exec appuser "$@"

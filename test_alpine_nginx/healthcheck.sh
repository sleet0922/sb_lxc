#!/bin/sh
# 健康检查脚本
curl -sf http://localhost:8080/ >/dev/null 2>&1 || exit 1
echo "alpine-nginx healthy"

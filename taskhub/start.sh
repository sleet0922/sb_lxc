#!/bin/bash
set -e

echo "[start.sh] 启动 TaskHub 服务栈"

# === 1. 启动 MariaDB (数据目录已在构建期预初始化) ===
echo "[start.sh] 启动 MariaDB (端口 3306)"
mysqld_safe --user=mysql &
sleep 3

# === 2. 启动 Redis ===
echo "[start.sh] 启动 Redis (端口 6379)"
redis-server --daemonize yes --bind 127.0.0.1 --port 6379
sleep 1

# === 3. 启动 Go 应用 ===
echo "[start.sh] 启动 TaskHub 应用 (端口 8080)"
# 使用 exec 替换进程，让 systemd 正确追踪主进程；日志由 systemd journald 接管
exec /app/bin/taskhub

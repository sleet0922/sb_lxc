#!/bin/bash
# 启动脚本：启动所有服务
set -e

echo "=== 启动 complex-demo 容器服务 ==="

# 1. SSH
echo "[1/3] 启动 SSH..."
service ssh start || /usr/sbin/sshd

# 2. DNSmasq
echo "[2/3] 启动 DNSmasq..."
dnsmasq --no-daemon --conf-file=/etc/dnsmasq.conf &

# 3. Nginx (前台运行)
echo "[3/3] 启动 Nginx..."
exec nginx

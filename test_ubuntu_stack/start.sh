#!/bin/bash
# 拉起 mariadb + redis + flask，systemd Type=simple 需要前台阻塞
set -e

# MariaDB (不走 systemd，直接启动)
mkdir -p /var/run/mysqld && chown mysql:mysql /var/run/mysqld
mysqld --user=mysql --datadir=/var/lib/mysql &

# 等待 MariaDB 就绪
for i in $(seq 1 15); do
    mysqladmin ping 2>/dev/null && break
    sleep 1
done

# Redis (不走 systemd，直接启动，daemonize no)
redis-server --daemonize no --save "" --appendonly no &
REDIS_PID=$!

# Flask 应用 (前台阻塞，systemd 主进程)
exec python3 /app/app.py

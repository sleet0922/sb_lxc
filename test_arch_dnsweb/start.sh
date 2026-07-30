#!/bin/sh
# 同时拉起 dnsmasq 和 busybox httpd
# systemd Type=simple 需要 exec 前台阻塞，但两个进程都要前台运行，
# 所以用 wait 等待任一退出
set -e

# 停止可能残留的 systemd-resolved (运行时保险)
systemctl stop systemd-resolved 2>/dev/null || true

# dnsmasq 前台模式 (keep-in-foreground)
dnsmasq --conf-file=/etc/dnsmasq.conf --conf-dir=/etc/dnsmasq.d,/etc/dnsmasq.d/.local --keep-in-foreground &

# busybox httpd 前台模式 (-f)
busybox httpd -f -p 8081 -h /var/www/html &

# 等待任一子进程退出 (systemd 主进程不能退出)
wait -n 2>/dev/null || wait

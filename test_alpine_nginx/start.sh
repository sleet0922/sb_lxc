#!/bin/sh
# nginx 前台运行
# /run 是 tmpfs，每次启动都会重置，运行时需重新创建 pid 目录
set -e
mkdir -p /run/nginx
exec nginx -g 'daemon off;'

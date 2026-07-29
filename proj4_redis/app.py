#!/usr/bin/env python3
"""Flask + Redis 计数器 - 用 Redis 持久化访问次数

与 proj1 (SQLite 版) 对比：Redis 提供更高并发写入性能，且数据落在内存中。
通过 Redis 的 INCR 原子命令实现计数，无需应用层加锁。
"""

import os
import redis
from flask import Flask, jsonify

app = Flask(__name__)

# Redis 连接配置 (容器内 127.0.0.1:6379)
REDIS_HOST = os.environ.get("REDIS_HOST", "127.0.0.1")
REDIS_PORT = int(os.environ.get("REDIS_PORT", "6379"))
COUNTER_KEY = "flask:counter"

# decode_responses=True 让返回值自动转 str，省去手动 .decode()
r = redis.Redis(
    host=REDIS_HOST,
    port=REDIS_PORT,
    decode_responses=True,
    socket_timeout=3,
    socket_connect_timeout=3,
)


def get_count() -> int:
    """读取当前计数 (不存在则返回 0)"""
    val = r.get(COUNTER_KEY)
    return int(val) if val is not None else 0


def increment_count() -> int:
    """INCR 是 Redis 原子命令，并发安全"""
    return r.incr(COUNTER_KEY)


def reset_count() -> None:
    r.set(COUNTER_KEY, 0)


HTML_PAGE = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Redis 计数器</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI",
                         "PingFang SC", "Microsoft YaHei", sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #00b09b 0%, #96c93d 100%);
            color: #333;
        }
        .card {
            background: #ffffff;
            border-radius: 16px;
            padding: 48px 64px;
            box-shadow: 0 20px 60px rgba(0, 0, 0, 0.25);
            text-align: center;
            max-width: 480px;
            width: 90%;
        }
        h1 { font-size: 28px; margin-bottom: 8px; color: #2c3e50; }
        .subtitle { font-size: 14px; color: #95a5a6; margin-bottom: 32px; }
        .count-label { font-size: 16px; color: #7f8c8d; margin-bottom: 8px; }
        .count-value {
            font-size: 72px; font-weight: 700; color: #00b09b;
            line-height: 1.1; font-variant-numeric: tabular-nums;
        }
        .actions {
            margin-top: 32px; display: flex; gap: 12px; justify-content: center;
        }
        a.btn {
            display: inline-block; padding: 10px 20px; border-radius: 8px;
            text-decoration: none; font-size: 14px; font-weight: 500;
            transition: transform 0.1s, opacity 0.2s;
        }
        a.btn:active { transform: scale(0.97); }
        .btn-refresh { background: #00b09b; color: #fff; }
        .btn-reset { background: #e74c3c; color: #fff; }
        .footer { margin-top: 24px; font-size: 12px; color: #bdc3c7; }
    </style>
</head>
<body>
    <div class="card">
        <h1>Redis 计数器</h1>
        <p class="subtitle">Redis 持久化 · sb_lxc 多阶段构建测试</p>
        <p class="count-label">本页已被访问</p>
        <div class="count-value" id="count">{count}</div>
        <p class="count-label" style="margin-top:8px;">次</p>
        <div class="actions">
            <a href="/" class="btn btn-refresh">刷新</a>
            <a href="/reset" class="btn btn-reset">重置</a>
        </div>
        <p class="footer">Powered by Flask + Redis · Debian 13 · Incus</p>
    </div>
</body>
</html>
"""


@app.route("/")
def index():
    count = increment_count()
    return HTML_PAGE.replace("{count}", str(count))


@app.route("/api/count")
def api_count():
    return jsonify({"count": get_count(), "backend": "redis"})


@app.route("/health")
def health():
    # 同时探测 Redis 连通性
    try:
        r.ping()
        redis_ok = True
    except redis.RedisError:
        redis_ok = False
    return jsonify({"status": "ok" if redis_ok else "degraded", "redis": redis_ok})


@app.route("/reset")
def reset():
    reset_count()
    return """
    <!DOCTYPE html>
    <html lang="zh-CN"><head><meta charset="UTF-8">
    <meta http-equiv="refresh" content="2;url=/">
    <title>已重置</title>
    <style>body{font-family:sans-serif;text-align:center;padding:80px;
        background:#f5f5f5;color:#333;}</style></head>
    <body><h2>计数已重置为 0</h2>
    <p>2 秒后返回首页...</p>
    <p><a href="/">立即返回</a></p></body></html>
    """


if __name__ == "__main__":
    app.run(host="0.0.0.0", port=5000)

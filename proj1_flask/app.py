#!/usr/bin/env python3
"""Flask 网页计数器 - 用 SQLite 持久化访问次数"""

import os
import sqlite3
import threading

from flask import Flask, jsonify

app = Flask(__name__)

# 数据库路径 (容器内 /app/data/counter.db)
DB_DIR = os.environ.get("COUNTER_DB_DIR", "/app/data")
DB_PATH = os.path.join(DB_DIR, "counter.db")

# 计数原子性保护
_lock = threading.Lock()


def init_db():
    """启动时自动建表 (若不存在)"""
    os.makedirs(DB_DIR, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    try:
        conn.execute(
            """
            CREATE TABLE IF NOT EXISTS counter (
                id    INTEGER PRIMARY KEY CHECK (id = 1),
                value INTEGER NOT NULL DEFAULT 0
            )
            """
        )
        # 确保唯一一行存在
        conn.execute(
            "INSERT INTO counter (id, value) VALUES (1, 0) "
            "ON CONFLICT(id) DO NOTHING"
        )
        conn.commit()
    finally:
        conn.close()


def get_count():
    """读取当前计数"""
    conn = sqlite3.connect(DB_PATH)
    try:
        cur = conn.execute("SELECT value FROM counter WHERE id = 1")
        row = cur.fetchone()
        return row[0] if row else 0
    finally:
        conn.close()


def increment_count():
    """计数 +1 并返回新值"""
    with _lock:
        conn = sqlite3.connect(DB_PATH)
        try:
            conn.execute(
                "UPDATE counter SET value = value + 1 WHERE id = 1"
            )
            conn.commit()
            return get_count()
        finally:
            conn.close()


def reset_count():
    """重置计数为 0"""
    with _lock:
        conn = sqlite3.connect(DB_PATH)
        try:
            conn.execute("UPDATE counter SET value = 0 WHERE id = 1")
            conn.commit()
        finally:
            conn.close()


HTML_PAGE = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Flask 计数器</title>
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI",
                         "PingFang SC", "Microsoft YaHei", sans-serif;
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
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
        h1 {
            font-size: 28px;
            margin-bottom: 8px;
            color: #2c3e50;
        }
        .subtitle {
            font-size: 14px;
            color: #95a5a6;
            margin-bottom: 32px;
        }
        .count-label {
            font-size: 16px;
            color: #7f8c8d;
            margin-bottom: 8px;
        }
        .count-value {
            font-size: 72px;
            font-weight: 700;
            color: #667eea;
            line-height: 1.1;
            font-variant-numeric: tabular-nums;
        }
        .actions {
            margin-top: 32px;
            display: flex;
            gap: 12px;
            justify-content: center;
        }
        a.btn {
            display: inline-block;
            padding: 10px 20px;
            border-radius: 8px;
            text-decoration: none;
            font-size: 14px;
            font-weight: 500;
            transition: transform 0.1s, opacity 0.2s;
        }
        a.btn:active { transform: scale(0.97); }
        .btn-refresh {
            background: #667eea;
            color: #fff;
        }
        .btn-reset {
            background: #e74c3c;
            color: #fff;
        }
        .footer {
            margin-top: 24px;
            font-size: 12px;
            color: #bdc3c7;
        }
    </style>
</head>
<body>
    <div class="card">
        <h1>Flask 计数器</h1>
        <p class="subtitle">SQLite 持久化 · sb_lxc 容器构建测试</p>
        <p class="count-label">本页已被访问</p>
        <div class="count-value" id="count">{count}</div>
        <p class="count-label" style="margin-top:8px;">次</p>
        <div class="actions">
            <a href="/" class="btn btn-refresh">刷新</a>
            <a href="/reset" class="btn btn-reset">重置</a>
        </div>
        <p class="footer">Powered by Flask 3.0 · Debian 13 · Incus</p>
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
    return jsonify({"count": get_count()})


@app.route("/health")
def health():
    return jsonify({"status": "ok"})


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
    init_db()
    app.run(host="0.0.0.0", port=5000)

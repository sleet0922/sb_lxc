import os
import time
import json
import redis
import pymysql
from flask import Flask, request, jsonify, Response

app = Flask(__name__)

# Redis connection
REDIS_HOST = os.environ.get("REDIS_HOST", "127.0.0.1")
REDIS_PORT = int(os.environ.get("REDIS_PORT", "6379"))
r = redis.Redis(host=REDIS_HOST, port=REDIS_PORT, decode_responses=True)

# MariaDB connection
DB_HOST = os.environ.get("DB_HOST", "127.0.0.1")
DB_PORT = int(os.environ.get("DB_PORT", "3306"))
DB_USER = os.environ.get("DB_USER", "appuser")
DB_PASS = os.environ.get("DB_PASS", "apppass")
DB_NAME = os.environ.get("DB_NAME", "appdb")


def get_db():
    return pymysql.connect(
        host=DB_HOST, port=DB_PORT, user=DB_USER,
        password=DB_PASS, database=DB_NAME,
        cursorclass=pymysql.cursors.DictCursor,
    )


def init_db():
    """Wait for MariaDB to be ready, then ensure table exists."""
    for i in range(30):
        try:
            conn = get_db()
            with conn.cursor() as cur:
                cur.execute("""
                    CREATE TABLE IF NOT EXISTS items (
                        id INT AUTO_INCREMENT PRIMARY KEY,
                        title VARCHAR(255) NOT NULL,
                        done BOOLEAN DEFAULT FALSE,
                        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
                    )
                """)
            conn.commit()
            conn.close()
            app.logger.info("Database initialized successfully")
            return
        except Exception as e:
            app.logger.warning(f"DB not ready (attempt {i+1}/30): {e}")
            time.sleep(2)
    app.logger.error("Database initialization failed after 30 attempts")


HTML_PAGE = """<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>Python Flask + Redis + MariaDB</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, sans-serif; background: #f0f2f5; padding: 40px; }
.container { max-width: 700px; margin: 0 auto; background: #fff; border-radius: 12px; padding: 32px; box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
h1 { color: #1a1a2e; margin-bottom: 8px; }
.subtitle { color: #666; margin-bottom: 24px; font-size: 14px; }
.card { background: #f8f9fa; border-radius: 8px; padding: 20px; margin-bottom: 16px; }
.card h2 { font-size: 18px; margin-bottom: 12px; color: #333; }
.counter { font-size: 48px; font-weight: bold; color: #4f46e5; }
.btn { padding: 10px 20px; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; }
.btn-primary { background: #4f46e5; color: #fff; }
.btn-primary:hover { background: #4338ca; }
.btn-danger { background: #ef4444; color: #fff; padding: 4px 10px; font-size: 12px; }
.input-row { display: flex; gap: 8px; margin-bottom: 12px; }
input[type="text"] { flex: 1; padding: 10px; border: 2px solid #e0e0e0; border-radius: 6px; }
ul { list-style: none; }
li { display: flex; align-items: center; padding: 12px; border-bottom: 1px solid #eee; gap: 12px; }
li.done span { text-decoration: line-through; color: #999; }
li span { flex: 1; }
.stats { margin-top: 16px; padding-top: 16px; border-top: 1px solid #eee; color: #666; font-size: 13px; display: flex; justify-content: space-between; }
</style>
</head>
<body>
<div class="container">
<h1>Flask + Redis + MariaDB</h1>
<p class="subtitle">多阶段 venv 构建 + 3 服务 systemd 编排 — sb_lxc 测试</p>

<div class="card">
<h2>Redis 计数器</h2>
<div class="counter" id="counter">0</div>
<br>
<button class="btn btn-primary" onclick="incr()">增加计数</button>
<button class="btn btn-primary" onclick="reset()">重置</button>
</div>

<div class="card">
<h2>MariaDB 待办事项</h2>
<div class="input-row">
<input type="text" id="title" placeholder="输入待办..." onkeydown="if(event.key==='Enter')addItem()">
<button class="btn btn-primary" onclick="addItem()">添加</button>
</div>
<ul id="list"></ul>
<div class="stats" id="stats"></div>
</div>
</div>
<script>
async function loadCounter(){const r=await fetch('/api/counter');const d=await r.json();document.getElementById('counter').textContent=d.count}
async function incr(){await fetch('/api/counter/increment',{method:'POST'});loadCounter()}
async function reset(){await fetch('/api/counter/reset',{method:'POST'});loadCounter()}
async function loadItems(){const r=await fetch('/api/items');const d=await r.json();const l=document.getElementById('list');l.innerHTML='';if(!d.length){l.innerHTML='<li style="color:#999">暂无数据</li>';updateStats(d);return}d.forEach(i=>{const li=document.createElement('li');li.className=i.done?'done':'';li.innerHTML='<input type="checkbox" '+(i.done?'checked':'')+' onchange="toggle('+i.id+',this.checked)"> <span>'+esc(i.title)+'</span> <button class="btn btn-danger" onclick="del('+i.id+')">删除</button>';l.appendChild(li)});updateStats(d)}
function updateStats(d){const done=d.filter(i=>i.done).length;document.getElementById('stats').innerHTML='<span>共 '+d.length+' 项</span><span>已完成 '+done+' 项</span>'}
async function addItem(){const i=document.getElementById('title');const v=i.value.trim();if(!v)return;await fetch('/api/items',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({title:v})});i.value='';loadItems()}
async function toggle(id,done){await fetch('/api/items/'+id,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({done:done})});loadItems()}
async function del(id){await fetch('/api/items/'+id,{method:'DELETE'});loadItems()}
function esc(s){return s.replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
loadCounter();loadItems()
</script>
</body>
</html>"""


@app.route("/")
def index():
    return HTML_PAGE


@app.route("/health")
def health():
    return jsonify({"status": "ok", "service": "python-django-stack"})


@app.route("/api/counter")
def get_counter():
    count = int(r.get("counter") or 0)
    return jsonify({"count": count})


@app.route("/api/counter/increment", methods=["POST"])
def incr_counter():
    count = r.incr("counter")
    return jsonify({"count": int(count)})


@app.route("/api/counter/reset", methods=["POST"])
def reset_counter():
    r.set("counter", 0)
    return jsonify({"count": 0})


@app.route("/api/items")
def list_items():
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute("SELECT * FROM items ORDER BY id DESC")
            return jsonify(cur.fetchall())
    finally:
        conn.close()


@app.route("/api/items", methods=["POST"])
def create_item():
    data = request.get_json()
    if not data or not data.get("title"):
        return jsonify({"error": "title required"}), 400
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute("INSERT INTO items (title) VALUES (%s)", (data["title"],))
        conn.commit()
        return jsonify({"ok": True}), 201
    finally:
        conn.close()


@app.route("/api/items/<int:item_id>", methods=["PUT"])
def update_item(item_id):
    data = request.get_json()
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute("UPDATE items SET done=%s WHERE id=%s", (data.get("done", False), item_id))
        conn.commit()
        return jsonify({"ok": True})
    finally:
        conn.close()


@app.route("/api/items/<int:item_id>", methods=["DELETE"])
def delete_item(item_id):
    conn = get_db()
    try:
        with conn.cursor() as cur:
            cur.execute("DELETE FROM items WHERE id=%s", (item_id,))
        conn.commit()
        return jsonify({"ok": True})
    finally:
        conn.close()


if __name__ == "__main__":
    init_db()
    app.run(host="0.0.0.0", port=5000)
else:
    # gunicorn 也会执行这里
    init_db()

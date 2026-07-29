<?php
// LAMP Stack — Caddy + PHP-FPM + MariaDB
// 单阶段多服务编排测试

$DB_HOST = '127.0.0.1';
$DB_USER = 'appuser';
$DB_PASS = 'apppass';
$DB_NAME = 'appdb';

function get_db() {
    global $DB_HOST, $DB_USER, $DB_PASS, $DB_NAME;
    static $pdo = null;
    if ($pdo === null) {
        try {
            $pdo = new PDO("mysql:host=$DB_HOST;dbname=$DB_NAME;charset=utf8mb4", $DB_USER, $DB_PASS, [
                PDO::ATTR_ERRMODE => PDO::ERRMODE_EXCEPTION,
                PDO::ATTR_DEFAULT_FETCH_MODE => PDO::FETCH_ASSOC,
            ]);
        } catch (PDOException $e) {
            http_response_code(500);
            header('Content-Type: application/json');
            echo json_encode(['error' => 'Database connection failed: ' . $e->getMessage()]);
            exit;
        }
    }
    return $pdo;
}

// Initialize table
function init_table() {
    $pdo = get_db();
    $pdo->exec("CREATE TABLE IF NOT EXISTS todos (
        id INT AUTO_INCREMENT PRIMARY KEY,
        title VARCHAR(255) NOT NULL,
        done TINYINT(1) DEFAULT 0,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    )");
}

$method = $_SERVER['REQUEST_METHOD'];
$path = parse_url($_SERVER['REQUEST_URI'], PHP_URL_PATH);

// Health check
if ($path === '/health') {
    header('Content-Type: application/json');
    echo json_encode(['status' => 'ok', 'service' => 'lamp-stack']);
    exit;
}

// API routes
if (strpos($path, '/api/') === 0) {
    header('Content-Type: application/json');

    if ($path === '/api/todos' && $method === 'GET') {
        init_table();
        $pdo = get_db();
        $rows = $pdo->query("SELECT * FROM todos ORDER BY id DESC")->fetchAll();
        echo json_encode(array_map(function($r) {
            $r['done'] = (bool)$r['done'];
            return $r;
        }, $rows));
        exit;
    }

    if ($path === '/api/todos' && $method === 'POST') {
        $body = json_decode(file_get_contents('php://input'), true);
        if (!$body || empty($body['title'])) {
            http_response_code(400);
            echo json_encode(['error' => 'title required']);
            exit;
        }
        init_table();
        $pdo = get_db();
        $stmt = $pdo->prepare("INSERT INTO todos (title) VALUES (?)");
        $stmt->execute([$body['title']]);
        $id = $pdo->lastInsertId();
        $row = $pdo->query("SELECT * FROM todos WHERE id=$id")->fetch();
        $row['done'] = (bool)$row['done'];
        http_response_code(201);
        echo json_encode($row);
        exit;
    }

    if (preg_match('#^/api/todos/(\d+)$#', $path, $m)) {
        $id = $m[1];
        init_table();
        $pdo = get_db();

        if ($method === 'DELETE') {
            $stmt = $pdo->prepare("DELETE FROM todos WHERE id=?");
            $stmt->execute([$id]);
            if ($stmt->rowCount() === 0) {
                http_response_code(404);
                echo json_encode(['error' => 'not found']);
            } else {
                http_response_code(204);
            }
            exit;
        }

        if ($method === 'PUT') {
            $body = json_decode(file_get_contents('php://input'), true);
            $done = !empty($body['done']) ? 1 : 0;
            $stmt = $pdo->prepare("UPDATE todos SET done=? WHERE id=?");
            $stmt->execute([$done, $id]);
            $row = $pdo->query("SELECT * FROM todos WHERE id=$id")->fetch();
            if (!$row) {
                http_response_code(404);
                echo json_encode(['error' => 'not found']);
            } else {
                $row['done'] = (bool)$row['done'];
                echo json_encode($row);
            }
            exit;
        }
    }

    http_response_code(404);
    echo json_encode(['error' => 'not found']);
    exit;
}

// Serve main page
header('Content-Type: text/html; charset=utf-8');
?>
<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="UTF-8">
<title>LAMP Stack — Caddy + PHP-FPM + MariaDB</title>
<style>
* { margin: 0; padding: 0; box-sizing: border-box; }
body { font-family: -apple-system, sans-serif; background: #f0f2f5; padding: 40px; }
.container { max-width: 700px; margin: 0 auto; background: #fff; border-radius: 12px; padding: 32px; box-shadow: 0 2px 12px rgba(0,0,0,0.08); }
h1 { color: #1a1a2e; margin-bottom: 8px; }
.subtitle { color: #666; margin-bottom: 24px; font-size: 14px; }
.card { background: #f8f9fa; border-radius: 8px; padding: 20px; margin-bottom: 16px; }
.card h2 { font-size: 18px; margin-bottom: 12px; }
.input-row { display: flex; gap: 8px; margin-bottom: 12px; }
input[type="text"] { flex: 1; padding: 10px; border: 2px solid #e0e0e0; border-radius: 6px; }
.btn { padding: 10px 20px; border: none; border-radius: 6px; cursor: pointer; font-size: 14px; background: #4f46e5; color: #fff; }
.btn:hover { background: #4338ca; }
.btn-danger { background: #ef4444; padding: 4px 10px; font-size: 12px; }
ul { list-style: none; }
li { display: flex; align-items: center; padding: 12px; border-bottom: 1px solid #eee; gap: 12px; }
li.done span { text-decoration: line-through; color: #999; }
li span { flex: 1; }
.stats { margin-top: 16px; padding-top: 16px; border-top: 1px solid #eee; color: #666; font-size: 13px; display: flex; justify-content: space-between; }
.empty { color: #999; padding: 20px; text-align: center; }
</style>
</head>
<body>
<div class="container">
<h1>LAMP Stack</h1>
<p class="subtitle">Caddy + PHP-FPM + MariaDB — 单阶段 3 服务编排 (sb_lxc 测试)</p>
<div class="card">
<h2>待办事项</h2>
<div class="input-row">
<input type="text" id="title" placeholder="输入待办..." onkeydown="if(event.key==='Enter')addItem()">
<button class="btn" onclick="addItem()">添加</button>
</div>
<ul id="list"><div class="empty">加载中...</div></ul>
<div class="stats" id="stats"></div>
</div>
</div>
<script>
async function load(){try{const r=await fetch('/api/todos');const d=await r.json();const l=document.getElementById('list');l.innerHTML='';if(!d.length){l.innerHTML='<div class="empty">暂无数据</div>';updateStats(d);return}d.forEach(i=>{const li=document.createElement('li');li.className=i.done?'done':'';li.innerHTML='<input type="checkbox" '+(i.done?'checked':'')+' onchange="toggle('+i.id+',this.checked)"> <span>'+esc(i.title)+'</span> <button class="btn-danger" onclick="del('+i.id+')">删除</button>';l.appendChild(li)});updateStats(d)}catch(e){document.getElementById('list').innerHTML='<div class="empty">错误: '+e.message+'</div>'}}
function updateStats(d){const done=d.filter(i=>i.done).length;document.getElementById('stats').innerHTML='<span>共 '+d.length+' 项</span><span>已完成 '+done+' 项</span>'}
async function addItem(){const i=document.getElementById('title');const v=i.value.trim();if(!v)return;await fetch('/api/todos',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({title:v})});i.value='';load()}
async function toggle(id,done){await fetch('/api/todos/'+id,{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({done:done})});load()}
async function del(id){await fetch('/api/todos/'+id,{method:'DELETE'});load()}
function esc(s){return s.replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
load()
</script>
</body>
</html>

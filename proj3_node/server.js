// node-todo-api - 内存版 Todo API
//
// 用内存 Map 存储 todo，不依赖真实 Redis 服务（避免运行时需要 redis-server）；
// 但顶部 require('redis') 会真正加载该模块 —— 若 builder 阶段 npm install 未生效，
// 启动时即抛 MODULE_NOT_FOUND，以此证明依赖确已安装。

const express = require('express');
const redis = require('redis');

const app = express();
const HOST = '0.0.0.0';
const PORT = 3000;

app.use(express.json());

// 仅引用 redis 模块以证明依赖已安装，不实际连接，避免运行时依赖 redis-server。
// redis.createClient 在 redis@4 中是一个函数，能取到即说明模块加载成功。
const redisCreateClient = redis.createClient;
const redisAvailable = typeof redisCreateClient === 'function';

// 内存存储
const todos = new Map();
let nextId = 1;

function nowIso() {
  return new Date().toISOString();
}

// 健康检查
app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'node-todo-api' });
});

// 首页：API 用法说明
app.get('/', (req, res) => {
  res.type('html').send(`<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <title>node-todo-api</title>
  <style>
    body { font-family: -apple-system, Segoe UI, Roboto, sans-serif; max-width: 720px; margin: 40px auto; padding: 0 16px; color: #222; }
    h1 { font-size: 1.4rem; }
    code { background: #f4f4f4; padding: 2px 6px; border-radius: 4px; }
    table { border-collapse: collapse; width: 100%; }
    th, td { border: 1px solid #ddd; padding: 8px 10px; text-align: left; font-size: 14px; }
    th { background: #fafafa; }
    .ok { color: #0a7d28; }
  </style>
</head>
<body>
  <h1>node-todo-api</h1>
  <p>一个最小化的内存 Todo API（数据存在进程内存中，重启后清空）。</p>
  <p class="ok">redis 模块加载状态: ${redisAvailable ? 'available' : 'missing'}</p>

  <h2>接口列表</h2>
  <table>
    <tr><th>方法</th><th>路径</th><th>说明</th></tr>
    <tr><td>GET</td><td><code>/health</code></td><td>健康检查</td></tr>
    <tr><td>GET</td><td><code>/api/todos</code></td><td>获取所有 todo</td></tr>
    <tr><td>POST</td><td><code>/api/todos</code></td><td>创建 todo，body: <code>{"title":"..."}</code></td></tr>
    <tr><td>PUT</td><td><code>/api/todos/:id/done</code></td><td>标记指定 todo 为已完成</td></tr>
    <tr><td>DELETE</td><td><code>/api/todos/:id</code></td><td>删除指定 todo</td></tr>
  </table>

  <h2>示例</h2>
  <pre>curl -X POST http://node.test/api/todos -H 'Content-Type: application/json' -d '{"title":"hello"}'
curl http://node.test/api/todos
curl -X PUT http://node.test/api/todos/1/done
curl -X DELETE http://node.test/api/todos/1</pre>
</body>
</html>`);
});

// 获取所有 todo
app.get('/api/todos', (req, res) => {
  const list = Array.from(todos.values());
  res.json(list);
});

// 创建 todo
app.post('/api/todos', (req, res) => {
  const title = req.body && typeof req.body.title === 'string' ? req.body.title.trim() : '';
  if (!title) {
    return res.status(400).json({ error: 'title is required' });
  }
  const id = String(nextId++);
  const todo = {
    id,
    title,
    done: false,
    createdAt: nowIso()
  };
  todos.set(id, todo);
  res.status(201).json(todo);
});

// 标记完成
app.put('/api/todos/:id/done', (req, res) => {
  const todo = todos.get(req.params.id);
  if (!todo) {
    return res.status(404).json({ error: 'todo not found' });
  }
  todo.done = true;
  todo.updatedAt = nowIso();
  res.json(todo);
});

// 删除 todo
app.delete('/api/todos/:id', (req, res) => {
  const id = req.params.id;
  if (!todos.has(id)) {
    return res.status(404).json({ error: 'todo not found' });
  }
  todos.delete(id);
  res.status(204).end();
});

app.listen(PORT, HOST, () => {
  console.log(`node-todo-api listening on http://${HOST}:${PORT}`);
  console.log(`redis module available: ${redisAvailable}`);
});

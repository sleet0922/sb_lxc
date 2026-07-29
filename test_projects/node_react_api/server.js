const express = require('express')
const path = require('path')
const fs = require('fs')

const app = express()
const PORT = process.env.PORT || 3000
const DATA_FILE = '/tmp/todos.json'

app.use(express.json())
app.use(express.static(path.join(__dirname, 'dist')))

// Load todos from file
function loadTodos() {
  try {
    const data = fs.readFileSync(DATA_FILE, 'utf-8')
    return JSON.parse(data)
  } catch {
    return []
  }
}

// Save todos to file
function saveTodos(todos) {
  fs.writeFileSync(DATA_FILE, JSON.stringify(todos, null, 2))
}

let todos = loadTodos()
let nextId = todos.reduce((max, t) => Math.max(max, t.id), 0) + 1

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'ok', service: 'node-react-api' })
})

// List todos
app.get('/api/todos', (req, res) => {
  res.json(todos)
})

// Create todo
app.post('/api/todos', (req, res) => {
  const { title } = req.body
  if (!title) {
    return res.status(400).json({ error: 'title required' })
  }
  const todo = {
    id: nextId++,
    title,
    done: false,
    created_at: new Date().toISOString(),
  }
  todos.push(todo)
  saveTodos(todos)
  res.status(201).json(todo)
})

// Update todo
app.put('/api/todos/:id', (req, res) => {
  const id = parseInt(req.params.id)
  const todo = todos.find(t => t.id === id)
  if (!todo) {
    return res.status(404).json({ error: 'not found' })
  }
  if (req.body.done !== undefined) todo.done = req.body.done
  if (req.body.title !== undefined) todo.title = req.body.title
  saveTodos(todos)
  res.json(todo)
})

// Delete todo
app.delete('/api/todos/:id', (req, res) => {
  const id = parseInt(req.params.id)
  const idx = todos.findIndex(t => t.id === id)
  if (idx === -1) {
    return res.status(404).json({ error: 'not found' })
  }
  todos.splice(idx, 1)
  saveTodos(todos)
  res.status(204).send()
})

// Stats
app.get('/api/stats', (req, res) => {
  const done = todos.filter(t => t.done).length
  res.json({ total: todos.length, done, pending: todos.length - done })
})

// SPA fallback
app.get('*', (req, res) => {
  res.sendFile(path.join(__dirname, 'dist', 'index.html'))
})

app.listen(PORT, '0.0.0.0', () => {
  console.log(`node-react-api listening on ${PORT}`)
})

import React, { useState, useEffect } from 'react'

const styles = {
  body: { fontFamily: '-apple-system, BlinkMacSystemFont, sans-serif', background: '#f0f2f5', minHeight: '100vh', display: 'flex', justifyContent: 'center', padding: '40px 20px' },
  container: { background: '#fff', borderRadius: '12px', boxShadow: '0 2px 12px rgba(0,0,0,0.08)', maxWidth: '600px', width: '100%', padding: '32px' },
  h1: { color: '#1a1a2e', marginBottom: '8px', fontSize: '28px' },
  subtitle: { color: '#666', marginBottom: '24px', fontSize: '14px' },
  inputRow: { display: 'flex', gap: '12px', marginBottom: '24px' },
  input: { flex: 1, padding: '12px 16px', border: '2px solid #e0e0e0', borderRadius: '8px', fontSize: '15px' },
  btn: { padding: '12px 24px', background: '#4f46e5', color: '#fff', border: 'none', borderRadius: '8px', fontSize: '15px', cursor: 'pointer' },
  btnDanger: { padding: '6px 12px', background: '#ef4444', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '13px', cursor: 'pointer' },
  todoItem: { display: 'flex', alignItems: 'center', padding: '16px', borderBottom: '1px solid #f0f0f0', gap: '12px' },
  title: { flex: 1, fontSize: '15px', color: '#333' },
  done: { textDecoration: 'line-through', color: '#999' },
  empty: { textAlign: 'center', color: '#999', padding: '40px', fontSize: '14px' },
  stats: { marginTop: '16px', paddingTop: '16px', borderTop: '1px solid #f0f0f0', color: '#666', fontSize: '13px', display: 'flex', justifyContent: 'space-between' },
}

export default function App() {
  const [todos, setTodos] = useState([])
  const [title, setTitle] = useState('')

  const load = async () => {
    const r = await fetch('/api/todos')
    const d = await r.json()
    setTodos(d)
  }

  useEffect(() => { load() }, [])

  const add = async () => {
    if (!title.trim()) return
    await fetch('/api/todos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title }),
    })
    setTitle('')
    load()
  }

  const toggle = async (id, done) => {
    await fetch(`/api/todos/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ done: !done }),
    })
    load()
  }

  const del = async (id) => {
    await fetch(`/api/todos/${id}`, { method: 'DELETE' })
    load()
  }

  const doneCount = todos.filter(t => t.done).length

  return (
    <div style={styles.body}>
      <div style={styles.container}>
        <h1 style={styles.h1}>Node React API</h1>
        <p style={styles.subtitle}>Express + React + Vite — sb_lxc 多阶段 npm 构建测试</p>
        <div style={styles.inputRow}>
          <input
            style={styles.input}
            value={title}
            onChange={e => setTitle(e.target.value)}
            onKeyDown={e => e.key === 'Enter' && add()}
            placeholder="输入待办事项..."
          />
          <button style={styles.btn} onClick={add}>添加</button>
        </div>
        {todos.length === 0 ? (
          <div style={styles.empty}>暂无待办事项</div>
        ) : (
          todos.map(t => (
            <div key={t.id} style={styles.todoItem}>
              <input type="checkbox" checked={t.done} onChange={() => toggle(t.id, t.done)} />
              <span style={{ ...styles.title, ...(t.done ? styles.done : {}) }}>{t.title}</span>
              <button style={styles.btnDanger} onClick={() => del(t.id)}>删除</button>
            </div>
          ))
        )}
        <div style={styles.stats}>
          <span>共 {todos.length} 项</span>
          <span>已完成 {doneCount} 项</span>
        </div>
      </div>
    </div>
  )
}

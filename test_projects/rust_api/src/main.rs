// Rust API — 零外部依赖 HTTP Todo API
// 测试 Rust 多阶段构建 (cargo build --release)
// 使用纯标准库实现 HTTP 服务器 + JSON 序列化

use std::collections::HashMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Clone)]
struct Todo {
    id: u64,
    title: String,
    done: bool,
    created_at: u64,
}

impl Todo {
    fn to_json(&self) -> String {
        format!(
            r#"{{"id":{},"title":"{}","done":{},"created_at":{}}}"#,
            self.id,
            self.title.replace('\\', r"\\").replace('"', r#"\""#),
            self.done,
            self.created_at
        )
    }
}

struct Store {
    todos: HashMap<u64, Todo>,
    next_id: u64,
}

impl Store {
    fn new() -> Self {
        Store {
            todos: HashMap::new(),
            next_id: 1,
        }
    }

    fn list(&self) -> String {
        let items: Vec<String> = self.todos.values().map(|t| t.to_json()).collect();
        format!("[{}]", items.join(","))
    }

    fn create(&mut self, title: &str) -> String {
        let id = self.next_id;
        self.next_id += 1;
        let todo = Todo {
            id,
            title: title.to_string(),
            done: false,
            created_at: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .unwrap()
                .as_secs(),
        };
        self.todos.insert(id, todo.clone());
        todo.to_json()
    }

    fn get(&self, id: u64) -> Option<&Todo> {
        self.todos.get(&id)
    }

    fn toggle(&mut self, id: u64, done: bool) -> Option<String> {
        if let Some(todo) = self.todos.get_mut(&id) {
            todo.done = done;
            return Some(todo.to_json());
        }
        None
    }

    fn delete(&mut self, id: u64) -> bool {
        self.todos.remove(&id).is_some()
    }

    fn count(&self) -> (usize, usize) {
        let total = self.todos.len();
        let done = self.todos.values().filter(|t| t.done).count();
        (total, done)
    }
}

fn extract_json_field(body: &str, field: &str) -> Option<String> {
    let pattern = format!(r#""{}""#, field);
    if let Some(pos) = body.find(&pattern) {
        let rest = &body[pos + pattern.len()..];
        if let Some(colon) = rest.find(':') {
            let rest = &rest[colon + 1..].trim_start();
            if rest.starts_with('"') {
                let rest = &rest[1..];
                let mut result = String::new();
                let mut chars = rest.chars().peekable();
                while let Some(c) = chars.next() {
                    if c == '\\' {
                        if let Some(&next) = chars.peek() {
                            result.push(next);
                            chars.next();
                        }
                    } else if c == '"' {
                        break;
                    } else {
                        result.push(c);
                    }
                }
                return Some(result);
            } else if rest.starts_with("true") {
                return Some("true".to_string());
            } else if rest.starts_with("false") {
                return Some("false".to_string());
            } else {
                let end = rest
                    .find(|c: char| c == ',' || c == '}' || c.is_whitespace())
                    .unwrap_or(rest.len());
                return Some(rest[..end].to_string());
            }
        }
    }
    None
}

fn handle_request(store: Arc<Mutex<Store>>, stream: TcpStream) {
    let mut reader = BufReader::new(stream.try_clone().unwrap());
    let mut request_line = String::new();
    if reader.read_line(&mut request_line).is_err() {
        return;
    }

    let parts: Vec<&str> = request_line.split_whitespace().collect();
    if parts.len() < 2 {
        return;
    }
    let method = parts[0];
    let path = parts[1];

    // Read headers to get content-length
    let mut content_length = 0;
    loop {
        let mut header = String::new();
        if reader.read_line(&mut header).is_err() {
            return;
        }
        if header.trim().is_empty() {
            break;
        }
        let lower = header.to_lowercase();
        if lower.starts_with("content-length:") {
            if let Ok(len) = lower["content-length:".len()..].trim().parse::<usize>() {
                content_length = len;
            }
        }
    }

    // Read body
    let mut body = String::new();
    if content_length > 0 {
        let mut buf = vec![0u8; content_length];
        if reader.read_exact(&mut buf).is_ok() {
            body = String::from_utf8_lossy(&buf).to_string();
        }
    }

    let (status, content_type, response_body) = route(store, method, path, &body);

    let response = format!(
        "HTTP/1.1 {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
        status,
        content_type,
        response_body.len(),
        response_body
    );

    let _ = stream.try_clone().unwrap().write_all(response.as_bytes());
}

fn route(store: Arc<Mutex<Store>>, method: &str, path: &str, body: &str) -> (&'static str, &'static str, String) {
    // Health check
    if method == "GET" && path == "/health" {
        return ("200 OK", "application/json", r#"{"status":"ok","service":"rust-api"}"#.to_string());
    }

    // API routes
    if path == "/api/todos" {
        match method {
            "GET" => {
                let s = store.lock().unwrap();
                return ("200 OK", "application/json", s.list());
            }
            "POST" => {
                if let Some(title) = extract_json_field(body, "title") {
                    if title.is_empty() {
                        return ("400 Bad Request", "application/json", r#"{"error":"title required"}"#.to_string());
                    }
                    let mut s = store.lock().unwrap();
                    let todo = s.create(&title);
                    return ("201 Created", "application/json", todo);
                }
                return ("400 Bad Request", "application/json", r#"{"error":"invalid body"}"#.to_string());
            }
            _ => {}
        }
    }

    if path.starts_with("/api/todos/") {
        let rest = &path["/api/todos/".len()..];
        let segments: Vec<&str> = rest.split('/').collect();
        if let Ok(id) = segments[0].parse::<u64>() {
            match method {
                "DELETE" => {
                    let mut s = store.lock().unwrap();
                    if s.delete(id) {
                        return ("204 No Content", "application/json", String::new());
                    }
                    return ("404 Not Found", "application/json", r#"{"error":"not found"}"#.to_string());
                }
                "PUT" => {
                    if segments.len() > 1 && segments[1] == "done" {
                        let done = extract_json_field(body, "done")
                            .map(|v| v == "true")
                            .unwrap_or(false);
                        let mut s = store.lock().unwrap();
                        if let Some(json) = s.toggle(id, done) {
                            return ("200 OK", "application/json", json);
                        }
                        return ("404 Not Found", "application/json", r#"{"error":"not found"}"#.to_string());
                    }
                }
                "GET" => {
                    let s = store.lock().unwrap();
                    if let Some(todo) = s.get(id) {
                        return ("200 OK", "application/json", todo.to_json());
                    }
                    return ("404 Not Found", "application/json", r#"{"error":"not found"}"#.to_string());
                }
                _ => {}
            }
        }
    }

    // Stats
    if path == "/api/stats" && method == "GET" {
        let s = store.lock().unwrap();
        let (total, done) = s.count();
        return (
            "200 OK",
            "application/json",
            format!(r#"{{"total":{},"done":{},"pending":{}}}"#, total, done, total - done),
        );
    }

    ("404 Not Found", "application/json", r#"{"error":"not found"}"#.to_string())
}

fn main() {
    let addr = std::env::var("LISTEN_ADDR").unwrap_or_else(|_| "0.0.0.0:8080".to_string());
    let listener = TcpListener::bind(&addr).expect("Failed to bind");
    let store = Arc::new(Mutex::new(Store::new()));

    println!("rust-api listening on {}", addr);

    for stream in listener.incoming() {
        if let Ok(stream) = stream {
            let store = Arc::clone(&store);
            thread::spawn(move || {
                handle_request(store, stream);
            });
        }
    }
}

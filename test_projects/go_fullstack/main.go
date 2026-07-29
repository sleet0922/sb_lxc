package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

//go:embed web/dist/*
var webFS embed.FS

type Todo struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Done      bool      `json:"done"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	mu    sync.RWMutex
	todos map[int]*Todo
	next  int
}

var store = &Store{todos: make(map[int]*Todo), next: 1}

func (s *Store) List() []*Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Todo, 0, len(s.todos))
	for _, t := range s.todos {
		list = append(list, t)
	}
	return list
}

func (s *Store) Create(title string) *Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t := &Todo{ID: s.next, Title: title, CreatedAt: time.Now()}
	s.todos[s.next] = t
	s.next++
	return t
}

func (s *Store) Get(id int) *Todo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.todos[id]
}

func (s *Store) Update(id int, done bool) *Todo {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.todos[id]
	if !ok {
		return nil
	}
	t.Done = done
	return t
}

func (s *Store) Delete(id int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.todos[id]; !ok {
		return false
	}
	delete(s.todos, id)
	return true
}

func main() {
	port := os.Getenv("LISTEN_ADDR")
	if port == "" {
		port = ":8080"
	}

	dist, err := fs.Sub(webFS, "web/dist")
	if err != nil {
		log.Fatalf("embed fs error: %v", err)
	}

	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/todos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			json.NewEncoder(w).Encode(store.List())
		case http.MethodPost:
			var body struct {
				Title string `json:"title"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `{"error":"invalid body"}`, 400)
				return
			}
			if body.Title == "" {
				http.Error(w, `{"error":"title required"}`, 400)
				return
			}
			t := store.Create(body.Title)
			w.WriteHeader(201)
			json.NewEncoder(w).Encode(t)
		default:
			http.Error(w, `{"error":"method not allowed"}`, 405)
		}
	})

	mux.HandleFunc("/api/todos/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/todos/"), "/")
		if len(parts) < 1 {
			http.Error(w, `{"error":"id required"}`, 400)
			return
		}
		id, err := strconv.Atoi(parts[0])
		if err != nil {
			http.Error(w, `{"error":"invalid id"}`, 400)
			return
		}

		switch r.Method {
		case http.MethodDelete:
			if !store.Delete(id) {
				http.Error(w, `{"error":"not found"}`, 404)
				return
			}
			w.WriteHeader(204)
		case http.MethodPut:
			if len(parts) < 2 || parts[1] != "done" {
				http.Error(w, `{"error":"invalid action"}`, 400)
				return
			}
			var body struct {
				Done bool `json:"done"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, `{"error":"invalid body"}`, 400)
				return
			}
			t := store.Update(id, body.Done)
			if t == nil {
				http.Error(w, `{"error":"not found"}`, 404)
				return
			}
			json.NewEncoder(w).Encode(t)
		default:
			http.Error(w, `{"error":"method not allowed"}`, 405)
		}
	})

	// Health check
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"ok","service":"go-fullstack"}`)
	})

	// Static files
	mux.Handle("/", http.FileServer(http.FS(dist)))

	log.Printf("go-fullstack listening on %s", port)
	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}

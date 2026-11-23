package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

type Store struct {
	mu    sync.RWMutex
	items []Todo
	seq   int
}

var db = &Store{items: []Todo{}}

// Вспомогательная функция для отправки JSON-ответов
func replyJSON(w http.ResponseWriter, code int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// GET /tasks и GET /tasks?done=true|false
func listTodos(w http.ResponseWriter, r *http.Request) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	doneParam := r.URL.Query().Get("done")
	if doneParam == "" {
		replyJSON(w, http.StatusOK, db.items)
		return
	}

	wantDone, err := strconv.ParseBool(doneParam)
	if err != nil {
		http.Error(w, "Некорректный параметр done (используйте true/false)", http.StatusBadRequest)
		return
	}

	filtered := []Todo{}
	for _, item := range db.items {
		if item.Done == wantDone {
			filtered = append(filtered, item)
		}
	}
	replyJSON(w, http.StatusOK, filtered)
}

// POST /tasks
func addTodo(w http.ResponseWriter, r *http.Request) {
	var in Todo
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Title) == "" {
		http.Error(w, "Неверный JSON или пустой title", http.StatusBadRequest)
		return
	}

	db.mu.Lock()
	db.seq++
	in.ID = db.seq
	db.items = append(db.items, in)
	db.mu.Unlock()

	replyJSON(w, http.StatusCreated, in)
}

// PUT /tasks/{id}
func updateTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный ID", http.StatusBadRequest)
		return
	}

	var patch Todo
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "Неверный формат JSON", http.StatusBadRequest)
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	for i := range db.items {
		if db.items[i].ID == id {
			if strings.TrimSpace(patch.Title) != "" {
				db.items[i].Title = patch.Title
			}
			db.items[i].Done = patch.Done
			replyJSON(w, http.StatusOK, db.items[i])
			return
		}
	}
	http.Error(w, "Задача не найдена", http.StatusNotFound)
}

// DELETE /tasks/{id}
func removeTodo(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		http.Error(w, "Некорректный ID", http.StatusBadRequest)
		return
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	for i, item := range db.items {
		if item.ID == id {
			db.items = append(db.items[:i], db.items[i+1:]...)
			replyJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
			return
		}
	}
	http.Error(w, "Задача не найдена", http.StatusNotFound)
}

func main() {
	router := http.NewServeMux()

	// Маршрутизация Go 1.22+ с поддержкой HTTP-методов и параметров {id}
	router.HandleFunc("GET /tasks", listTodos)
	router.HandleFunc("POST /tasks", addTodo)
	router.HandleFunc("PUT /tasks/{id}", updateTodo)
	router.HandleFunc("DELETE /tasks/{id}", removeTodo)

	fmt.Println("🚀 REST API запущен на http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}

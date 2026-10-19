package httpapi_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gribeiro239/go-tasks-api/internal/httpapi"
	"github.com/gribeiro239/go-tasks-api/internal/task"
)

func request(t *testing.T, h http.Handler, method, path, body string, want int) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s: status %d, esperado %d; %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}

func TestTaskLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	store, err := task.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(store)
	request(t, h, "GET", "/health", "", 200)
	if w := request(t, h, "GET", "/tasks", "", 200); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal(w.Body.String())
	}
	w := request(t, h, "POST", "/tasks", `{"title":"  Estudar Go  "}`, 201)
	var created task.Task
	if err := json.Unmarshal(w.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.ID != 1 || created.Title != "Estudar Go" || created.Completed || created.CreatedAt.IsZero() {
		t.Fatalf("tarefa inesperada: %+v", created)
	}
	if w.Header().Get("Location") != "/tasks/1" {
		t.Fatal("Location incorreto")
	}
	request(t, h, "GET", "/tasks/1", "", 200)
	request(t, h, "PATCH", "/tasks/1", `{"completed":true}`, 200)
	if w := request(t, h, "GET", "/tasks?completed=false", "", 200); strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal("filtro ignorado")
	}
	request(t, h, "PATCH", "/tasks/1", `{"completed":false,"title":"Praticar Go"}`, 200)
	reopened, err := task.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := reopened.Get(1)
	if err != nil || saved.Completed || saved.Title != "Praticar Go" {
		t.Fatalf("persistência incorreta: %+v, %v", saved, err)
	}
	request(t, h, "DELETE", "/tasks/1", "", 204)
	request(t, h, "GET", "/tasks/1", "", 404)
	request(t, h, "DELETE", "/tasks/1", "", 404)
	reopened, err = task.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.List(nil)) != 0 {
		t.Fatal("exclusão não persistiu")
	}
	next, err := reopened.Create("Outra tarefa")
	if err != nil || next.ID != 2 {
		t.Fatalf("ID reutilizado: %+v, %v", next, err)
	}
}

func TestInvalidRequests(t *testing.T) {
	store, err := task.Open(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	h := httpapi.New(store)
	for i, tc := range []struct {
		method, path, body string
		status             int
	}{
		{"POST", "/tasks", `{"title":" "}`, 400},
		{"POST", "/tasks", `{"title":42}`, 400},
		{"POST", "/tasks", `{"title":"Go","extra":true}`, 400},
		{"POST", "/tasks", `{"title":"Go"} {}`, 400},
		{"POST", "/tasks", `{"title":`, 400},
		{"POST", "/tasks", `null`, 400},
		{"POST", "/tasks", "", 400},
		{"POST", "/tasks", `{"title":"` + strings.Repeat("á", 201) + `"}`, 400},
		{"POST", "/tasks", `{"title":"` + strings.Repeat("a", 1<<20) + `"}`, 413},
		{"PATCH", "/tasks/1", `{}`, 400},
		{"PATCH", "/tasks/1", `{"completed":null}`, 400},
		{"PATCH", "/tasks/1", `{"completed":"false"}`, 400},
		{"PATCH", "/tasks/99", `{"completed":true}`, 404},
		{"GET", "/tasks/abc", "", 400},
		{"GET", "/tasks/0", "", 400},
		{"GET", "/tasks/99", "", 404},
		{"GET", "/tasks?completed=maybe", "", 400},
		{"PUT", "/tasks/1", `{}`, 405},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) { request(t, h, tc.method, tc.path, tc.body, tc.status) })
	}
	r := httptest.NewRequest("POST", "/tasks", strings.NewReader(`{"title":"Go"}`))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 415 {
		t.Fatalf("sem Content-Type: %d", w.Code)
	}
	if len(store.List(nil)) != 0 {
		t.Fatal("requisições inválidas alteraram os dados")
	}
}

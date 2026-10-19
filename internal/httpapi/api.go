// Package httpapi traduz requisições HTTP em operações sobre tarefas.
package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"strconv"

	"github.com/gribeiro239/go-tasks-api/internal/task"
)

type API struct{ store *task.Store }

func New(store *task.Store) http.Handler {
	a := &API{store: store}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /tasks", a.list)
	mux.HandleFunc("POST /tasks", a.create)
	mux.HandleFunc("GET /tasks/{id}", a.get)
	mux.HandleFunc("PATCH /tasks/{id}", a.update)
	mux.HandleFunc("DELETE /tasks/{id}", a.delete)
	return mux
}

func (a *API) list(w http.ResponseWriter, r *http.Request) {
	var completed *bool
	if r.URL.Query().Has("completed") {
		raw := r.URL.Query().Get("completed")
		if raw != "true" && raw != "false" {
			fail(w, 400, "completed deve ser true ou false")
			return
		}
		value := raw == "true"
		completed = &value
	}
	writeJSON(w, 200, a.store.List(completed))
}

func (a *API) create(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Title string `json:"title"`
	}
	if !readJSON(w, r, &input) {
		return
	}
	t, err := a.store.Create(input.Title)
	if err != nil {
		storeError(w, err)
		return
	}
	w.Header().Set("Location", "/tasks/"+strconv.Itoa(t.ID))
	writeJSON(w, http.StatusCreated, t)
}

func (a *API) get(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	t, err := a.store.Get(id)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, t)
}

func (a *API) update(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	var input task.Update
	if !readJSON(w, r, &input) {
		return
	}
	if input.Title == nil && input.Completed == nil {
		fail(w, 400, "informe title ou completed")
		return
	}
	t, err := a.store.Update(id, input)
	if err != nil {
		storeError(w, err)
		return
	}
	writeJSON(w, 200, t)
}

func (a *API) delete(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := a.store.Delete(id); err != nil {
		storeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func taskID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id < 1 {
		fail(w, 400, "id deve ser um inteiro positivo")
		return 0, false
	}
	return id, true
}

func readJSON(w http.ResponseWriter, r *http.Request, dest any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		fail(w, 415, "use Content-Type: application/json")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		jsonError(w, err)
		return false
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			fail(w, 400, "envie apenas um objeto JSON")
		} else {
			jsonError(w, err)
		}
		return false
	}
	return true
}

func jsonError(w http.ResponseWriter, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(w, 413, "corpo excede 1 MiB")
		return
	}
	fail(w, 400, "JSON inválido, vazio, com campos desconhecidos ou tipos incorretos")
}

func storeError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, task.ErrNotFound):
		fail(w, 404, err.Error())
	case errors.Is(err, task.ErrTitle):
		fail(w, 400, err.Error())
	default:
		slog.Error("falha na persistência", "error", err)
		fail(w, 500, "não foi possível salvar ou consultar a tarefa")
	}
}

func fail(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("enviar resposta", "error", err)
	}
}

// Package task contém o modelo de tarefa e sua persistência em JSON.
package task

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var (
	ErrNotFound = errors.New("tarefa não encontrada")
	ErrTitle    = errors.New("o título deve ter entre 1 e 200 caracteres")
)

type Task struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Completed bool      `json:"completed"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Ponteiros distinguem um campo ausente de completed: false em um PATCH.
type Update struct {
	Title     *string `json:"title"`
	Completed *bool   `json:"completed"`
}

type snapshot struct {
	NextID int    `json:"next_id"`
	Tasks  []Task `json:"tasks"`
}

// Store serializa as operações de escrita e protege os dados entre goroutines.
// Use apenas um processo por arquivo de dados.
type Store struct {
	mu    sync.RWMutex
	path  string
	state snapshot
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, state: snapshot{NextID: 1, Tasks: []Task{}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ler tarefas: %w", err)
	}
	if err := json.Unmarshal(data, &s.state); err != nil {
		return nil, fmt.Errorf("arquivo de tarefas inválido: %w", err)
	}
	if s.state.NextID < 1 {
		return nil, errors.New("next_id inválido no arquivo de tarefas")
	}
	seen := make(map[int]bool)
	for _, t := range s.state.Tasks {
		if t.ID < 1 || t.ID >= s.state.NextID || seen[t.ID] {
			return nil, errors.New("IDs inválidos no arquivo de tarefas")
		}
		seen[t.ID] = true
		if _, err := cleanTitle(t.Title); err != nil {
			return nil, err
		}
	}
	if s.state.Tasks == nil {
		s.state.Tasks = []Task{}
	}
	return s, nil
}

func cleanTitle(title string) (string, error) {
	title = strings.TrimSpace(title)
	if n := utf8.RuneCountInString(title); n < 1 || n > 200 {
		return "", ErrTitle
	}
	return title, nil
}

func (s *Store) List(completed *bool) []Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Task, 0, len(s.state.Tasks))
	for _, t := range s.state.Tasks {
		if completed == nil || t.Completed == *completed {
			result = append(result, t)
		}
	}
	return result
}

func (s *Store) Get(id int) (Task, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.state.Tasks {
		if t.ID == id {
			return t, nil
		}
	}
	return Task{}, ErrNotFound
}

func (s *Store) Create(title string) (Task, error) {
	title, err := cleanTitle(title)
	if err != nil {
		return Task{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	t := Task{ID: s.state.NextID, Title: title, CreatedAt: now, UpdatedAt: now}
	next := s.clone()
	next.NextID++
	next.Tasks = append(next.Tasks, t)
	if err := s.commit(next); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (s *Store) Update(id int, update Update) (Task, error) {
	if update.Title != nil {
		title, err := cleanTitle(*update.Title)
		if err != nil {
			return Task{}, err
		}
		update.Title = &title
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.clone()
	for i, t := range next.Tasks {
		if t.ID != id {
			continue
		}
		if update.Title != nil {
			t.Title = *update.Title
		}
		if update.Completed != nil {
			t.Completed = *update.Completed
		}
		t.UpdatedAt = time.Now().UTC()
		next.Tasks[i] = t
		if err := s.commit(next); err != nil {
			return Task{}, err
		}
		return t, nil
	}
	return Task{}, ErrNotFound
}

func (s *Store) Delete(id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.clone()
	for i, t := range next.Tasks {
		if t.ID != id {
			continue
		}
		next.Tasks = append(next.Tasks[:i], next.Tasks[i+1:]...)
		return s.commit(next)
	}
	return ErrNotFound
}

// Só é chamado com o mutex de escrita adquirido.
func (s *Store) clone() snapshot {
	tasks := make([]Task, len(s.state.Tasks))
	copy(tasks, s.state.Tasks)
	return snapshot{NextID: s.state.NextID, Tasks: tasks}
}

// Grava em um temporário no mesmo diretório antes de substituir o arquivo.
// A memória só muda depois que a persistência tem sucesso.
func (s *Store) commit(next snapshot) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("criar diretório: %w", err)
	}
	f, err := os.CreateTemp(dir, ".tasks-*.tmp")
	if err != nil {
		return fmt.Errorf("criar temporário: %w", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(f.Name(), s.path); err != nil {
		return fmt.Errorf("salvar tarefas: %w", err)
	}
	s.state = next
	return nil
}

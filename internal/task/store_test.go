package task_test

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/gribeiro239/go-tasks-api/internal/task"
)

func TestConcurrentCreates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	s, err := task.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Create("Concorrência"); err != nil {
				t.Error(err)
			}
			_ = s.List(nil)
		}()
	}
	wg.Wait()
	reopened, err := task.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	tasks := reopened.List(nil)
	if len(tasks) != 20 {
		t.Fatalf("esperado 20 tarefas, recebido %d", len(tasks))
	}
	for i, item := range tasks {
		if item.ID != i+1 {
			t.Fatalf("ID inesperado: %d", item.ID)
		}
	}
}

func TestWriteFailurePreservesMemory(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "data")
	s, err := task.Open(filepath.Join(parent, "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(parent, []byte("bloqueia a criação do diretório"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create("Não deve entrar na memória"); err == nil {
		t.Fatal("esperada falha na gravação")
	}
	if len(s.List(nil)) != 0 {
		t.Fatal("memória alterada após falha")
	}
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	created, err := s.Create("Agora funciona")
	if err != nil || created.ID != 1 {
		t.Fatalf("ID consumido após falha: %+v, %v", created, err)
	}
}

func TestCorruptFileIsPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	content := []byte(`{"tasks":`)
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := task.Open(path); err == nil {
		t.Fatal("arquivo inválido foi aceito")
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != string(content) {
		t.Fatal("arquivo corrompido foi sobrescrito")
	}
}

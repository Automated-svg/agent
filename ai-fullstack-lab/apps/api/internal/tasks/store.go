package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"example.com/ai-fullstack-lab/api/internal/api"
)

var ErrNotFound = errors.New("task not found")

// Store 面向单个后端进程；更新先落盘，成功后再替换内存快照。
type Store struct {
	mu    sync.RWMutex
	path  string
	items []api.Task
}

func Open(path string) (*Store, error) {
	s := &Store{path: path, items: []api.Task{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &s.items); err != nil {
		return nil, err
	}
	if s.items == nil {
		s.items = []api.Task{}
	}
	seen := make(map[string]bool, len(s.items))
	for index, item := range s.items {
		field := ""
		switch {
		case strings.TrimSpace(item.Id) == "":
			field = "id"
		case seen[item.Id]:
			field = "duplicate id"
		case strings.TrimSpace(item.Title) == "" || utf8.RuneCountInString(item.Title) > 120:
			field = "title"
		case item.Status != api.Todo && item.Status != api.Doing && item.Status != api.Done:
			field = "status"
		case item.CreatedAt.IsZero():
			field = "createdAt"
		}
		if field != "" {
			return nil, fmt.Errorf("stored task %d has invalid %s", index, field)
		}
		seen[item.Id] = true
	}
	return s, nil
}

func (s *Store) List() []api.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]api.Task{}, s.items...)
}

func (s *Store) commit(items []api.Task) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(s.path), ".tasks-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = os.Rename(file.Name(), s.path); err != nil {
		return err
	}
	s.items = items
	return nil
}

func (s *Store) Create(title string) (api.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return api.Task{}, err
	}
	task := api.Task{Id: hex.EncodeToString(id), Title: title, Status: api.Todo, CreatedAt: time.Now().UTC()}
	next := append(append([]api.Task{}, s.items...), task)
	return task, s.commit(next)
}

func (s *Store) Update(id string, status api.TaskStatus) (api.Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := append([]api.Task{}, s.items...)
	for index, item := range next {
		if item.Id == id {
			next[index].Status = status
			return next[index], s.commit(next)
		}
	}
	return api.Task{}, ErrNotFound
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := make([]api.Task, 0, len(s.items))
	found := false
	for _, item := range s.items {
		if item.Id == id {
			found = true
		} else {
			next = append(next, item)
		}
	}
	if !found {
		return ErrNotFound
	}
	return s.commit(next)
}

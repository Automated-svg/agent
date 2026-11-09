package tasks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"example.com/ai-fullstack-lab/api/internal/api"
)

func TestOpenRejectsInvalidStoredTasks(t *testing.T) {
	valid := api.Task{Id: "legacy-task", Title: "历史任务", Status: api.Todo, CreatedAt: time.Now().UTC()}
	cases := []struct {
		name string
		edit func(*api.Task)
	}{
		{"missing_id", func(task *api.Task) { task.Id = "" }},
		{"blank_title", func(task *api.Task) { task.Title = "  " }},
		{"long_title", func(task *api.Task) { task.Title = strings.Repeat("中", 121) }},
		{"invalid_status", func(task *api.Task) { task.Status = "unknown" }},
		{"missing_creation_time", func(task *api.Task) { task.CreatedAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			item := valid
			tc.edit(&item)
			path := filepath.Join(t.TempDir(), "tasks.json")
			data, err := json.Marshal([]api.Task{item})
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("invalid stored task accepted")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(data) {
				t.Fatal("failed startup modified the original file")
			}
		})
	}
	t.Run("duplicate_ids", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "tasks.json")
		data, _ := json.Marshal([]api.Task{valid, valid})
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Open(path); err == nil {
			t.Fatal("duplicate stored IDs accepted")
		}
	})
}

func TestOpenKeepsValidLegacyTasksAndEmptyLists(t *testing.T) {
	for _, body := range []string{"[]", "null", `[{"id":"legacy","title":"历史任务","status":"doing","createdAt":"2025-01-01T00:00:00Z"}]`} {
		t.Run(body, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "tasks.json")
			if err := os.WriteFile(path, []byte(body), 0600); err != nil {
				t.Fatal(err)
			}
			store, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if store.List() == nil {
				t.Fatal("empty list must not become null in API response")
			}
			if body != "[]" && body != "null" && store.List()[0].Id != "legacy" {
				t.Fatal("legacy task changed")
			}
		})
	}
}

func TestRenameFailureRollsBackEveryMutation(t *testing.T) {
	for _, operation := range []string{"create", "update", "delete"} {
		t.Run(operation, func(t *testing.T) {
			originalPath := filepath.Join(t.TempDir(), "tasks.json")
			store, err := Open(originalPath)
			if err != nil {
				t.Fatal(err)
			}
			task, err := store.Create("保留原任务")
			if err != nil {
				t.Fatal(err)
			}
			before := store.List()
			blocked := filepath.Join(t.TempDir(), "existing-directory")
			if err := os.Mkdir(blocked, 0700); err != nil {
				t.Fatal(err)
			}
			store.path = blocked // 临时文件可写，但不能用普通文件替换这个目录。
			switch operation {
			case "create":
				_, err = store.Create("不能保存")
			case "update":
				_, err = store.Update(task.Id, api.Done)
			case "delete":
				err = store.Delete(task.Id)
			}
			if err == nil {
				t.Fatal("expected rename failure")
			}
			if !reflect.DeepEqual(store.List(), before) {
				t.Fatal("failed write changed memory")
			}
			reopened, err := Open(originalPath)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(reopened.List(), before) {
				t.Fatal("failed write changed stored snapshot")
			}
			leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(blocked), ".tasks-*"))
			if err != nil {
				t.Fatal(err)
			}
			if len(leftovers) != 0 {
				t.Fatal("failed write left temporary files")
			}
		})
	}
}

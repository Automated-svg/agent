package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"example.com/ai-fullstack-lab/api/internal/api"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

func setup(t *testing.T) (*Store, http.Handler) {
	t.Helper()
	store, err := Open(filepath.Join(t.TempDir(), "tasks.json"))
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHandler(store)
	if err != nil {
		t.Fatal(err)
	}
	return store, handler
}

// 每个请求同时验证响应是否符合 OpenAPI，防止只生成类型却漏掉运行行为。
func call(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != status {
		t.Fatalf("%s %s: want %d got %d: %s", method, path, status, res.Code, res.Body.String())
	}
	spec, err := api.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = nil
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatal(err)
	}
	input := &openapi3filter.ResponseValidationInput{RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route}, Status: res.Code, Header: res.Header()}
	input.SetBodyBytes(res.Body.Bytes())
	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Fatalf("response violates contract: %v", err)
	}
	return res
}

func TestLifecycleAndPersistence(t *testing.T) {
	store, handler := setup(t)
	call(t, handler, "GET", "/api/v1/health", "", 200)
	empty := call(t, handler, "GET", "/api/v1/tasks", "", 200)
	if strings.TrimSpace(empty.Body.String()) != "[]" {
		t.Fatal(empty.Body.String())
	}
	created := call(t, handler, "POST", "/api/v1/tasks", `{"title":"实现联调"}`, 201)
	var task api.Task
	if err := json.Unmarshal(created.Body.Bytes(), &task); err != nil {
		t.Fatal(err)
	}
	if task.Id == "" || task.Status != api.Todo {
		t.Fatalf("unexpected task: %+v", task)
	}
	path := "/api/v1/tasks/" + task.Id
	call(t, handler, "PATCH", path, `{"status":"done"}`, 200)
	reopened, err := Open(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.List()) != 1 || reopened.List()[0].Status != api.Done {
		t.Fatal("data lost on reopen")
	}
	call(t, handler, "DELETE", path, "", 204)
	call(t, handler, "DELETE", path, "", 404)
	call(t, handler, "PATCH", path, `{"status":"doing"}`, 404)
}

func TestInvalidRequests(t *testing.T) {
	_, handler := setup(t)
	for _, body := range []string{`{}`, `{"title":""}`, `{"title":"   "}`, `{"title":null}`, `{"title":12}`, `{"title":"ok","extra":true}`, `{"title":"` + strings.Repeat("中", 121) + `"}`, `null`, `{`} {
		t.Run(body, func(t *testing.T) { call(t, handler, "POST", "/api/v1/tasks", body, 400) })
	}
	call(t, handler, "PATCH", "/api/v1/tasks/missing", `{"status":"invalid"}`, 400)
	call(t, handler, "POST", "/api/v1/tasks", `{"title":"`+strings.Repeat("中", 120)+`"}`, 201)
}

func TestFailedWriteDoesNotMutateMemory(t *testing.T) {
	store, handler := setup(t)
	store.path = filepath.Join(t.TempDir(), "missing-parent", "file")
	// 在路径中放置文件，使 MkdirAll 确定性失败，避免依赖 chmod/root 权限。
	if err := os.WriteFile(filepath.Dir(store.path), []byte("blocking file"), 0600); err != nil {
		t.Fatal(err)
	}
	call(t, handler, "POST", "/api/v1/tasks", `{"title":"save fails"}`, 500)
	if len(store.List()) != 0 {
		t.Fatal("failed write leaked into memory")
	}
}

func TestConcurrentCreates(t *testing.T) {
	store, _ := setup(t)
	var group sync.WaitGroup
	for i := 0; i < 12; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			if _, err := store.Create(fmt.Sprintf("task %d", index)); err != nil {
				t.Error(err)
			}
		}(i)
	}
	group.Wait()
	if len(store.List()) != 12 {
		t.Fatal("lost concurrent writes")
	}
	reopened, err := Open(store.path)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.List()) != 12 {
		t.Fatal("disk snapshot is incomplete")
	}
}

func TestOversizedBody(t *testing.T) {
	_, handler := setup(t)
	req := httptest.NewRequest("POST", "/api/v1/tasks", bytes.NewBufferString(`{"title":"`+strings.Repeat("x", 20000)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code < 400 || res.Code >= 500 {
		t.Fatalf("unexpected status %d", res.Code)
	}
}

func TestUnsupportedContentType(t *testing.T) {
	_, handler := setup(t)
	req := httptest.NewRequest("POST", "/api/v1/tasks", strings.NewReader(`{"title":"task"}`))
	req.Header.Set("Content-Type", "text/plain")
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	if res.Code != 400 {
		t.Fatalf("want declared 400, got %d", res.Code)
	}
	var body api.Error
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "bad_request" {
		t.Fatalf("unexpected error: %+v", body)
	}
}

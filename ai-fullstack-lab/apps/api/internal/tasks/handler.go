package tasks

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"example.com/ai-fullstack-lab/api/internal/api"
	middleware "github.com/oapi-codegen/nethttp-middleware"
)

type service struct{ store *Store }

var _ api.StrictServerInterface = (*service)(nil)

func (s *service) GetHealth(context.Context, api.GetHealthRequestObject) (api.GetHealthResponseObject, error) {
	return api.GetHealth200JSONResponse{Status: "ok"}, nil
}
func (s *service) ListTasks(context.Context, api.ListTasksRequestObject) (api.ListTasksResponseObject, error) {
	return api.ListTasks200JSONResponse(s.store.List()), nil
}
func (s *service) CreateTask(_ context.Context, req api.CreateTaskRequestObject) (api.CreateTaskResponseObject, error) {
	if req.Body == nil {
		return nil, errors.New("missing validated body")
	}
	title := strings.TrimSpace(req.Body.Title)
	if title == "" || utf8.RuneCountInString(title) > 120 {
		return api.CreateTask400JSONResponse{BadRequestJSONResponse: api.BadRequestJSONResponse{Code: "bad_request", Message: "标题需要 1–120 个字符"}}, nil
	}
	task, err := s.store.Create(title)
	if err != nil {
		return nil, err
	}
	return api.CreateTask201JSONResponse(task), nil
}
func (s *service) UpdateTask(_ context.Context, req api.UpdateTaskRequestObject) (api.UpdateTaskResponseObject, error) {
	if req.Body == nil {
		return nil, errors.New("missing validated body")
	}
	task, err := s.store.Update(req.Id, req.Body.Status)
	if errors.Is(err, ErrNotFound) {
		return api.UpdateTask404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Code: "not_found", Message: "任务不存在"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.UpdateTask200JSONResponse(task), nil
}
func (s *service) DeleteTask(_ context.Context, req api.DeleteTaskRequestObject) (api.DeleteTaskResponseObject, error) {
	err := s.store.Delete(req.Id)
	if errors.Is(err, ErrNotFound) {
		return api.DeleteTask404JSONResponse{NotFoundJSONResponse: api.NotFoundJSONResponse{Code: "not_found", Message: "任务不存在"}}, nil
	}
	if err != nil {
		return nil, err
	}
	return api.DeleteTask204Response{}, nil
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(api.Error{Code: code, Message: message})
}

// NewHandler 将 OpenAPI 请求校验置于生成路由和业务处理之前。
func NewHandler(store *Store) (http.Handler, error) {
	spec, err := api.GetSwagger()
	if err != nil {
		return nil, err
	}
	spec.Servers = nil
	strict := api.NewStrictHandlerWithOptions(&service{store}, nil, api.StrictHTTPServerOptions{
		RequestErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			writeError(w, 400, "bad_request", "请求格式不正确")
		},
		ResponseErrorHandlerFunc: func(w http.ResponseWriter, r *http.Request, err error) {
			slog.Error("request failed", "method", r.Method, "error", err)
			writeError(w, 500, "internal_error", "服务暂时无法保存任务")
		},
	})
	validated := middleware.OapiRequestValidatorWithOptions(spec, &middleware.Options{
		ErrorHandler: func(w http.ResponseWriter, message string, status int) {
			// 校验器的请求格式错误统一映射到契约声明的 400。
			code := "bad_request"
			if status == 404 {
				code = "not_found"
			} else {
				status = http.StatusBadRequest
			}
			writeError(w, status, code, "请求不符合 API 契约")
		},
	})(api.Handler(strict))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
		w.Header().Set("Cache-Control", "no-store")
		validated.ServeHTTP(w, r)
	}), nil
}

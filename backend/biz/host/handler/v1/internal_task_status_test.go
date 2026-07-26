package v1

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/GoYoko/web"
	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/Y-vQv-Y/DevLoom/backend/consts"
	"github.com/Y-vQv-Y/DevLoom/backend/pkg/lifecycle"
)

func TestInternalHostHandlerTaskStatusCompletesLifecycle(t *testing.T) {
	redisServer := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	manager := lifecycle.NewManager[uuid.UUID, consts.TaskStatus, lifecycle.TaskMetadata](
		rdb,
		lifecycle.WithTransitions[uuid.UUID, consts.TaskStatus, lifecycle.TaskMetadata](lifecycle.TaskTransitions()),
	)
	taskID := uuid.New()
	ctx := context.Background()
	if err := manager.Transition(ctx, taskID, consts.TaskStatusPending, lifecycle.TaskMetadata{TaskID: taskID}); err != nil {
		t.Fatal(err)
	}
	if err := manager.Transition(ctx, taskID, consts.TaskStatusProcessing, lifecycle.TaskMetadata{TaskID: taskID}); err != nil {
		t.Fatal(err)
	}

	handler := &InternalHostHandler{
		logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		taskLifecycle: manager,
	}
	w := web.New()
	w.POST("/internal/task-status", web.BindHandler(handler.TaskStatus))
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/internal/task-status", strings.NewReader(`{"id":"`+taskID.String()+`","status":"completed"}`))
	request.Header.Set("Content-Type", "application/json")
	w.Echo().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	state, err := manager.GetState(ctx, taskID)
	if err != nil {
		t.Fatal(err)
	}
	if state != consts.TaskStatusFinished {
		t.Fatalf("task state = %q, want %q", state, consts.TaskStatusFinished)
	}
}

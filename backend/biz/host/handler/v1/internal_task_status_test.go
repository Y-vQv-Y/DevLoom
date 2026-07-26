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

func TestInternalHostHandlerTaskStatusPreservesFinishReason(t *testing.T) {
	for _, tc := range []struct {
		status string
		want   consts.TaskFinishReason
	}{
		{status: "completed", want: consts.TaskFinishReasonCompleted},
		{status: "cancelled", want: consts.TaskFinishReasonCancelled},
	} {
		t.Run(tc.status, func(t *testing.T) {
			redisServer := miniredis.RunT(t)
			rdb := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
			t.Cleanup(func() { _ = rdb.Close() })
			manager := lifecycle.NewManager[uuid.UUID, consts.TaskStatus, lifecycle.TaskMetadata](
				rdb,
				lifecycle.WithTransitions[uuid.UUID, consts.TaskStatus, lifecycle.TaskMetadata](lifecycle.TaskTransitions()),
			)
			recorderHook := &finishReasonRecorder{}
			manager.Register(recorderHook)
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
			httpRecorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/internal/task-status", strings.NewReader(`{"id":"`+taskID.String()+`","status":"`+tc.status+`"}`))
			request.Header.Set("Content-Type", "application/json")
			w.Echo().ServeHTTP(httpRecorder, request)
			if httpRecorder.Code != http.StatusOK {
				t.Fatalf("status = %d, body = %s", httpRecorder.Code, httpRecorder.Body.String())
			}
			state, err := manager.GetState(ctx, taskID)
			if err != nil {
				t.Fatal(err)
			}
			if state != consts.TaskStatusFinished {
				t.Fatalf("task state = %q, want %q", state, consts.TaskStatusFinished)
			}
			if recorderHook.metadata.FinishReason != tc.want {
				t.Fatalf("finish reason = %q, want %q", recorderHook.metadata.FinishReason, tc.want)
			}
		})
	}
}

type finishReasonRecorder struct {
	metadata lifecycle.TaskMetadata
}

func (*finishReasonRecorder) Name() string  { return "finish-reason-recorder" }
func (*finishReasonRecorder) Priority() int { return 0 }
func (*finishReasonRecorder) Async() bool   { return false }
func (r *finishReasonRecorder) OnStateChange(_ context.Context, _ uuid.UUID, _, to consts.TaskStatus, metadata lifecycle.TaskMetadata) error {
	if to == consts.TaskStatusFinished {
		r.metadata = metadata
	}
	return nil
}

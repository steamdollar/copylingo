package miniapp

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/service"
)

type mockTipRepo struct {
	listActiveFn func(ctx context.Context, language, level string, limit int) ([]model.Tip, error)
}

func (m *mockTipRepo) ListActive(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
	return m.listActiveFn(ctx, language, level, limit)
}

func TestListTips(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		repo := &mockTipRepo{
			listActiveFn: func(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
				return []model.Tip{
					{ID: 1, Language: "ja", ProficiencyLevel: "N5", Category: "kana_shape", Body: "Tip 1"},
				}, nil
			},
		}
		tipSvc := service.NewTipService(repo)
		services := &service.Services{Tip: tipSvc}
		handler := NewHandler(services, nil, nil, nil, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/miniapp/tips?language=ja&level=N5", nil)

		handler.ListTips(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}

		var tips []model.Tip
		if err := json.Unmarshal(w.Body.Bytes(), &tips); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if len(tips) != 1 || tips[0].Body != "Tip 1" {
			t.Errorf("unexpected response: %v", tips)
		}
	})

	t.Run("missing parameters", func(t *testing.T) {
		handler := NewHandler(&service.Services{}, nil, nil, nil, nil)
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/miniapp/tips?language=ja", nil)

		handler.ListTips(c)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}
	})

	t.Run("empty result returns array", func(t *testing.T) {
		repo := &mockTipRepo{
			listActiveFn: func(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
				return nil, nil
			},
		}
		tipSvc := service.NewTipService(repo)
		services := &service.Services{Tip: tipSvc}
		handler := NewHandler(services, nil, nil, nil, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/miniapp/tips?language=ja&level=N5", nil)

		handler.ListTips(c)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", w.Code)
		}
		if got := w.Body.String(); got != "[]" {
			t.Errorf("expected empty JSON array, got %q", got)
		}
	})

	t.Run("repo error", func(t *testing.T) {
		repo := &mockTipRepo{
			listActiveFn: func(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
				return nil, errors.New("db down")
			},
		}
		tipSvc := service.NewTipService(repo)
		services := &service.Services{Tip: tipSvc}
		handler := NewHandler(services, nil, nil, nil, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/miniapp/tips?language=ja&level=N5", nil)

		handler.ListTips(c)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status 500, got %d", w.Code)
		}
	})

	t.Run("limit clamp", func(t *testing.T) {
		var capturedLimit int
		repo := &mockTipRepo{
			listActiveFn: func(ctx context.Context, language, level string, limit int) ([]model.Tip, error) {
				capturedLimit = limit
				return []model.Tip{}, nil
			},
		}
		tipSvc := service.NewTipService(repo)
		services := &service.Services{Tip: tipSvc}
		handler := NewHandler(services, nil, nil, nil, nil)

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request, _ = http.NewRequest("GET", "/api/miniapp/tips?language=ja&level=N5&limit=999", nil)

		handler.ListTips(c)

		if capturedLimit != 50 {
			t.Errorf("expected limit 50, got %d", capturedLimit)
		}
	})
}

func TestParseHandwritingMessageRef(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		raw        string
		wantChatID int64
		wantMsgID  int
		wantErr    bool
	}{
		{name: "valid private chat", raw: "12345:678", wantChatID: 12345, wantMsgID: 678},
		{name: "valid group chat", raw: "-10012345:678", wantChatID: -10012345, wantMsgID: 678},
		{name: "missing separator", raw: "12345", wantErr: true},
		{name: "bad chat id", raw: "abc:678", wantErr: true},
		{name: "zero chat id", raw: "0:678", wantErr: true},
		{name: "bad message id", raw: "12345:abc", wantErr: true},
		{name: "zero message id", raw: "12345:0", wantErr: true},
		{name: "negative message id", raw: "12345:-1", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotChatID, gotMsgID, err := parseHandwritingMessageRef(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotChatID != tt.wantChatID || gotMsgID != tt.wantMsgID {
				t.Fatalf("parseHandwritingMessageRef()=(%d,%d), want (%d,%d)",
					gotChatID, gotMsgID, tt.wantChatID, tt.wantMsgID)
			}
		})
	}
}

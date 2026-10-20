package miniapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/observability"
	"github.com/lsj/copylingo/internal/service"
)

func TestRefreshHandwritingMessagePreservesParentCorrelation(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(observability.NewContextHandler(slog.NewJSONHandler(
		&output,
		nil,
	))))
	defer slog.SetDefault(previous)

	ctx := observability.WithAttrs(
		context.Background(),
		slog.String(
			"interaction_id",
			"http-parent",
		),
	)
	handler := NewHandler(HandlerDeps{})
	handler.refreshHandwritingMessage(
		context.WithoutCancel(ctx),
		1,
		2,
	)

	var entry map[string]any
	if err := json.Unmarshal(
		output.Bytes(),
		&entry,
	); err != nil {
		t.Fatalf(
			"json.Unmarshal(%q) error = %v",
			output.Bytes(),
			err,
		)
	}
	if got := entry["interaction_id"]; got != "http-parent" {
		t.Fatalf(
			"entry[interaction_id] = %#v, want http-parent",
			got,
		)
	}
	if got := entry["event"]; got != "handwriting.cleanup.skipped" {
		t.Fatalf(
			"entry[event] = %#v, want handwriting.cleanup.skipped",
			got,
		)
	}
}

// fakeHandwritingScreen records the graded question the Mini App reports.
type fakeHandwritingScreen struct {
	sessionID   int
	questionID  int
	hasDeadline bool
}

func (f *fakeHandwritingScreen) ShowHandwritingGraded(
	ctx context.Context,
	sessionID,
	questionID int,
) {
	f.sessionID = sessionID
	f.questionID = questionID
	_, f.hasDeadline = ctx.Deadline()
}

func TestRefreshHandwritingMessageReportsGradedQuestionWithTimeout(t *testing.T) {
	screen := &fakeHandwritingScreen{}
	handler := NewHandler(HandlerDeps{HandwritingScreen: screen})

	handler.refreshHandwritingMessage(
		context.Background(),
		1,
		2,
	)

	if screen.sessionID != 1 || screen.questionID != 2 || !screen.hasDeadline {
		t.Fatalf(
			"screen got %+v, want session 1 question 2 with a deadline",
			*screen,
		)
	}
}

// mockQuizSession fakes service.SessionService's Mini App surface.
type mockQuizSession struct {
	submitHandwritingFn func(
		ctx context.Context,
		req service.HandwritingSubmitRequest,
	) (*service.HandwritingSubmitResult, error)
}

func (m *mockQuizSession) SubmitHandwriting(
	ctx context.Context,
	req service.HandwritingSubmitRequest,
) (*service.HandwritingSubmitResult, error) {
	return m.submitHandwritingFn(
		ctx,
		req,
	)
}

type mockVerifier struct {
	verifyFn func(initData string) (*TelegramUser, error)
}

func (m *mockVerifier) Verify(initData string) (*TelegramUser, error) {
	return m.verifyFn(initData)
}

func TestSubmitHandwriting_ErrorSanitization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		serviceErr    error
		wantStatus    int
		wantBody      string
		notWantInBody string
	}{
		{
			name: "unknown llm error sanitized",
			serviceErr: errors.New(
				`grade handwriting answer: llm handwriting grading request failed: error, status code: 500, body: {"error":"provider internal stack"}`,
			),
			wantStatus:    http.StatusServiceUnavailable,
			wantBody:      "현재 AI 채점이 지연되고 있습니다. 잠시 후 다시 시도해 주세요.",
			notWantInBody: "provider internal stack",
		},
		{
			name:       "unauthorized error mapping",
			serviceErr: service.ErrHandwritingUnauthorized,
			wantStatus: http.StatusForbidden,
			wantBody:   "권한이 없습니다.",
		},
		{
			name:       "already answered mapping",
			serviceErr: service.ErrHandwritingAlreadyAnswered,
			wantStatus: http.StatusConflict,
			wantBody:   "이미 채점된 문항입니다.",
		},
		{
			name:       "invalid question mapping",
			serviceErr: service.ErrHandwritingInvalidQuestion,
			wantStatus: http.StatusBadRequest,
			wantBody:   "손글씨 제출 정보를 확인할 수 없습니다.",
		},
		{
			name:       "ai unavailable mapping",
			serviceErr: service.ErrAIUnavailable,
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "현재 AI 채점 설정을 사용할 수 없습니다.",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				handwriting := &mockQuizSession{
					submitHandwritingFn: func(
						ctx context.Context,
						req service.HandwritingSubmitRequest,
					) (*service.HandwritingSubmitResult, error) {
						return nil, tt.serviceErr
					},
				}
				verifier := &mockVerifier{
					verifyFn: func(initData string) (*TelegramUser, error) {
						return &TelegramUser{ID: 123}, nil
					},
				}
				handler := NewHandler(HandlerDeps{
					Session:  handwriting,
					Verifier: verifier,
				})

				w := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(w)
				reqBody := `{"init_data":"dummy","session_id":1,"question_id":1,"strokes":[]}`
				c.Request, _ = http.NewRequest(
					"POST",
					"/api/miniapp/handwriting/submit",
					strings.NewReader(reqBody),
				)
				c.Request.Header.Set(
					"Content-Type",
					"application/json",
				)

				handler.SubmitHandwriting(c)

				if w.Code != tt.wantStatus {
					t.Errorf(
						"expected status %d, got %d",
						tt.wantStatus,
						w.Code,
					)
				}

				var resp map[string]string
				if err := json.Unmarshal(
					w.Body.Bytes(),
					&resp,
				); err != nil {
					t.Fatalf(
						"failed to unmarshal response: %v",
						err,
					)
				}

				if got := resp["error"]; got != tt.wantBody {
					t.Errorf(
						"expected error message %q, got %q",
						tt.wantBody,
						got,
					)
				}

				if tt.notWantInBody != "" && strings.Contains(
					w.Body.String(),
					tt.notWantInBody,
				) {
					t.Errorf(
						"response leaked sensitive information: %s",
						w.Body.String(),
					)
				}
			},
		)
	}
}

func TestSubmitHandwriting_AuthAndBind(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run(
		"invalid json",
		func(t *testing.T) {
			handler := NewHandler(HandlerDeps{})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(`{invalid`),
			)
			handler.SubmitHandwriting(c)
			if w.Code != http.StatusBadRequest {
				t.Errorf(
					"expected 400, got %d",
					w.Code,
				)
			}
		},
	)

	t.Run(
		"auth failure",
		func(t *testing.T) {
			verifier := &mockVerifier{
				verifyFn: func(initData string) (*TelegramUser, error) {
					return nil, errors.New("auth failed")
				},
			}
			handler := NewHandler(HandlerDeps{Verifier: verifier})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			reqBody := `{"init_data":"bad","session_id":1,"question_id":1,"strokes":[]}`
			c.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(reqBody),
			)
			c.Request.Header.Set(
				"Content-Type",
				"application/json",
			)
			handler.SubmitHandwriting(c)
			if w.Code != http.StatusUnauthorized {
				t.Errorf(
					"expected 401, got %d",
					w.Code,
				)
			}
		},
	)

	t.Run(
		"success",
		func(t *testing.T) {
			handwriting := &mockQuizSession{
				submitHandwritingFn: func(
					ctx context.Context,
					req service.HandwritingSubmitRequest,
				) (*service.HandwritingSubmitResult, error) {
					return &service.HandwritingSubmitResult{IsCorrect: true}, nil
				},
			}
			verifier := &mockVerifier{
				verifyFn: func(initData string) (*TelegramUser, error) {
					return &TelegramUser{ID: 123}, nil
				},
			}
			handler := NewHandler(HandlerDeps{
				Session:  handwriting,
				Verifier: verifier,
			})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			reqBody := `{"init_data":"dummy","session_id":1,"question_id":1,"strokes":[]}`
			c.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(reqBody),
			)
			c.Request.Header.Set(
				"Content-Type",
				"application/json",
			)

			handler.SubmitHandwriting(c)

			if w.Code != http.StatusOK {
				t.Errorf(
					"expected 200, got %d",
					w.Code,
				)
			}
			if !strings.Contains(
				w.Body.String(),
				`"is_correct":true`,
			) {
				t.Errorf(
					"unexpected body: %s",
					w.Body.String(),
				)
			}
		},
	)
}

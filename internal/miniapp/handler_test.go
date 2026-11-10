package miniapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/lsj/copylingo/internal/service"
)

// fakeHandwritingScreen records the graded question the Mini App reports.
type fakeHandwritingScreen struct {
	sessionID   int
	questionID  int
	hasDeadline bool
}

func (fake *fakeHandwritingScreen) ShowHandwritingGraded(
	ctx context.Context,
	sessionID,
	questionID int,
) {
	fake.sessionID = sessionID
	fake.questionID = questionID
	_, fake.hasDeadline = ctx.Deadline()
}

func TestRefreshHandwritingMessageReportsGradedQuestionWithTimeout(t *testing.T) {
	screen := &fakeHandwritingScreen{}
	handler := NewHandler(HandlerDependencies{HandwritingScreen: screen})

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

// oneStrokeSubmitBody is a submit request whose strokes render to a PNG.
const oneStrokeSubmitBody = `{"init_data":"dummy","session_id":1,"question_id":1,` +
	`"strokes":[{"points":[{"x":0,"y":0,"drawing":true},{"x":10,"y":10,"drawing":true}]}]}`

// mockQuizSession fakes service.SessionService's Mini App surface.
type mockQuizSession struct {
	submitHandwritingFunc func(
		ctx context.Context,
		request service.HandwritingSubmitRequest,
	) (*service.HandwritingSubmitResult, error)
}

func (mock *mockQuizSession) SubmitHandwriting(
	ctx context.Context,
	request service.HandwritingSubmitRequest,
) (*service.HandwritingSubmitResult, error) {
	return mock.submitHandwritingFunc(
		ctx,
		request,
	)
}

type mockVerifier struct {
	verifyFunc func(initData string) (*TelegramUser, error)
}

func (mock *mockVerifier) Verify(initData string) (*TelegramUser, error) {
	return mock.verifyFunc(initData)
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
	}

	for _, tt := range tests {
		t.Run(
			tt.name,
			func(t *testing.T) {
				handwriting := &mockQuizSession{
					submitHandwritingFunc: func(
						ctx context.Context,
						request service.HandwritingSubmitRequest,
					) (*service.HandwritingSubmitResult, error) {
						return nil, tt.serviceErr
					},
				}
				verifier := &mockVerifier{
					verifyFunc: func(initData string) (*TelegramUser, error) {
						return &TelegramUser{ID: 123}, nil
					},
				}
				handler := NewHandler(HandlerDependencies{
					Session:           handwriting,
					Verifier:          verifier,
					HandwritingScreen: &fakeHandwritingScreen{},
				})

				recorder := httptest.NewRecorder()
				ginContext, _ := gin.CreateTestContext(recorder)
				requestBody := oneStrokeSubmitBody
				ginContext.Request, _ = http.NewRequest(
					"POST",
					"/api/miniapp/handwriting/submit",
					strings.NewReader(requestBody),
				)
				ginContext.Request.Header.Set(
					"Content-Type",
					"application/json",
				)

				handler.SubmitHandwriting(ginContext)

				if recorder.Code != tt.wantStatus {
					t.Errorf(
						"expected status %d, got %d",
						tt.wantStatus,
						recorder.Code,
					)
				}

				var response map[string]string
				if err := json.Unmarshal(
					recorder.Body.Bytes(),
					&response,
				); err != nil {
					t.Fatalf(
						"failed to unmarshal response: %v",
						err,
					)
				}

				if got := response["error"]; got != tt.wantBody {
					t.Errorf(
						"expected error message %q, got %q",
						tt.wantBody,
						got,
					)
				}

				if tt.notWantInBody != "" && strings.Contains(
					recorder.Body.String(),
					tt.notWantInBody,
				) {
					t.Errorf(
						"response leaked sensitive information: %s",
						recorder.Body.String(),
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
			handler := NewHandler(HandlerDependencies{})
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			ginContext.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(`{invalid`),
			)
			handler.SubmitHandwriting(ginContext)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf(
					"expected 400, got %d",
					recorder.Code,
				)
			}
		},
	)

	t.Run(
		"auth failure",
		func(t *testing.T) {
			verifier := &mockVerifier{
				verifyFunc: func(initData string) (*TelegramUser, error) {
					return nil, errors.New("auth failed")
				},
			}
			handler := NewHandler(HandlerDependencies{Verifier: verifier})
			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			requestBody := `{"init_data":"bad","session_id":1,"question_id":1,"strokes":[]}`
			ginContext.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(requestBody),
			)
			ginContext.Request.Header.Set(
				"Content-Type",
				"application/json",
			)
			handler.SubmitHandwriting(ginContext)
			if recorder.Code != http.StatusUnauthorized {
				t.Errorf(
					"expected 401, got %d",
					recorder.Code,
				)
			}
		},
	)

	t.Run(
		"success",
		func(t *testing.T) {
			handwriting := &mockQuizSession{
				submitHandwritingFunc: func(
					ctx context.Context,
					request service.HandwritingSubmitRequest,
				) (*service.HandwritingSubmitResult, error) {
					if !bytes.HasPrefix(
						request.Image,
						[]byte("\x89PNG"),
					) {
						t.Errorf(
							"service got %d bytes, want a rendered PNG",
							len(request.Image),
						)
					}
					return &service.HandwritingSubmitResult{IsCorrect: true}, nil
				},
			}
			verifier := &mockVerifier{
				verifyFunc: func(initData string) (*TelegramUser, error) {
					return &TelegramUser{ID: 123}, nil
				},
			}
			handler := NewHandler(HandlerDependencies{
				Session:           handwriting,
				Verifier:          verifier,
				HandwritingScreen: &fakeHandwritingScreen{},
			})

			recorder := httptest.NewRecorder()
			ginContext, _ := gin.CreateTestContext(recorder)
			requestBody := oneStrokeSubmitBody
			ginContext.Request, _ = http.NewRequest(
				"POST",
				"/submit",
				strings.NewReader(requestBody),
			)
			ginContext.Request.Header.Set(
				"Content-Type",
				"application/json",
			)

			handler.SubmitHandwriting(ginContext)

			if recorder.Code != http.StatusOK {
				t.Errorf(
					"expected 200, got %d",
					recorder.Code,
				)
			}
			if !strings.Contains(
				recorder.Body.String(),
				`"is_correct":true`,
			) {
				t.Errorf(
					"unexpected body: %s",
					recorder.Body.String(),
				)
			}
		},
	)
}

// Empty strokes cannot be rendered, so the request is rejected before grading.
func TestSubmitHandwriting_EmptyStrokesRejectedBeforeGrading(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(HandlerDependencies{
		Session: &mockQuizSession{
			submitHandwritingFunc: func(
				context.Context,
				service.HandwritingSubmitRequest,
			) (*service.HandwritingSubmitResult, error) {
				t.Fatal("service called with empty strokes")
				return nil, nil
			},
		},
		Verifier: &mockVerifier{
			verifyFunc: func(string) (*TelegramUser, error) {
				return &TelegramUser{ID: 123}, nil
			},
		},
	})

	recorder := httptest.NewRecorder()
	ginContext, _ := gin.CreateTestContext(recorder)
	ginContext.Request, _ = http.NewRequest(
		"POST",
		"/submit",
		strings.NewReader(`{"init_data":"dummy","session_id":1,"question_id":1,"strokes":[]}`),
	)
	ginContext.Request.Header.Set(
		"Content-Type",
		"application/json",
	)

	handler.SubmitHandwriting(ginContext)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf(
			"status = %d, want 400",
			recorder.Code,
		)
	}
}

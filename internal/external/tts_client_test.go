package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGeminiNativeBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://generativelanguage.googleapis.com/v1beta/openai/": "https://generativelanguage.googleapis.com/v1beta",
		"https://generativelanguage.googleapis.com/v1beta/openai":  "https://generativelanguage.googleapis.com/v1beta",
		"https://example.com/v1beta/":                              "https://example.com/v1beta",
		"":                                                         "https://generativelanguage.googleapis.com/v1beta",
	}
	for in, want := range cases {
		if got := geminiNativeBaseURL(in); got != want {
			t.Errorf("geminiNativeBaseURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSynthesize_ConfigMissing(t *testing.T) {
	c := &GeminiTTSClient{apiKey: "", model: ""}
	if _, err := c.Synthesize(context.Background(), "hi"); !errors.Is(err, ErrTTSConfigMissing) {
		t.Fatalf("expected ErrTTSConfigMissing, got %v", err)
	}
}

func TestSynthesize_EmptyText(t *testing.T) {
	c := &GeminiTTSClient{apiKey: "k", model: "m"}
	if _, err := c.Synthesize(context.Background(), "   "); err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestSynthesize_Success(t *testing.T) {
	var gotBody ttsRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf("missing api key header, got %q", r.Header.Get("x-goog-api-key"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request: %v", err)
		}
		pcm := base64.StdEncoding.EncodeToString([]byte("raw-pcm"))
		fmt.Fprintf(
			w,
			`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16;rate=24000","data":%q}}]}}]}`,
			pcm,
		)
	}))
	defer srv.Close()

	c := &GeminiTTSClient{
		httpClient: srv.Client(),
		apiKey:     "test-key",
		baseURL:    srv.URL,
		model:      "tts-model",
		voice:      "Kore",
		transcode: func(_ context.Context, pcm []byte) ([]byte, error) {
			if string(pcm) != "raw-pcm" {
				t.Errorf("transcoder got pcm %q, want raw-pcm", pcm)
			}
			return []byte("ogg-out"), nil
		},
	}

	got, err := c.Synthesize(context.Background(), "こんにちは")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != "ogg-out" {
		t.Errorf("Synthesize = %q, want ogg-out", got)
	}

	// Request body reflects the native TTS contract (ADR-031).
	if len(gotBody.Contents) != 1 || len(gotBody.Contents[0].Parts) != 1 ||
		gotBody.Contents[0].Parts[0].Text != "こんにちは" {
		t.Errorf("request contents = %+v", gotBody.Contents)
	}
	if len(gotBody.GenerationConfig.ResponseModalities) != 1 ||
		gotBody.GenerationConfig.ResponseModalities[0] != "AUDIO" {
		t.Errorf("responseModalities = %v, want [AUDIO]", gotBody.GenerationConfig.ResponseModalities)
	}
	if gotBody.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName != "Kore" {
		t.Errorf(
			"voiceName = %q, want Kore",
			gotBody.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName,
		)
	}
}

func TestSynthesize_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":"rate limited"}`)
	}))
	defer srv.Close()

	c := &GeminiTTSClient{
		httpClient: srv.Client(),
		apiKey:     "k",
		baseURL:    srv.URL,
		model:      "m",
		voice:      "Kore",
		transcode:  func(_ context.Context, pcm []byte) ([]byte, error) { return pcm, nil },
	}
	if _, err := c.Synthesize(context.Background(), "x"); err == nil {
		t.Fatal("expected error on non-200 status")
	}
}

func TestSynthesize_NoAudioInResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"candidates":[{"content":{"parts":[{"text":"oops no audio"}]}}]}`)
	}))
	defer srv.Close()

	c := &GeminiTTSClient{
		httpClient: srv.Client(),
		apiKey:     "k",
		baseURL:    srv.URL,
		model:      "m",
		voice:      "Kore",
		transcode:  func(_ context.Context, pcm []byte) ([]byte, error) { return pcm, nil },
	}
	if _, err := c.Synthesize(context.Background(), "x"); err == nil {
		t.Fatal("expected error when response has no audio data")
	}
}

func TestExtractTTSAudioData(t *testing.T) {
	resp := ttsResponse{}
	if got := extractTTSAudioData(resp); got != "" {
		t.Errorf("empty response should yield empty data, got %q", got)
	}

	resp.Candidates = []struct {
		Content struct {
			Parts []ttsPart `json:"parts"`
		} `json:"content"`
	}{{}}
	resp.Candidates[0].Content.Parts = []ttsPart{
		{Text: "ignored"},
		{InlineData: &ttsInlineData{Data: "AAAA"}},
	}
	if got := extractTTSAudioData(resp); got != "AAAA" {
		t.Errorf("extractTTSAudioData = %q, want AAAA", got)
	}
}

package external

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeminiNativeBaseURL(t *testing.T) {
	cases := map[string]string{
		"https://generativelanguage.googleapis.com/v1beta/openai/": "https://generativelanguage.googleapis.com/v1beta",
		"https://generativelanguage.googleapis.com/v1beta/openai":  "https://generativelanguage.googleapis.com/v1beta",
		"https://example.com/v1beta/":                              "https://example.com/v1beta",
		"":                                                         "https://generativelanguage.googleapis.com/v1beta",
	}
	for input, want := range cases {
		if got := geminiNativeBaseURL(input); got != want {
			t.Errorf(
				"geminiNativeBaseURL(%q) = %q, want %q",
				input,
				got,
				want,
			)
		}
	}
}

func TestSynthesize_ConfigMissing(t *testing.T) {
	client := &GeminiTTSClient{apiKey: "", model: ""}
	if _, err := client.Synthesize(
		context.Background(),
		"hi",
	); !errors.Is(
		err,
		ErrTTSConfigMissing,
	) {
		t.Fatalf(
			"expected ErrTTSConfigMissing, got %v",
			err,
		)
	}
}

func TestSynthesize_EmptyText(t *testing.T) {
	client := &GeminiTTSClient{apiKey: "k", model: "m"}
	if _, err := client.Synthesize(
		context.Background(),
		"   ",
	); err == nil {
		t.Fatal("expected error for empty text")
	}
}

func TestSynthesize_Success(t *testing.T) {
	var gotBody ttsRequest
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) {
		if request.Header.Get("x-goog-api-key") != "test-key" {
			t.Errorf(
				"missing api key header, got %q",
				request.Header.Get("x-goog-api-key"),
			)
		}
		if err := json.NewDecoder(request.Body).Decode(&gotBody); err != nil {
			t.Errorf(
				"decode request: %v",
				err,
			)
		}
		encodedPCM := base64.StdEncoding.EncodeToString([]byte("raw-pcm"))
		fmt.Fprintf(
			responseWriter,
			`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16;rate=24000","data":%q}}]}}]}`,
			encodedPCM,
		)
	}))
	defer server.Close()

	client := &GeminiTTSClient{
		httpClient: server.Client(),
		apiKey:     "test-key",
		baseURL:    server.URL,
		model:      "tts-model",
		voice:      "Kore",
		transcode: func(
			_ context.Context,
			pcmAudio []byte,
		) ([]byte, error) {
			if string(pcmAudio) != "raw-pcm" {
				t.Errorf(
					"transcoder got pcm %q, want raw-pcm",
					pcmAudio,
				)
			}
			return []byte("ogg-out"), nil
		},
	}

	got, err := client.Synthesize(
		context.Background(),
		"こんにちは",
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}
	if string(got) != "ogg-out" {
		t.Errorf(
			"Synthesize = %q, want ogg-out",
			got,
		)
	}

	// Request body reflects the native TTS contract (ADR-031).
	if len(gotBody.Contents) != 1 || len(gotBody.Contents[0].Parts) != 1 ||
		gotBody.Contents[0].Parts[0].Text != "こんにちは" {
		t.Errorf(
			"request contents = %+v",
			gotBody.Contents,
		)
	}
	if len(gotBody.GenerationConfig.ResponseModalities) != 1 ||
		gotBody.GenerationConfig.ResponseModalities[0] != "AUDIO" {
		t.Errorf(
			"responseModalities = %v, want [AUDIO]",
			gotBody.GenerationConfig.ResponseModalities,
		)
	}
	if gotBody.GenerationConfig.SpeechConfig.VoiceConfig == nil ||
		gotBody.GenerationConfig.SpeechConfig.VoiceConfig.PrebuiltVoiceConfig.VoiceName != "Kore" {
		t.Errorf(
			"voiceConfig = %+v, want Kore",
			gotBody.GenerationConfig.SpeechConfig.VoiceConfig,
		)
	}
	if gotBody.GenerationConfig.SpeechConfig.MultiSpeakerVoiceConfig != nil {
		t.Error("single-speaker request includes multi-speaker config")
	}
}

func TestSynthesize_DialogueUsesTwoVoices(t *testing.T) {
	var gotBody ttsRequest
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		request *http.Request,
	) {
		if err := json.NewDecoder(request.Body).Decode(&gotBody); err != nil {
			t.Errorf(
				"decode request: %v",
				err,
			)
		}
		fmt.Fprint(
			responseWriter,
			`{"candidates":[{"content":{"parts":[{"inlineData":{"mimeType":"audio/L16;rate=24000","data":"YQ=="}}]}}]}`,
		)
	}))
	defer server.Close()

	client := &GeminiTTSClient{
		httpClient: server.Client(),
		apiKey:     "test-key",
		baseURL:    server.URL,
		model:      "tts-model",
		voice:      "Kore",
		voiceB:     "Puck",
		transcode: func(
			_ context.Context,
			pcmAudio []byte,
		) ([]byte, error) {
			return pcmAudio, nil
		},
	}
	if _, err := client.Synthesize(
		context.Background(),
		"「どこへ行きますか。」「駅へ行きます。」「わかりました。」",
	); err != nil {
		t.Fatalf(
			"Synthesize dialogue: %v",
			err,
		)
	}

	config := gotBody.GenerationConfig.SpeechConfig
	if config.VoiceConfig != nil || config.MultiSpeakerVoiceConfig == nil {
		t.Fatalf(
			"speech config = %+v, want only multi-speaker config",
			config,
		)
	}
	speakers := config.MultiSpeakerVoiceConfig.SpeakerVoiceConfigs
	if len(speakers) != 2 || speakers[0].Speaker != "A" || speakers[1].Speaker != "B" ||
		speakers[0].VoiceConfig.PrebuiltVoiceConfig.VoiceName != "Kore" ||
		speakers[1].VoiceConfig.PrebuiltVoiceConfig.VoiceName != "Puck" {
		t.Fatalf(
			"speaker voices = %+v, want A=Kore, B=Puck",
			speakers,
		)
	}
	text := gotBody.Contents[0].Parts[0].Text
	for _, turn := range []string{"A: どこへ行きますか。", "B: 駅へ行きます。", "A: わかりました。"} {
		if !strings.Contains(
			text,
			turn,
		) {
			t.Errorf(
				"dialogue prompt %q missing %q",
				text,
				turn,
			)
		}
	}
}

func TestSynthesize_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		_ *http.Request,
	) {
		responseWriter.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(
			responseWriter,
			`{"error":"rate limited"}`,
		)
	}))
	defer server.Close()

	client := &GeminiTTSClient{
		httpClient: server.Client(),
		apiKey:     "k",
		baseURL:    server.URL,
		model:      "m",
		voice:      "Kore",
		transcode: func(
			_ context.Context,
			pcmAudio []byte,
		) ([]byte, error) {
			return pcmAudio, nil
		},
	}
	if _, err := client.Synthesize(
		context.Background(),
		"x",
	); err == nil {
		t.Fatal("expected error on non-200 status")
	}
}

func TestSynthesize_NoAudioInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter,
		_ *http.Request,
	) {
		fmt.Fprint(
			responseWriter,
			`{"candidates":[{"content":{"parts":[{"text":"oops no audio"}]}}]}`,
		)
	}))
	defer server.Close()

	client := &GeminiTTSClient{
		httpClient: server.Client(),
		apiKey:     "k",
		baseURL:    server.URL,
		model:      "m",
		voice:      "Kore",
		transcode: func(
			_ context.Context,
			pcmAudio []byte,
		) ([]byte, error) {
			return pcmAudio, nil
		},
	}
	if _, err := client.Synthesize(
		context.Background(),
		"x",
	); err == nil {
		t.Fatal("expected error when response has no audio data")
	}
}

func TestExtractTTSAudioData(t *testing.T) {
	response := ttsResponse{}
	if got := extractTTSAudioData(response); got != "" {
		t.Errorf(
			"empty response should yield empty data, got %q",
			got,
		)
	}

	response.Candidates = []struct {
		Content struct {
			Parts []ttsPart `json:"parts"`
		} `json:"content"`
	}{{}}
	response.Candidates[0].Content.Parts = []ttsPart{
		{Text: "ignored"},
		{InlineData: &ttsInlineData{Data: "AAAA"}},
	}
	if got := extractTTSAudioData(response); got != "AAAA" {
		t.Errorf(
			"extractTTSAudioData = %q, want AAAA",
			got,
		)
	}
}

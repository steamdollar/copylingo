package external

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/lsj/copylingo/internal/config"
)

// Gemini native TTS returns raw PCM (ADR-031): signed 16-bit little-endian,
// 24kHz, mono. These parameters drive the ffmpeg transcode below.
const (
	ttsPCMSampleRate = 24000
	ttsPCMChannels   = 1
	ttsHTTPTimeout   = 60 * time.Second
)

// TTSClient synthesizes text into a Telegram-ready OGG/Opus voice clip.
type TTSClient interface {
	// Synthesize returns OGG/Opus audio bytes for the given text, ready for
	// Telegram sendVoice.
	Synthesize(ctx context.Context, text string) ([]byte, error)
}

// transcoder converts raw PCM into OGG/Opus. It is a field on the client so tests
// can substitute a fake and avoid the external ffmpeg dependency.
type transcoder func(ctx context.Context, pcm []byte) ([]byte, error)

// GeminiTTSClient calls the Gemini native generateContent AUDIO modality. The
// OpenAI-compatible layer used by the chat LLM does not expose TTS (ADR-031), so
// this is a separate hand-rolled call — the API key is reused, the client is not.
type GeminiTTSClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string // native API base, e.g. https://generativelanguage.googleapis.com/v1beta
	model      string
	voice      string
	transcode  transcoder
}

// NewTTSClient builds a Gemini native TTS client. It reuses the LLM API key and
// derives the native base URL from the LLM (OpenAI-compat) base URL so a single
// endpoint swap moves both.
func NewTTSClient(cfg *config.Config) *GeminiTTSClient {
	return &GeminiTTSClient{
		httpClient: &http.Client{Timeout: ttsHTTPTimeout},
		apiKey:     cfg.LLM.APIKey,
		baseURL:    geminiNativeBaseURL(cfg.LLM.BaseURL),
		model:      cfg.TTS.Model,
		voice:      cfg.TTS.VoiceName,
		transcode:  ffmpegPCMToOGG,
	}
}

// geminiNativeBaseURL turns the OpenAI-compat base URL
// (…/v1beta/openai/) into the native base (…/v1beta) that generateContent needs.
func geminiNativeBaseURL(compatBaseURL string) string {
	u := strings.TrimRight(strings.TrimSpace(compatBaseURL), "/")
	u = strings.TrimSuffix(u, "/openai")
	if u == "" {
		return "https://generativelanguage.googleapis.com/v1beta"
	}
	return u
}

// Synthesize generates audio for text: native TTS -> raw PCM -> OGG/Opus.
func (c *GeminiTTSClient) Synthesize(ctx context.Context, text string) ([]byte, error) {
	if c.apiKey == "" || c.model == "" {
		return nil, ErrTTSConfigMissing
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("tts synthesize: empty text")
	}

	pcm, err := c.generatePCM(ctx, text)
	if err != nil {
		return nil, err
	}

	ogg, err := c.transcode(ctx, pcm)
	if err != nil {
		return nil, fmt.Errorf("tts transcode failed: %w", err)
	}
	return ogg, nil
}

// --- Gemini native generateContent (AUDIO modality) ---

type ttsRequest struct {
	Contents         []ttsContent        `json:"contents"`
	GenerationConfig ttsGenerationConfig `json:"generationConfig"`
}

type ttsContent struct {
	Parts []ttsPart `json:"parts"`
}

type ttsPart struct {
	Text       string         `json:"text,omitempty"`
	InlineData *ttsInlineData `json:"inlineData,omitempty"`
}

type ttsGenerationConfig struct {
	ResponseModalities []string        `json:"responseModalities"`
	SpeechConfig       ttsSpeechConfig `json:"speechConfig"`
}

type ttsSpeechConfig struct {
	VoiceConfig ttsVoiceConfig `json:"voiceConfig"`
}

type ttsVoiceConfig struct {
	PrebuiltVoiceConfig ttsPrebuiltVoiceConfig `json:"prebuiltVoiceConfig"`
}

type ttsPrebuiltVoiceConfig struct {
	VoiceName string `json:"voiceName"`
}

type ttsInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"` // base64-encoded raw PCM
}

type ttsResponse struct {
	Candidates []struct {
		Content struct {
			Parts []ttsPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}

// generatePCM performs the native generateContent call and returns decoded raw PCM.
func (c *GeminiTTSClient) generatePCM(ctx context.Context, text string) ([]byte, error) {
	reqBody := ttsRequest{
		Contents: []ttsContent{{Parts: []ttsPart{{Text: text}}}},
		GenerationConfig: ttsGenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig: ttsSpeechConfig{
				VoiceConfig: ttsVoiceConfig{
					PrebuiltVoiceConfig: ttsPrebuiltVoiceConfig{VoiceName: c.voice},
				},
			},
		},
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tts marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("tts new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tts request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tts read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tts request status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed ttsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("tts parse response: %w", err)
	}

	b64 := extractTTSAudioData(parsed)
	if b64 == "" {
		return nil, fmt.Errorf("tts response contained no audio data")
	}
	pcm, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("tts decode base64 audio: %w", err)
	}
	return pcm, nil
}

// extractTTSAudioData pulls the first inline audio payload out of the response.
func extractTTSAudioData(resp ttsResponse) string {
	for _, cand := range resp.Candidates {
		for _, part := range cand.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				return part.InlineData.Data
			}
		}
	}
	return ""
}

// ffmpegPCMToOGG converts raw signed-16-bit/24kHz/mono PCM into OGG/Opus, the
// container/codec Telegram sendVoice accepts. ffmpeg is an external binary
// dependency (ADR-031: no mature pure-Go Opus encoder).
func ffmpegPCMToOGG(ctx context.Context, pcm []byte) ([]byte, error) {
	if len(pcm) == 0 {
		return nil, fmt.Errorf("ffmpeg: empty pcm input")
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-hide_banner", "-loglevel", "error",
		"-f", "s16le",
		"-ar", strconv.Itoa(ttsPCMSampleRate),
		"-ac", strconv.Itoa(ttsPCMChannels),
		"-i", "pipe:0",
		"-c:a", "libopus",
		"-b:a", "24k",
		"-f", "ogg",
		"pipe:1",
	)

	var out, stderr bytes.Buffer
	cmd.Stdin = bytes.NewReader(pcm)
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}
	return out.Bytes(), nil
}

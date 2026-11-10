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
)

// Gemini native TTS returns raw PCM (ADR-031): signed 16-bit little-endian, 24kHz, mono; these drive the transcode.
const (
	ttsPCMSampleRate = 24000
	ttsPCMChannels   = 1
	ttsHTTPTimeout   = 60 * time.Second
)

// transcoder is a field so tests can fake it and skip the ffmpeg dependency.
type transcoder func(
	ctx context.Context,
	pcmAudio []byte,
) ([]byte, error)

// GeminiTTSClient calls Gemini's native generateContent AUDIO modality; the OpenAI-compat layer has no TTS (ADR-031).
type GeminiTTSClient struct {
	httpClient *http.Client
	apiKey     string
	baseURL    string // native API base, e.g. https://generativelanguage.googleapis.com/v1beta
	model      string
	voice      string
	voiceB     string
	transcode  transcoder
}

// APIKey and BaseURL are the LLM's (OpenAI-compat) values; Voice is single-speaker and speaker A of conversations,
// VoiceB is speaker B.
type TTSOptions struct {
	APIKey  string
	BaseURL string
	Model   string
	Voice   string
	VoiceB  string
}

// NewTTSClient reuses the LLM API key and derives the native base URL from the LLM one, so one swap moves both.
func NewTTSClient(options TTSOptions) *GeminiTTSClient {
	return &GeminiTTSClient{
		httpClient: &http.Client{Timeout: ttsHTTPTimeout},
		apiKey:     options.APIKey,
		baseURL:    geminiNativeBaseURL(options.BaseURL),
		model:      options.Model,
		voice:      options.Voice,
		voiceB:     options.VoiceB,
		transcode:  ffmpegPCMToOGG,
	}
}

// geminiNativeBaseURL turns the OpenAI-compat base (…/v1beta/openai/) into the native one (…/v1beta).
func geminiNativeBaseURL(compatibleBaseURL string) string {
	baseURL := strings.TrimRight(
		strings.TrimSpace(compatibleBaseURL),
		"/",
	)
	baseURL = strings.TrimSuffix(
		baseURL,
		"/openai",
	)
	if baseURL == "" {
		return "https://generativelanguage.googleapis.com/v1beta"
	}
	return baseURL
}

func (client *GeminiTTSClient) Synthesize(
	ctx context.Context,
	text string,
) ([]byte, error) {
	if client.apiKey == "" || client.model == "" {
		return nil, ErrTTSConfigMissing
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("tts synthesize: empty text")
	}

	pcmAudio, err := client.generatePCM(
		ctx,
		text,
	)
	if err != nil {
		return nil, err
	}

	ogg, err := client.transcode(
		ctx,
		pcmAudio,
	)
	if err != nil {
		return nil, fmt.Errorf(
			"tts transcode failed: %w",
			err,
		)
	}
	return ogg, nil
}

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
	VoiceConfig             *ttsVoiceConfig             `json:"voiceConfig,omitempty"`
	MultiSpeakerVoiceConfig *ttsMultiSpeakerVoiceConfig `json:"multiSpeakerVoiceConfig,omitempty"`
}

type ttsMultiSpeakerVoiceConfig struct {
	SpeakerVoiceConfigs []ttsSpeakerVoiceConfig `json:"speakerVoiceConfigs"`
}

type ttsSpeakerVoiceConfig struct {
	Speaker     string         `json:"speaker"`
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

func (client *GeminiTTSClient) generatePCM(
	ctx context.Context,
	text string,
) ([]byte, error) {
	speechConfig := ttsSpeechConfig{
		VoiceConfig: &ttsVoiceConfig{PrebuiltVoiceConfig: ttsPrebuiltVoiceConfig{VoiceName: client.voice}},
	}
	if turns := dialogueTurns(text); len(turns) > 0 {
		if client.voiceB == "" {
			return nil, ErrTTSConfigMissing
		}
		// Speaker labels match the two configured voices and are instructions, not spoken text.
		var dialogue strings.Builder
		dialogue.WriteString("Read this Japanese dialogue. Speak only the lines, not the speaker labels.\n")
		for index, turn := range turns {
			speaker := "A"
			if index%2 == 1 {
				speaker = "B"
			}
			fmt.Fprintf(
				&dialogue,
				"%s: %s\n",
				speaker,
				turn,
			)
		}
		text = dialogue.String()
		speechConfig = ttsSpeechConfig{MultiSpeakerVoiceConfig: &ttsMultiSpeakerVoiceConfig{
			SpeakerVoiceConfigs: []ttsSpeakerVoiceConfig{
				{
					Speaker:     "A",
					VoiceConfig: ttsVoiceConfig{PrebuiltVoiceConfig: ttsPrebuiltVoiceConfig{VoiceName: client.voice}},
				},
				{
					Speaker:     "B",
					VoiceConfig: ttsVoiceConfig{PrebuiltVoiceConfig: ttsPrebuiltVoiceConfig{VoiceName: client.voiceB}},
				},
			},
		}}
	}

	requestBody := ttsRequest{
		Contents: []ttsContent{{Parts: []ttsPart{{Text: text}}}},
		GenerationConfig: ttsGenerationConfig{
			ResponseModalities: []string{"AUDIO"},
			SpeechConfig:       speechConfig,
		},
	}

	payload, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf(
			"tts marshal request: %w",
			err,
		)
	}

	url := fmt.Sprintf(
		"%s/models/%s:generateContent",
		client.baseURL,
		client.model,
	)
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		url,
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"tts new request: %w",
			err,
		)
	}
	request.Header.Set(
		"Content-Type",
		"application/json",
	)
	request.Header.Set(
		"x-goog-api-key",
		client.apiKey,
	)

	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf(
			"tts request failed: %w",
			err,
		)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf(
			"tts read response: %w",
			err,
		)
	}
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"tts request status %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var parsed ttsResponse
	if err := json.Unmarshal(
		body,
		&parsed,
	); err != nil {
		return nil, fmt.Errorf(
			"tts parse response: %w",
			err,
		)
	}

	encoded := extractTTSAudioData(parsed)
	if encoded == "" {
		return nil, fmt.Errorf("tts response contained no audio data")
	}
	pcmAudio, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, fmt.Errorf(
			"tts decode base64 audio: %w",
			err,
		)
	}
	return pcmAudio, nil
}

// Only wholly quoted, alternating turns are treated as a two-person dialogue.
func dialogueTurns(script string) []string {
	script = strings.TrimSpace(script)
	if !strings.HasPrefix(
		script,
		"「",
	) || !strings.HasSuffix(
		script,
		"」",
	) {
		return nil
	}
	turns := strings.Split(
		strings.TrimSuffix(
			strings.TrimPrefix(
				script,
				"「",
			),
			"」",
		),
		"」「",
	)
	if len(turns) < 2 {
		return nil
	}
	for _, turn := range turns {
		if strings.TrimSpace(turn) == "" || strings.ContainsAny(
			turn,
			"「」",
		) {
			return nil
		}
	}
	return turns
}

func extractTTSAudioData(response ttsResponse) string {
	for _, candidate := range response.Candidates {
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				return part.InlineData.Data
			}
		}
	}
	return ""
}

// ffmpegPCMToOGG transcodes PCM to OGG/Opus, which sendVoice accepts; no mature pure-Go Opus encoder (ADR-031).
func ffmpegPCMToOGG(
	ctx context.Context,
	pcmAudio []byte,
) ([]byte, error) {
	if len(pcmAudio) == 0 {
		return nil, fmt.Errorf("ffmpeg: empty pcm input")
	}

	command := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel",
		"error",
		"-f",
		"s16le",
		"-ar",
		strconv.Itoa(ttsPCMSampleRate),
		"-ac",
		strconv.Itoa(ttsPCMChannels),
		"-i",
		"pipe:0",
		"-c:a",
		"libopus",
		"-b:a",
		"24k",
		"-f",
		"ogg",
		"pipe:1",
	)

	var out, stderr bytes.Buffer
	command.Stdin = bytes.NewReader(pcmAudio)
	command.Stdout = &out
	command.Stderr = &stderr

	if err := command.Run(); err != nil {
		return nil, fmt.Errorf(
			"ffmpeg: %w (%s)",
			err,
			strings.TrimSpace(stderr.String()),
		)
	}
	return out.Bytes(), nil
}

package external

import "errors"

// ErrAIConfigMissing indicates that the AI/LLM client setup is missing or disabled.
var ErrAIConfigMissing = errors.New("ai system is not configured")

// ErrTTSConfigMissing indicates that the TTS client setup is missing or disabled.
var ErrTTSConfigMissing = errors.New("tts system is not configured")

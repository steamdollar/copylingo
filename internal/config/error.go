package config

import "errors"

// ErrAIConfigMissing indicates that the AI/LLM client setup is missing or disabled.
var ErrAIConfigMissing = errors.New("ai system is not configured")

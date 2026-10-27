package external

import "errors"

// ErrTTSConfigMissing indicates that the TTS client lacks required settings.
var ErrTTSConfigMissing = errors.New("tts system is not configured")

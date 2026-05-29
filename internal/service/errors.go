package service

import "errors"

// ErrAIUnavailable indicates that AI-backed grading is currently unavailable.
var ErrAIUnavailable = errors.New("ai grading is unavailable")

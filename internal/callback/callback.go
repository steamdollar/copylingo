package callback

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"strconv"
	"strings"

	"github.com/lsj/copylingo/internal/config"
)

// ParseHandwritingMessageRef parses a string in "chat_id:message_id" format.
func ParseHandwritingMessageRef(raw string) (int64, int, error) {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("expected chat_id:message_id")
	}

	chatID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("parse chat_id: %w", err)
	}
	if chatID == 0 {
		return 0, 0, fmt.Errorf("chat_id is zero")
	}

	msgID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("parse message_id: %w", err)
	}
	if msgID <= 0 {
		return 0, 0, fmt.Errorf("message_id must be positive")
	}

	return chatID, msgID, nil
}

// FormatHandwritingNext generates callback data for the "Next Question" button in handwriting tasks.
func FormatHandwritingNext(sessionID, questionIdx int, publicBaseURL string) string {
	token := MiniAppURLFingerprint(publicBaseURL)
	if token == "" {
		return fmt.Sprintf(config.FormatQuestionNext, sessionID, questionIdx)
	}
	return fmt.Sprintf("%s:u:%s", fmt.Sprintf(config.FormatQuestionNext, sessionID, questionIdx), token)
}

// MiniAppURLFingerprint generates a 8-char hex fingerprint of the host part of a URL.
// This is used to detect if a callback was generated for a different server instance (e.g. tunnel restart).
func MiniAppURLFingerprint(rawURL string) string {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Host == "" {
		return ""
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(u.Host)))
	return fmt.Sprintf("%08x", h.Sum32())
}

// IsStaleMiniAppCallback checks if the callback data was generated for the current server instance.
func IsStaleMiniAppCallback(parts []string, currentPublicBaseURL string) bool {
	messageToken, ok := MiniAppFingerprintFromCallbackParts(parts)
	if !ok {
		// Older messages did not carry a URL fingerprint, so regenerate once.
		return true
	}
	currentToken := MiniAppURLFingerprint(currentPublicBaseURL)
	return currentToken == "" || messageToken != currentToken
}

// MiniAppFingerprintFromCallbackParts extracts the URL fingerprint from parsed callback data.
func MiniAppFingerprintFromCallbackParts(parts []string) (string, bool) {
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "u" && parts[i+1] != "" {
			return parts[i+1], true
		}
	}
	return "", false
}

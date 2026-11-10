package callback

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"
)

// Question callbacks are also created by the Mini App and handwriting flow.
const (
	QuestionRoot         = "q"
	QuestionPrefix       = QuestionRoot + ":"
	QuestionActionNext   = "next"
	QuestionActionPolicy = "policy"
	FormatQuestionNext   = QuestionPrefix + "%d:" + QuestionActionNext + ":%d"
	FormatQuestionPolicy = QuestionPrefix + "%d:" + QuestionActionPolicy + ":%d"
)

// FormatHandwritingNext generates callback data for the "Next Question" button in handwriting tasks.
func FormatHandwritingNext(
	sessionID,
	questionIndex int,
	publicBaseURL string,
) string {
	token := MiniAppURLFingerprint(publicBaseURL)
	if token == "" {
		return fmt.Sprintf(
			FormatQuestionNext,
			sessionID,
			questionIndex,
		)
	}
	return fmt.Sprintf(
		"%s:u:%s",
		fmt.Sprintf(
			FormatQuestionNext,
			sessionID,
			questionIndex,
		),
		token,
	)
}

// MiniAppURLFingerprint generates a 8-char hex fingerprint of the host part of a URL.
// This is used to detect if a callback was generated for a different server instance (e.g. tunnel restart).
func MiniAppURLFingerprint(rawURL string) string {
	parsedURL, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsedURL.Host == "" {
		return ""
	}
	hasher := fnv.New32a()
	_, _ = hasher.Write([]byte(strings.ToLower(parsedURL.Host)))
	return fmt.Sprintf(
		"%08x",
		hasher.Sum32(),
	)
}

// IsStaleMiniAppCallback checks if the callback data was generated for the current server instance.
func IsStaleMiniAppCallback(
	parts []string,
	currentPublicBaseURL string,
) bool {
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

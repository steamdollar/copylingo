package callback

import (
	"fmt"
	"hash/fnv"
	"net/url"
	"strings"
)

const (
	QuestionRoot         = "q"
	QuestionPrefix       = QuestionRoot + ":"
	QuestionActionNext   = "next"
	QuestionActionPolicy = "policy"
	FormatQuestionNext   = QuestionPrefix + "%d:" + QuestionActionNext + ":%d"
	FormatQuestionPolicy = QuestionPrefix + "%d:" + QuestionActionPolicy + ":%d"
)

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

// MiniAppURLFingerprint is 8 hex chars of the URL host, to detect callbacks from another server instance.
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

func MiniAppFingerprintFromCallbackParts(parts []string) (string, bool) {
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "u" && parts[i+1] != "" {
			return parts[i+1], true
		}
	}
	return "", false
}

package bot

import (
	"fmt"
	"strconv"
	"strings"
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

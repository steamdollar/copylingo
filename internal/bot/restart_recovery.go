package bot

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/config"
	"github.com/lsj/copylingo/internal/model"
)

// RefreshStaleMiniAppMessages is called once at server startup to check in-progress sessions.
// If the next unanswered question is a handwriting task and the Mini App URL has changed,
// it re-sends the question with a fresh URL.
func (b *Bot) RefreshStaleMiniAppMessages(ctx context.Context) {
	baseURL := b.cfg.Server.PublicBaseURL
	if baseURL == "" {
		log.Printf("[restart-recovery] PublicBaseURL empty; skipping")
		return
	}
	currentFp := callback.MiniAppURLFingerprint(baseURL)

	sessions, err := b.services.SessionBuilder.GetAllInProgressSessions(ctx)
	if err != nil {
		log.Printf("[restart-recovery] list in_progress sessions: %v", err)
		return
	}

	for _, s := range sessions {
		// Skip if fingerprint unchanged
		key := fmt.Sprintf("copylingo:miniapp:last_fingerprint:%d", s.ID)
		if last, _ := b.rdb.Get(ctx, key).Result(); last == currentFp {
			continue
		}

		state, err := b.services.ActiveSession.Get(ctx, s.ID)
		if err != nil {
			log.Printf("[restart-recovery] active session state unavailable session=%d: %v", s.ID, err)
			continue
		}

		idx := state.NextUnansweredIndex()
		if idx >= len(state.Items) {
			continue
		}

		q := state.Items[idx].Question
		if q.Type != model.QuestionKanaHandwriting {
			continue
		}

		// (a) best-effort: edit old message to strip buttons via HandwritingMessageRedisKey
		oldKey := config.HandwritingMessageRedisKey.Format(s.ID, q.ID)
		if val, err := b.rdb.Get(ctx, oldKey).Result(); err == nil {
			if chatID, msgID, perr := ParseHandwritingMessageRef(val); perr == nil {
				_ = b.ClearInlineKeyboard(chatID, msgID)
			}
		}

		// (b) re-send the question with fresh URL
		log.Printf("[restart-recovery] refreshing stale link for session=%d user=%d", s.ID, s.UserID)
		b.SendMessage(s.UserID, "🔄 손글씨 링크가 갱신되었습니다. 아래 버튼으로 다시 진행해 주세요.")
		b.flow.showQuestion(ctx, s.UserID, nil, s.ID, idx)

		_ = b.rdb.Set(ctx, key, currentFp, 24*time.Hour).Err()
	}
}

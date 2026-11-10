package bot

import (
	"context"
	"log/slog"

	"github.com/lsj/copylingo/internal/callback"
	"github.com/lsj/copylingo/internal/model"
	"github.com/lsj/copylingo/internal/observability"
)

// Runs once at startup: re-sends a pending handwriting question when the Mini App URL changed.
func (flow *SessionFlow) RefreshStaleMiniAppMessages(ctx context.Context) {
	ctx = observability.WithAttributes(
		ctx,
		slog.String(
			"interaction_id",
			observability.NewInteractionID("job-restart_recovery"),
		),
		slog.String(
			"source",
			"telegram.restart_recovery",
		),
	)
	baseURL := flow.publicBaseURL
	if baseURL == "" {
		slog.InfoContext(
			ctx,
			"Restart recovery skipped because public base URL is empty",
			"event",
			"telegram.restart_recovery.skipped",
		)
		return
	}
	currentFingerprint := callback.MiniAppURLFingerprint(baseURL)

	sessions, err := flow.session.ListInProgressQuizzes(ctx)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to list in-progress sessions for restart recovery",
			"event",
			"telegram.restart_recovery.session_list_failed",
			"error",
			err,
		)
		return
	}

	for _, session := range sessions {
		// Shutdown cancels ctx; stop instead of failing every remaining session.
		if ctx.Err() != nil {
			return
		}
		if last, _ := flow.recovery.GetMiniAppFingerprint(
			ctx,
			session.ID,
		); last == currentFingerprint {
			continue
		}

		state, err := flow.session.QuizProgress(
			ctx,
			session.ID,
		)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Active session state unavailable during restart recovery",
				"event",
				"telegram.restart_recovery.active_state_unavailable",
				"session_id",
				session.ID,
				"error",
				err,
			)
			continue
		}

		index := state.NextUnansweredIndex()
		if index >= len(state.Items) {
			continue
		}

		question := state.Items[index].Question
		if question.Type != model.QuestionKanaHandwriting {
			continue
		}

		// Best-effort: edit the old message to strip its stale buttons.
		if messageReference, err := flow.messages.GetHandwritingMessage(
			ctx,
			session.ID,
			question.ID,
		); err == nil && messageReference != nil {
			_ = flow.telegram.ClearInlineKeyboard(
				messageReference.ChatID,
				messageReference.MessageID,
			)
		}

		slog.InfoContext(
			ctx,
			"Refreshing stale handwriting link",
			"event",
			"telegram.restart_recovery.link_refreshing",
			"session_id",
			session.ID,
			"user_id",
			session.UserID,
		)
		flow.telegram.SendMessage(
			session.UserID,
			botMessagesByLocale[botDefaultLocale].handwritingLinkUpdated,
		)
		flow.showQuestion(
			ctx,
			session.UserID,
			nil,
			session.ID,
			index,
		)

		_ = flow.recovery.SetMiniAppFingerprint(
			ctx,
			session.ID,
			currentFingerprint,
		)
	}
}

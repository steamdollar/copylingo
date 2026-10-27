package service

import (
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// srs: Spaced Repetition System. Only the SM-2 scheduling lives here; due
// reviews are read straight from QuestionRepo with the session level scope.

// scheduleAnswer applies one answer to progress in memory. The Quiz flush
// writes it to the DB when the session completes.
func scheduleAnswer(
	progress *model.UserQuestionProgress,
	isCorrect bool,
) {
	quality := 1 // wrong
	if isCorrect {
		quality = 4 // correct with some hesitation
	}
	updateSchedule(
		progress,
		quality,
	)
}

// updateSchedule applies the SM-2 algorithm to update the user's Question progress.
func updateSchedule(
	q *model.UserQuestionProgress,
	quality int,
) {
	now := time.Now()
	q.LastReviewedAt = &now

	if quality >= 3 { // Correct answer
		switch q.Repetitions {
		case 0:
			// 첫 복습 간격: 1일이면 하루 2회 push 환경에서 학습한 항목이
			// 다음날 곧장 due로 재등장한다. 3일로 늘려 매일 재출제를 끊는다.
			q.IntervalDays = 3
		case 1:
			q.IntervalDays = 6
		default:
			q.IntervalDays = int(float64(q.IntervalDays) * q.EaseFactor)
		}
		q.Repetitions++
	} else { // Wrong answer — reset
		q.Repetitions = 0
		q.IntervalDays = 1
	}

	// Update ease factor
	ef := q.EaseFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if ef < 1.3 {
		ef = 1.3
	}
	q.EaseFactor = ef

	nextReview := now.AddDate(
		0,
		0,
		q.IntervalDays,
	)
	q.NextReviewAt = &nextReview
}

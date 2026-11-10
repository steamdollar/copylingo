package service

import (
	"time"

	"github.com/lsj/copylingo/internal/model"
)

// Only SM-2 scheduling lives here; due reviews are read straight from QuestionRepository.

// scheduleAnswer applies one answer in memory; the Quiz flush writes it to the DB on completion.
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

func updateSchedule(
	progress *model.UserQuestionProgress,
	quality int,
) {
	now := time.Now()
	progress.LastReviewedAt = &now

	if quality >= 3 { // Correct answer
		switch progress.Repetitions {
		case 0:
			// 첫 복습 간격: 1일이면 하루 2회 push 환경에서 학습한 항목이
			// 다음날 곧장 due로 재등장한다. 3일로 늘려 매일 재출제를 끊는다.
			progress.IntervalDays = 3
		case 1:
			progress.IntervalDays = 6
		default:
			progress.IntervalDays = int(float64(progress.IntervalDays) * progress.EaseFactor)
		}
		progress.Repetitions++
	} else { // Wrong answer — reset
		progress.Repetitions = 0
		progress.IntervalDays = 1
	}

	easeFactor := progress.EaseFactor + (0.1 - float64(5-quality)*(0.08+float64(5-quality)*0.02))
	if easeFactor < 1.3 {
		easeFactor = 1.3
	}
	progress.EaseFactor = easeFactor

	nextReview := now.AddDate(
		0,
		0,
		progress.IntervalDays,
	)
	progress.NextReviewAt = &nextReview
}

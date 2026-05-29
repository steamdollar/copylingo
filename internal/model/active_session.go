package model

import "time"

const ActiveSessionStateVersion = 1

// ActiveSessionState is the Redis working state for an in-progress session.
type ActiveSessionState struct {
	Version       int                     `json:"version"`
	Session       Session                 `json:"session"`
	Items         []ActiveSessionQuestion `json:"items"`
	CurrentIndex  int                     `json:"current_index"`
	AnsweredCount int                     `json:"answered_count"`
	UpdatedAt     time.Time               `json:"updated_at"`
}

// ActiveSessionQuestion keeps the ordered session question and its question copy together.
type ActiveSessionQuestion struct {
	SessionQuestion SessionQuestion `json:"session_question"`
	Question        Question        `json:"question"`
}

func (s *ActiveSessionState) RecountAnswered() int {
	count := 0
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil {
			count++
		}
	}
	s.AnsweredCount = count
	return count
}

func (s *ActiveSessionState) FindItemByQuestionID(questionID int) (*ActiveSessionQuestion, int, bool) {
	for i := range s.Items {
		if s.Items[i].Question.ID == questionID {
			return &s.Items[i], i, true
		}
	}
	return nil, -1, false
}

func (s *ActiveSessionState) ItemAt(idx int) (*ActiveSessionQuestion, bool) {
	if idx < 0 || idx >= len(s.Items) {
		return nil, false
	}
	return &s.Items[idx], true
}

func (s *ActiveSessionState) NextUnansweredIndex() int {
	for idx, item := range s.Items {
		if item.SessionQuestion.IsCorrect == nil {
			return idx
		}
	}
	return len(s.Items)
}

func (s *ActiveSessionState) CorrectCount() int {
	count := 0
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil && *item.SessionQuestion.IsCorrect {
			count++
		}
	}
	return count
}

func (s *ActiveSessionState) WrongAnswers() []ActiveSessionQuestion {
	wrong := make([]ActiveSessionQuestion, 0)
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil && !*item.SessionQuestion.IsCorrect {
			wrong = append(wrong, item)
		}
	}
	return wrong
}

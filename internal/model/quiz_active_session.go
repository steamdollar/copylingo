package model

import "time"

const QuizActiveSessionStateVersion = 2

// QuizActiveSessionState is the Redis working state for an in-progress quiz session.
type QuizActiveSessionState struct {
	Version int `json:"version"`
	// Session is copied from the sessions table and flushed back on complete.
	Session Session `json:"session"`
	// Items are copied from session_questions + questions and mutated in Redis
	// while the user answers.
	Items         []QuizActiveSessionQuestion `json:"items"`
	CurrentIndex  int                         `json:"current_index"`
	AnsweredCount int                         `json:"answered_count"`
	UpdatedAt     time.Time                   `json:"updated_at"`
}

// QuizActiveSessionQuestion keeps the ordered session question and its question copy together.
type QuizActiveSessionQuestion struct {
	SessionQuestion SessionQuestion      `json:"session_question"`
	Question        Question             `json:"question"`
	Progress        UserQuestionProgress `json:"progress"`
}

func (s *QuizActiveSessionState) RecountAnswered() int {
	count := 0
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil {
			count++
		}
	}
	s.AnsweredCount = count
	return count
}

func (s *QuizActiveSessionState) CurrentItemByQuestionID(questionID int) (*QuizActiveSessionQuestion, int, bool) {
	item, ok := s.ItemAt(s.CurrentIndex)
	if !ok || item.SessionQuestion.QuestionID != questionID {
		return nil, -1, false
	}
	return item, s.CurrentIndex, true
}

// ItemByQuestionID finds the session item for a question regardless of CurrentIndex.
// Used after answering when the current index may have advanced (e.g. in-quiz LLM "ask" flow).
func (s *QuizActiveSessionState) ItemByQuestionID(questionID int) (*QuizActiveSessionQuestion, bool) {
	for i := range s.Items {
		if s.Items[i].SessionQuestion.QuestionID == questionID {
			return &s.Items[i], true
		}
	}
	return nil, false
}

func (s *QuizActiveSessionState) ItemAt(idx int) (*QuizActiveSessionQuestion, bool) {
	if idx < 0 || idx >= len(s.Items) {
		return nil, false
	}
	return &s.Items[idx], true
}

func (s *QuizActiveSessionState) NextUnansweredIndex() int {
	for idx, item := range s.Items {
		if item.SessionQuestion.IsCorrect == nil {
			return idx
		}
	}
	return len(s.Items)
}

func (s *QuizActiveSessionState) CorrectCount() int {
	count := 0
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil && *item.SessionQuestion.IsCorrect {
			count++
		}
	}
	return count
}

func (s *QuizActiveSessionState) WrongAnswers() []QuizActiveSessionQuestion {
	wrong := make([]QuizActiveSessionQuestion, 0)
	for _, item := range s.Items {
		if item.SessionQuestion.IsCorrect != nil && !*item.SessionQuestion.IsCorrect {
			wrong = append(wrong, item)
		}
	}
	return wrong
}

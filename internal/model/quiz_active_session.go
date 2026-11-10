package model

import "time"

const QuizActiveSessionStateVersion = 2

type QuizActiveSessionState struct {
	Version int `json:"version"`
	// Session is copied from the sessions table and flushed back on complete.
	Session Session `json:"session"`
	// Items are copied from session_questions + questions and mutated in Redis while the user answers.
	Items         []QuizActiveSessionQuestion `json:"items"`
	CurrentIndex  int                         `json:"current_index"`
	AnsweredCount int                         `json:"answered_count"`
	UpdatedAt     time.Time                   `json:"updated_at"`
}

type QuizActiveSessionQuestion struct {
	SessionQuestion SessionQuestion      `json:"session_question"`
	Question        Question             `json:"question"`
	Progress        UserQuestionProgress `json:"progress"`
}

func (state *QuizActiveSessionState) RecountAnswered() int {
	count := 0
	for _, item := range state.Items {
		if item.SessionQuestion.IsCorrect != nil {
			count++
		}
	}
	state.AnsweredCount = count
	return count
}

func (state *QuizActiveSessionState) CurrentItemByQuestionID(questionID int) (*QuizActiveSessionQuestion, int, bool) {
	item, ok := state.ItemAt(state.CurrentIndex)
	if !ok || item.SessionQuestion.QuestionID != questionID {
		return nil, -1, false
	}
	return item, state.CurrentIndex, true
}

// ItemByQuestionID finds an item regardless of CurrentIndex, which may have advanced (e.g. in-quiz LLM "ask" flow).
func (state *QuizActiveSessionState) ItemByQuestionID(questionID int) (*QuizActiveSessionQuestion, bool) {
	for i := range state.Items {
		if state.Items[i].SessionQuestion.QuestionID == questionID {
			return &state.Items[i], true
		}
	}
	return nil, false
}

func (state *QuizActiveSessionState) ItemAt(index int) (*QuizActiveSessionQuestion, bool) {
	if index < 0 || index >= len(state.Items) {
		return nil, false
	}
	return &state.Items[index], true
}

func (state *QuizActiveSessionState) NextUnansweredIndex() int {
	for index, item := range state.Items {
		if item.SessionQuestion.IsCorrect == nil {
			return index
		}
	}
	return len(state.Items)
}

func (state *QuizActiveSessionState) CorrectCount() int {
	count := 0
	for _, item := range state.Items {
		if item.SessionQuestion.IsCorrect != nil && *item.SessionQuestion.IsCorrect {
			count++
		}
	}
	return count
}

func (state *QuizActiveSessionState) WrongAnswers() []QuizActiveSessionQuestion {
	wrong := make(
		[]QuizActiveSessionQuestion,
		0,
	)
	for _, item := range state.Items {
		if item.SessionQuestion.IsCorrect != nil && !*item.SessionQuestion.IsCorrect {
			wrong = append(
				wrong,
				item,
			)
		}
	}
	return wrong
}

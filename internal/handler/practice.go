package handler

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
)

func (h *Handler) PracticeStart(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	count, err := h.Questions.CountQuestions(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	attempt, err := h.Attempts.CreateAttempt(subjectID, "flashcard", count)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/practice/%d/question?attempt=%d", subjectID, attempt.ID), http.StatusSeeOther)
}

func (h *Handler) PracticeQuestion(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	attemptID, err := strconv.ParseInt(r.URL.Query().Get("attempt"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid attempt ID", http.StatusBadRequest)
		return
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get already answered question IDs
	answeredIDs, err := h.Attempts.GetAnsweredQuestionIDs(attemptID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	question, err := h.Questions.GetRandomQuestion(subjectID, answeredIDs)
	if err == sql.ErrNoRows {
		// All questions answered, finish attempt and show result
		h.Attempts.FinishAttempt(attemptID)
		http.Redirect(w, r, fmt.Sprintf("/exam/%d/result", attemptID), http.StatusSeeOther)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "practice.html", map[string]any{
		"Subject":   subject,
		"Question":  question,
		"AttemptID": attemptID,
		"Progress":  len(answeredIDs) + 1,
		"Total":     subject.QuestionCount,
	})
}

func (h *Handler) PracticeAnswer(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	attemptID, _ := strconv.ParseInt(r.FormValue("attempt_id"), 10, 64)
	questionID, _ := strconv.ParseInt(r.FormValue("question_id"), 10, 64)

	// Get question to check if multi-answer
	question, err := h.Questions.GetQuestionWithAnswers(questionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Collect selected answer IDs (single or multiple)
	var selectedAnswerIDs []int64
	if question.MultiAnswer {
		for _, v := range r.Form["answer_id"] {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				selectedAnswerIDs = append(selectedAnswerIDs, id)
			}
		}
	} else {
		if id, err := strconv.ParseInt(r.FormValue("answer_id"), 10, 64); err == nil {
			selectedAnswerIDs = []int64{id}
		}
	}

	// Record the answers
	err = h.Attempts.RecordAnswers(attemptID, questionID, selectedAnswerIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Build selected/correct answer labels for display
	var selectedLabels, correctLabels []string
	selectedSet := make(map[int64]bool)
	for _, id := range selectedAnswerIDs {
		selectedSet[id] = true
	}
	for _, a := range question.Answers {
		if selectedSet[a.ID] {
			selectedLabels = append(selectedLabels, a.Label+". "+a.Content)
		}
		if a.IsCorrect {
			correctLabels = append(correctLabels, a.Label+". "+a.Content)
		}
	}

	// Check correctness
	isCorrect := len(selectedSet) == len(correctLabels)
	if isCorrect {
		for _, a := range question.Answers {
			if a.IsCorrect && !selectedSet[a.ID] {
				isCorrect = false
				break
			}
		}
	}

	subject, _ := h.Questions.GetSubject(subjectID)
	answeredIDs, _ := h.Attempts.GetAnsweredQuestionIDs(attemptID)

	// If all questions answered, finish the attempt to calculate score
	isLastQuestion := len(answeredIDs) >= subject.QuestionCount
	if isLastQuestion {
		h.Attempts.FinishAttempt(attemptID)
	}

	h.render(w, "practice_result.html", map[string]any{
		"Subject":        subject,
		"Question":       question,
		"SelectedLabels": selectedLabels,
		"CorrectLabels":  correctLabels,
		"IsCorrect":      isCorrect,
		"AttemptID":      attemptID,
		"SubjectID":      subjectID,
		"Progress":       len(answeredIDs),
		"Total":          subject.QuestionCount,
	})
}

package handler

import (
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

// PracticeSetup is the flashcard entry point. With a single chapter it starts
// immediately (preserving the one-click flow); with multiple chapters it shows
// a page to pick which chapters to study.
func (h *Handler) PracticeSetup(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("PracticeSetup: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		slog.Error("PracticeSetup: get subject failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Không tìm thấy chủ đề", http.StatusNotFound)
		return
	}

	chapters, err := h.Questions.ListChapters(subjectID)
	if err != nil {
		slog.Error("PracticeSetup: list chapters failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	if len(chapters) <= 1 {
		h.startPractice(w, r, subjectID, nil)
		return
	}

	h.render(w, "practice_setup.html", map[string]any{
		"Subject":  subject,
		"Chapters": chapters,
	})
}

// PracticeStart creates a flashcard attempt scoped to the selected chapters.
func (h *Handler) PracticeStart(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("PracticeStart: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}
	r.ParseForm()
	h.startPractice(w, r, subjectID, parseChapterIDs(r))
}

func (h *Handler) startPractice(w http.ResponseWriter, r *http.Request, subjectID int64, chapterIDs []int64) {
	count, err := h.Questions.CountQuestionsInChapters(subjectID, chapterIDs)
	if err != nil {
		slog.Error("startPractice: count questions failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}
	if count == 0 {
		slog.Warn("startPractice: no questions in selected chapters", "subjectID", subjectID)
		http.Error(w, "Không có câu hỏi nào trong các chương đã chọn", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.CreateAttempt(subjectID, "flashcard", count)
	if err != nil {
		slog.Error("startPractice: create attempt failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	if err := h.Attempts.SetAttemptChapters(attempt.ID, chapterIDs); err != nil {
		slog.Error("startPractice: set attempt chapters failed", "attemptID", attempt.ID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.url(fmt.Sprintf("/practice/%d/question?attempt=%d", subjectID, attempt.ID)), http.StatusSeeOther)
}

func (h *Handler) PracticeQuestion(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("PracticeQuestion: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}

	attemptID, err := strconv.ParseInt(r.URL.Query().Get("attempt"), 10, 64)
	if err != nil {
		slog.Warn("PracticeQuestion: invalid attempt ID", "subjectID", subjectID, "raw", r.URL.Query().Get("attempt"))
		http.Error(w, "Attempt ID không hợp lệ", http.StatusBadRequest)
		return
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		slog.Error("PracticeQuestion: get subject failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Không tìm thấy chủ đề", http.StatusNotFound)
		return
	}

	chapterIDs, err := h.Attempts.GetAttemptChapters(attemptID)
	if err != nil {
		slog.Error("PracticeQuestion: get attempt chapters failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	answeredIDs, err := h.Attempts.GetAnsweredQuestionIDs(attemptID)
	if err != nil {
		slog.Error("PracticeQuestion: get answered IDs failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	question, err := h.Questions.GetRandomQuestionInChapters(subjectID, chapterIDs, answeredIDs)
	if err == sql.ErrNoRows {
		if _, err := h.Attempts.FinishAttempt(attemptID); err != nil {
			slog.Error("PracticeQuestion: finish attempt failed", "attemptID", attemptID, "error", err)
		}
		http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/result", attemptID)), http.StatusSeeOther)
		return
	}
	if err != nil {
		slog.Error("PracticeQuestion: get random question failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	total, err := h.Questions.CountQuestionsInChapters(subjectID, chapterIDs)
	if err != nil {
		slog.Error("PracticeQuestion: count questions failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	h.render(w, "practice.html", map[string]any{
		"Subject":   subject,
		"Question":  question,
		"AttemptID": attemptID,
		"Progress":  len(answeredIDs) + 1,
		"Total":     total,
	})
}

func (h *Handler) PracticeAnswer(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("PracticeAnswer: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Warn("PracticeAnswer: parse form failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Dữ liệu form không hợp lệ", http.StatusBadRequest)
		return
	}

	attemptID, err := strconv.ParseInt(r.FormValue("attempt_id"), 10, 64)
	if err != nil {
		slog.Warn("PracticeAnswer: invalid attempt ID", "subjectID", subjectID, "raw", r.FormValue("attempt_id"))
		http.Error(w, "Attempt ID không hợp lệ", http.StatusBadRequest)
		return
	}

	questionID, err := strconv.ParseInt(r.FormValue("question_id"), 10, 64)
	if err != nil {
		slog.Warn("PracticeAnswer: invalid question ID", "subjectID", subjectID, "raw", r.FormValue("question_id"))
		http.Error(w, "Question ID không hợp lệ", http.StatusBadRequest)
		return
	}

	question, err := h.Questions.GetQuestionWithAnswers(questionID)
	if err != nil {
		slog.Error("PracticeAnswer: get question failed", "questionID", questionID, "error", err)
		http.Error(w, "Không tìm thấy câu hỏi", http.StatusNotFound)
		return
	}

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

	if err := h.Attempts.RecordAnswers(attemptID, questionID, selectedAnswerIDs); err != nil {
		slog.Error("PracticeAnswer: record answers failed", "attemptID", attemptID, "questionID", questionID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

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

	isCorrect := len(selectedSet) == len(correctLabels)
	if isCorrect {
		for _, a := range question.Answers {
			if a.IsCorrect && !selectedSet[a.ID] {
				isCorrect = false
				break
			}
		}
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		slog.Error("PracticeAnswer: get subject failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	chapterIDs, err := h.Attempts.GetAttemptChapters(attemptID)
	if err != nil {
		slog.Error("PracticeAnswer: get attempt chapters failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	total, err := h.Questions.CountQuestionsInChapters(subjectID, chapterIDs)
	if err != nil {
		slog.Error("PracticeAnswer: count questions failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	answeredIDs, err := h.Attempts.GetAnsweredQuestionIDs(attemptID)
	if err != nil {
		slog.Error("PracticeAnswer: get answered IDs failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	if len(answeredIDs) >= total {
		finished, err := h.Attempts.FinishAttempt(attemptID)
		if err != nil {
			slog.Error("PracticeAnswer: finish attempt failed", "attemptID", attemptID, "error", err)
		} else {
			slog.Info("flashcard session finished", "attemptID", attemptID, "subjectID", subjectID, "score", fmt.Sprintf("%.1f%%", finished.Score), "correct", finished.CorrectCount, "total", finished.TotalQuestions)
		}
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
		"Total":          total,
	})
}

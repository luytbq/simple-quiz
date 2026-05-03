package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

func (h *Handler) ExamSetup(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("ExamSetup: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}

	subject, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		slog.Error("ExamSetup: get subject failed", "subjectID", subjectID, "error", err)
		http.Error(w, "Không tìm thấy chủ đề", http.StatusNotFound)
		return
	}

	h.render(w, "exam_setup.html", map[string]any{
		"Subject": subject,
	})
}

func (h *Handler) ExamStart(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		slog.Warn("ExamStart: invalid subject ID", "raw", r.PathValue("subjectID"))
		http.Error(w, "Subject ID không hợp lệ", http.StatusBadRequest)
		return
	}

	r.ParseForm()
	count, err := strconv.Atoi(r.FormValue("question_count"))
	if err != nil || count <= 0 {
		slog.Warn("ExamStart: invalid question count", "subjectID", subjectID, "raw", r.FormValue("question_count"))
		http.Error(w, "Số câu hỏi không hợp lệ", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.CreateAttempt(subjectID, "exam", count)
	if err != nil {
		slog.Error("ExamStart: create attempt failed", "subjectID", subjectID, "count", count, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/take", attempt.ID)), http.StatusSeeOther)
}

func (h *Handler) ExamTake(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		slog.Warn("ExamTake: invalid attempt ID", "raw", r.PathValue("attemptID"))
		http.Error(w, "Attempt ID không hợp lệ", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.GetAttempt(attemptID)
	if err != nil {
		slog.Error("ExamTake: get attempt failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Không tìm thấy bài thi", http.StatusNotFound)
		return
	}

	questions, err := h.Questions.GetQuestions(attempt.SubjectID, attempt.TotalQuestions)
	if err != nil {
		slog.Error("ExamTake: get questions failed", "subjectID", attempt.SubjectID, "count", attempt.TotalQuestions, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	subject, err := h.Questions.GetSubject(attempt.SubjectID)
	if err != nil {
		slog.Error("ExamTake: get subject failed", "subjectID", attempt.SubjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	h.render(w, "exam.html", map[string]any{
		"Subject":   subject,
		"Attempt":   attempt,
		"Questions": questions,
	})
}

func (h *Handler) ExamSubmit(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		slog.Warn("ExamSubmit: invalid attempt ID", "raw", r.PathValue("attemptID"))
		http.Error(w, "Attempt ID không hợp lệ", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		slog.Warn("ExamSubmit: parse form failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Dữ liệu form không hợp lệ", http.StatusBadRequest)
		return
	}

	seen := make(map[int64]bool)
	for key := range r.Form {
		if !strings.HasPrefix(key, "q_") {
			continue
		}
		qid, err := strconv.ParseInt(strings.TrimPrefix(key, "q_"), 10, 64)
		if err != nil || seen[qid] {
			continue
		}
		seen[qid] = true

		values := r.Form[key]
		if len(values) == 0 {
			if err := h.Attempts.RecordAnswer(attemptID, qid, nil); err != nil {
				slog.Error("ExamSubmit: record answer failed", "attemptID", attemptID, "questionID", qid, "error", err)
				http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
				return
			}
			continue
		}
		var answerIDs []int64
		for _, v := range values {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				answerIDs = append(answerIDs, id)
			}
		}
		if err := h.Attempts.RecordAnswers(attemptID, qid, answerIDs); err != nil {
			slog.Error("ExamSubmit: record answers failed", "attemptID", attemptID, "questionID", qid, "error", err)
			http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
			return
		}
	}

	attempt, err := h.Attempts.FinishAttempt(attemptID)
	if err != nil {
		slog.Error("ExamSubmit: finish attempt failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}
	slog.Info("exam submitted", "attemptID", attemptID, "subjectID", attempt.SubjectID, "score", fmt.Sprintf("%.1f%%", attempt.Score), "correct", attempt.CorrectCount, "total", attempt.TotalQuestions)

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/result", attemptID)), http.StatusSeeOther)
}

func (h *Handler) ExamResult(w http.ResponseWriter, r *http.Request) {
	attemptID, err := pathInt64(r, "attemptID")
	if err != nil {
		slog.Warn("ExamResult: invalid attempt ID", "raw", r.PathValue("attemptID"))
		http.Error(w, "Attempt ID không hợp lệ", http.StatusBadRequest)
		return
	}

	attempt, err := h.Attempts.GetAttempt(attemptID)
	if err != nil {
		slog.Error("ExamResult: get attempt failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Không tìm thấy bài thi", http.StatusNotFound)
		return
	}

	details, err := h.Attempts.GetAttemptDetails(attemptID)
	if err != nil {
		slog.Error("ExamResult: get attempt details failed", "attemptID", attemptID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	subject, err := h.Questions.GetSubject(attempt.SubjectID)
	if err != nil {
		slog.Error("ExamResult: get subject failed", "subjectID", attempt.SubjectID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	h.render(w, "exam_result.html", map[string]any{
		"Subject": subject,
		"Attempt": attempt,
		"Details": details,
	})
}

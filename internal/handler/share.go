package handler

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
)

func (h *Handler) SharePage(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("shareCode")

	subject, err := h.Questions.GetSubjectByShareCode(code)
	if err != nil {
		slog.Warn("SharePage: subject not found", "shareCode", code, "error", err)
		http.Error(w, "Không tìm thấy đề thi", http.StatusNotFound)
		return
	}

	h.render(w, "share.html", map[string]any{
		"Subject":   subject,
		"ShareCode": code,
	})
}

func (h *Handler) ShareStart(w http.ResponseWriter, r *http.Request) {
	code := r.PathValue("shareCode")

	subject, err := h.Questions.GetSubjectByShareCode(code)
	if err != nil {
		slog.Warn("ShareStart: subject not found", "shareCode", code, "error", err)
		http.Error(w, "Không tìm thấy đề thi", http.StatusNotFound)
		return
	}

	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count <= 0 {
		count = subject.QuestionCount
	}
	if count > subject.QuestionCount {
		count = subject.QuestionCount
	}

	attempt, err := h.Attempts.CreateAttempt(subject.ID, "exam", count)
	if err != nil {
		slog.Error("ShareStart: create attempt failed", "subjectID", subject.ID, "count", count, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/take", attempt.ID)), http.StatusSeeOther)
}

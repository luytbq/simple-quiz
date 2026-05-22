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

	chapters, err := h.Questions.ListChapters(subject.ID)
	if err != nil {
		slog.Error("SharePage: list chapters failed", "subjectID", subject.ID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	h.render(w, "share.html", map[string]any{
		"Subject":     subject,
		"ShareCode":   code,
		"Chapters":    chapters,
		"HasChapters": len(chapters) > 1,
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

	r.ParseForm()
	chapterIDs := parseChapterIDs(r)

	available, err := h.Questions.CountQuestionsInChapters(subject.ID, chapterIDs)
	if err != nil {
		slog.Error("ShareStart: count questions failed", "subjectID", subject.ID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}
	if available == 0 {
		slog.Warn("ShareStart: no questions in selected chapters", "subjectID", subject.ID)
		http.Error(w, "Không có câu hỏi nào trong các chương đã chọn", http.StatusBadRequest)
		return
	}

	count, err := strconv.Atoi(r.URL.Query().Get("count"))
	if err != nil || count <= 0 {
		count = available
	}
	if count > available {
		count = available
	}

	attempt, err := h.Attempts.CreateAttempt(subject.ID, "exam", count)
	if err != nil {
		slog.Error("ShareStart: create attempt failed", "subjectID", subject.ID, "count", count, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	if err := h.Attempts.SetAttemptChapters(attempt.ID, chapterIDs); err != nil {
		slog.Error("ShareStart: set attempt chapters failed", "attemptID", attempt.ID, "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, h.url(fmt.Sprintf("/exam/%d/take", attempt.ID)), http.StatusSeeOther)
}

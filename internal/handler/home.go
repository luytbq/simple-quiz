package handler

import (
	"log/slog"
	"net/http"
)

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.Questions.ListSubjects()
	if err != nil {
		slog.Error("Home: list subjects failed", "error", err)
		http.Error(w, "Lỗi hệ thống", http.StatusInternalServerError)
		return
	}

	h.render(w, "home.html", map[string]any{
		"Subjects": subjects,
		"Imported": r.URL.Query().Get("imported"),
	})
}

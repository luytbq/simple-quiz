package handler

import "net/http"

func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	subjects, err := h.Questions.ListSubjects()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "home.html", map[string]any{
		"Subjects": subjects,
		"Imported": r.URL.Query().Get("imported"),
	})
}

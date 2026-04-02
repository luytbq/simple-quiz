package handler

import "net/http"

func (h *Handler) Guide(w http.ResponseWriter, r *http.Request) {
	h.render(w, "guide.html", nil)
}

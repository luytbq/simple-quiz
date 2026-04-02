package handler

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"quiz/internal/db"
	"quiz/internal/service"
)

func (h *Handler) ImportForm(w http.ResponseWriter, r *http.Request) {
	subjects, _ := h.Questions.ListSubjects()

	// Ensure all subjects have share codes
	for _, s := range subjects {
		if s.ShareCode == "" {
			h.Questions.EnsureShareCode(s.ID)
		}
	}
	if len(subjects) > 0 {
		subjects, _ = h.Questions.ListSubjects()
	}

	h.render(w, "import.html", map[string]any{
		"Subjects": subjects,
		"Error":    r.URL.Query().Get("error"),
		"Success":  r.URL.Query().Get("success"),
	})
}

func (h *Handler) CheckImport(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	rawText := r.FormValue("json_data")

	if strings.TrimSpace(rawText) == "" {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Vui lòng paste dữ liệu JSON")), http.StatusSeeOther)
		return
	}

	result := service.RefineImportData(rawText)

	if !result.OK {
		h.render(w, "import_preview.html", map[string]any{
			"Failed":   true,
			"HelpHTML": template.HTML(result.HelpHTML),
			"Changes":  result.Changes,
			"Errors":   result.Errors,
			"RawText":  rawText,
		})
		return
	}

	// Re-serialize refined data for hidden form
	refinedJSON, _ := json.MarshalIndent(result.Data, "", "  ")

	h.render(w, "import_preview.html", map[string]any{
		"Failed":      false,
		"Data":        result.Data,
		"Changes":     result.Changes,
		"RefinedJSON": string(refinedJSON),
		"Preview":     result.Data.Questions,
	})
}

func (h *Handler) ConfirmImport(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	refinedJSON := r.FormValue("refined_json")

	var importData db.ImportData
	if err := json.Unmarshal([]byte(refinedJSON), &importData); err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Lỗi khi xử lý dữ liệu")), http.StatusSeeOther)
		return
	}

	sub, count, err := h.Questions.ImportQuestions(importData)
	if err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Import thất bại: "+err.Error())), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Đã import %d câu hỏi cho '%s'", count, sub.Name)
	http.Redirect(w, r, h.url("/manage?success="+url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *Handler) DeleteSubject(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	sub, err := h.Questions.GetSubject(subjectID)
	if err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Subject not found")), http.StatusSeeOther)
		return
	}

	if err := h.Questions.DeleteSubject(subjectID); err != nil {
		http.Redirect(w, r, h.url("/manage?error="+url.QueryEscape("Delete failed: "+err.Error())), http.StatusSeeOther)
		return
	}

	msg := fmt.Sprintf("Đã xoá '%s'", sub.Name)
	http.Redirect(w, r, h.url("/manage?success="+url.QueryEscape(msg)), http.StatusSeeOther)
}

func (h *Handler) ExportSubject(w http.ResponseWriter, r *http.Request) {
	subjectID, err := pathInt64(r, "subjectID")
	if err != nil {
		http.Error(w, "Invalid subject ID", http.StatusBadRequest)
		return
	}

	data, err := h.Questions.ExportSubject(subjectID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	safeName := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '"' {
			return -1
		}
		return r
	}, data.Subject)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.json"`, safeName))
	w.Write(jsonBytes)
}

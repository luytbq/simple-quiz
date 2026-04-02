package handler

import (
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"quiz/internal/service"
)

var buildVersion = fmt.Sprintf("%d", time.Now().Unix())

type Handler struct {
	Questions *service.QuestionService
	Attempts  *service.AttemptService
	BasePath  string
	templates map[string]*template.Template
}

func New(qs *service.QuestionService, as *service.AttemptService, templateFS fs.FS, basePath string) *Handler {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"percent": func(score float64) string {
			return fmt.Sprintf("%.1f%%", score)
		},
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"bp": func() string { return basePath },
		"v":  func() string { return buildVersion },
	}

	pages := []string{
		"home.html", "practice.html", "practice_result.html",
		"exam_setup.html", "exam.html", "exam_result.html",
		"import.html", "stats.html", "stats_detail.html", "guide.html", "share.html",
	}

	templates := make(map[string]*template.Template)
	for _, page := range pages {
		templates[page] = template.Must(
			template.New("").Funcs(funcMap).ParseFS(templateFS,
				"templates/layout.html", "templates/"+page),
		)
	}

	return &Handler{
		Questions: qs,
		Attempts:  as,
		BasePath:  basePath,
		templates: templates,
	}
}

func (h *Handler) render(w http.ResponseWriter, name string, data any) {
	tmpl, ok := h.templates[name]
	if !ok {
		slog.Error("template not found", "name", name)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if err := tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		slog.Error("render template", "name", name, "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Home
	mux.HandleFunc("GET /{$}", h.Home)

	// Manage
	mux.HandleFunc("GET /manage", h.ImportForm)
	mux.HandleFunc("POST /manage", h.ImportSubmit)
	mux.HandleFunc("POST /manage/{subjectID}/delete", h.DeleteSubject)
	mux.HandleFunc("GET /manage/{subjectID}/export", h.ExportSubject)

	// Practice (flashcard)
	mux.HandleFunc("GET /practice/{subjectID}", h.PracticeStart)
	mux.HandleFunc("GET /practice/{subjectID}/question", h.PracticeQuestion)
	mux.HandleFunc("POST /practice/{subjectID}/answer", h.PracticeAnswer)

	// Exam
	mux.HandleFunc("GET /exam/{subjectID}", h.ExamSetup)
	mux.HandleFunc("POST /exam/{subjectID}/start", h.ExamStart)
	mux.HandleFunc("GET /exam/{attemptID}/take", h.ExamTake)
	mux.HandleFunc("POST /exam/{attemptID}/submit", h.ExamSubmit)
	mux.HandleFunc("GET /exam/{attemptID}/result", h.ExamResult)

	// Stats
	mux.HandleFunc("GET /stats", h.Stats)
	mux.HandleFunc("GET /stats/{subjectID}", h.SubjectStats)

	// Share
	mux.HandleFunc("GET /s/{shareCode}", h.SharePage)
	mux.HandleFunc("GET /s/{shareCode}/start", h.ShareStart)

	// Guide
	mux.HandleFunc("GET /guide", h.Guide)
}

func (h *Handler) url(path string) string {
	return h.BasePath + path
}

func pathInt64(r *http.Request, name string) (int64, error) {
	v := r.PathValue(name)
	return strconv.ParseInt(v, 10, 64)
}

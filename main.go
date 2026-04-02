package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"quiz/cmd"
	"quiz/internal/db"
	"quiz/internal/handler"
	"quiz/internal/service"
)

//go:embed templates/* static/*
var content embed.FS

func main() {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "quiz.db"
	}

	// CLI: quiz import <file.json>
	if len(os.Args) >= 2 && os.Args[1] == "import" {
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "Usage: quiz import <file.json>")
			os.Exit(1)
		}
		if err := cmd.RunImport(os.Args[2], dbPath); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Web server
	database, err := db.Open(dbPath)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		slog.Error("migrate database", "error", err)
		os.Exit(1)
	}

	basePath := strings.TrimRight(os.Getenv("BASE_PATH"), "/")

	qs := &service.QuestionService{DB: database}
	as := &service.AttemptService{DB: database}

	templateFS, _ := fs.Sub(content, ".")
	h := handler.New(qs, as, templateFS, basePath)

	staticFS, _ := fs.Sub(content, "static")

	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	// Wrap with base path prefix stripping
	var root http.Handler = mux
	if basePath != "" {
		root = http.StripPrefix(basePath, mux)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	slog.Info("starting server", "port", port, "basePath", basePath)
	if err := http.ListenAndServe(":"+port, root); err != nil {
		slog.Error("server error", "error", err)
		os.Exit(1)
	}
}

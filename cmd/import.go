package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"quiz/internal/db"
	"quiz/internal/service"
)

func RunImport(filePath string, dbPath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	var importData db.ImportData
	if err := json.Unmarshal(data, &importData); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}

	if importData.Subject == "" {
		return fmt.Errorf("missing 'subject' field in JSON")
	}
	if len(importData.Questions) == 0 {
		return fmt.Errorf("no questions found in JSON")
	}

	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	if err := db.Migrate(database); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	qs := &service.QuestionService{DB: database}
	sub, count, err := qs.ImportQuestions(importData)
	if err != nil {
		return fmt.Errorf("import: %w", err)
	}

	fmt.Printf("Imported %d questions for subject '%s' (id=%d)\n", count, sub.Name, sub.ID)
	return nil
}

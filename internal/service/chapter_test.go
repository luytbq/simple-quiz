package service

import (
	"testing"

	"quiz/internal/db"
)

// importDataWithChapters builds an ImportData with two chapters and the given
// number of questions in each.
func importDataWithChapters(subject string, n1, imp1, n2, imp2 int) db.ImportData {
	data := db.ImportData{
		Subject: subject,
		Chapters: []db.ImportChapter{
			{ID: 1, Name: "Chương 1", Importance: imp1},
			{ID: 2, Name: "Chương 2", Importance: imp2},
		},
	}
	mk := func(chapterID, idx int) db.ImportQuestion {
		cid := chapterID
		return db.ImportQuestion{
			Content:   "Q",
			ChapterID: &cid,
			Answers: []db.ImportAnswer{
				{Label: "A", Content: "x", IsCorrect: true},
				{Label: "B", Content: "y", IsCorrect: false},
			},
		}
	}
	for i := 0; i < n1; i++ {
		data.Questions = append(data.Questions, mk(1, i))
	}
	for i := 0; i < n2; i++ {
		data.Questions = append(data.Questions, mk(2, i))
	}
	return data
}

func chapterByImportance(chapters []db.Chapter, importance int) *db.Chapter {
	for i := range chapters {
		if chapters[i].Importance == importance {
			return &chapters[i]
		}
	}
	return nil
}

func TestImportQuestions_WithChapters(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, count, err := qs.ImportQuestions(importDataWithChapters("Subj", 3, 4, 7, 10))
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}
	if count != 10 {
		t.Fatalf("count = %d, want 10", count)
	}

	chapters, err := qs.ListChapters(sub.ID)
	if err != nil {
		t.Fatalf("ListChapters: %v", err)
	}
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2", len(chapters))
	}
	if c := chapterByImportance(chapters, 4); c == nil || c.QuestionCount != 3 {
		t.Errorf("importance-4 chapter should have 3 questions, got %+v", c)
	}
	if c := chapterByImportance(chapters, 10); c == nil || c.QuestionCount != 7 {
		t.Errorf("importance-10 chapter should have 7 questions, got %+v", c)
	}
}

func TestImportQuestions_LegacyDefaultChapter(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	sub, _, err := qs.ImportQuestions(sampleImportData()) // no chapters
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}
	chapters, err := qs.ListChapters(sub.ID)
	if err != nil {
		t.Fatalf("ListChapters: %v", err)
	}
	if len(chapters) != 1 {
		t.Fatalf("got %d chapters, want 1 default", len(chapters))
	}
	if chapters[0].Name != db.DefaultChapterName {
		t.Errorf("default chapter name = %q, want %q", chapters[0].Name, db.DefaultChapterName)
	}
	if chapters[0].QuestionCount != 2 {
		t.Errorf("default chapter should hold all 2 questions, got %d", chapters[0].QuestionCount)
	}
}

func TestImportQuestions_MissingChapterIDGoesToDefault(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}

	cid := 1
	data := db.ImportData{
		Subject:  "Mixed",
		Chapters: []db.ImportChapter{{ID: 1, Name: "Chương 1", Importance: 8}},
		Questions: []db.ImportQuestion{
			{Content: "has chapter", ChapterID: &cid, Answers: []db.ImportAnswer{
				{Label: "A", Content: "a", IsCorrect: true}, {Label: "B", Content: "b"}}},
			{Content: "no chapter", Answers: []db.ImportAnswer{
				{Label: "A", Content: "a", IsCorrect: true}, {Label: "B", Content: "b"}}},
		},
	}
	sub, _, err := qs.ImportQuestions(data)
	if err != nil {
		t.Fatalf("ImportQuestions: %v", err)
	}
	chapters, _ := qs.ListChapters(sub.ID)
	if len(chapters) != 2 {
		t.Fatalf("got %d chapters, want 2 (declared + default)", len(chapters))
	}
	def := chapterByImportance(chapters, db.DefaultImportance)
	if def == nil || def.Name != db.DefaultChapterName || def.QuestionCount != 1 {
		t.Errorf("default chapter should hold the chapter-less question, got %+v", def)
	}
}

func TestCountQuestionsInChapters(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	sub, _, _ := qs.ImportQuestions(importDataWithChapters("Subj", 3, 4, 7, 10))
	chapters, _ := qs.ListChapters(sub.ID)

	if n, _ := qs.CountQuestionsInChapters(sub.ID, nil); n != 10 {
		t.Errorf("all chapters = %d, want 10", n)
	}
	c10 := chapterByImportance(chapters, 10)
	if n, _ := qs.CountQuestionsInChapters(sub.ID, []int64{c10.ID}); n != 7 {
		t.Errorf("importance-10 chapter = %d, want 7", n)
	}
}

func TestGetRandomQuestionInChapters_OnlySelected(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	sub, _, _ := qs.ImportQuestions(importDataWithChapters("Subj", 3, 4, 7, 10))
	chapters, _ := qs.ListChapters(sub.ID)
	c4 := chapterByImportance(chapters, 4)

	for i := 0; i < 20; i++ {
		q, err := qs.GetRandomQuestionInChapters(sub.ID, []int64{c4.ID}, nil)
		if err != nil {
			t.Fatalf("GetRandomQuestionInChapters: %v", err)
		}
		if q.ChapterID == nil || *q.ChapterID != c4.ID {
			t.Fatalf("question from wrong chapter: %+v", q.ChapterID)
		}
	}
}

func TestGetExamQuestions_Allocation(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	// ch importance 1 (5 q) + ch importance 10 (5 q); request 6.
	sub, _, _ := qs.ImportQuestions(importDataWithChapters("Subj", 5, 1, 5, 10))
	chapters, _ := qs.ListChapters(sub.ID)
	low := chapterByImportance(chapters, 1)
	high := chapterByImportance(chapters, 10)

	questions, err := qs.GetExamQuestions(sub.ID, nil, 6)
	if err != nil {
		t.Fatalf("GetExamQuestions: %v", err)
	}
	if len(questions) != 6 {
		t.Fatalf("got %d questions, want 6", len(questions))
	}
	perChapter := map[int64]int{}
	for _, q := range questions {
		if q.ChapterID == nil {
			t.Fatal("question missing chapter_id")
		}
		perChapter[*q.ChapterID]++
	}
	// min 1 each, remainder to the importance-10 chapter (capped at 5).
	if perChapter[low.ID] != 1 {
		t.Errorf("low-importance chapter got %d, want 1", perChapter[low.ID])
	}
	if perChapter[high.ID] != 5 {
		t.Errorf("high-importance chapter got %d, want 5", perChapter[high.ID])
	}
}

func TestExportSubject_ChaptersRoundtrip(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	sub, _, _ := qs.ImportQuestions(importDataWithChapters("Subj", 3, 4, 7, 10))

	exported, err := qs.ExportSubject(sub.ID)
	if err != nil {
		t.Fatalf("ExportSubject: %v", err)
	}
	if len(exported.Chapters) != 2 {
		t.Fatalf("exported %d chapters, want 2", len(exported.Chapters))
	}
	for _, q := range exported.Questions {
		if q.ChapterID == nil {
			t.Error("exported question missing chapter_id")
		}
	}

	// Re-import into a fresh DB and verify chapter structure survives.
	d2 := setupTestDB(t)
	qs2 := &QuestionService{DB: d2}
	sub2, _, err := qs2.ImportQuestions(*exported)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	chapters2, _ := qs2.ListChapters(sub2.ID)
	if len(chapters2) != 2 {
		t.Fatalf("round-trip chapters = %d, want 2", len(chapters2))
	}
	if c := chapterByImportance(chapters2, 10); c == nil || c.QuestionCount != 7 {
		t.Errorf("round-trip importance-10 chapter wrong: %+v", c)
	}
}

func TestExportSubject_SingleChapterLegacy(t *testing.T) {
	d := setupTestDB(t)
	qs := &QuestionService{DB: d}
	sub, _, _ := qs.ImportQuestions(sampleImportData()) // single default chapter

	exported, err := qs.ExportSubject(sub.ID)
	if err != nil {
		t.Fatalf("ExportSubject: %v", err)
	}
	if len(exported.Chapters) != 0 {
		t.Errorf("single-chapter export should omit chapters, got %d", len(exported.Chapters))
	}
	for _, q := range exported.Questions {
		if q.ChapterID != nil {
			t.Error("single-chapter export should omit chapter_id on questions")
		}
	}
}

func TestNormalizeChapters_FillsData(t *testing.T) {
	missingCID := 99
	data := db.ImportData{
		Subject: "S",
		Chapters: []db.ImportChapter{
			{ID: 1, Name: "", Importance: 0},     // missing name + importance
			{ID: 2, Name: "Big", Importance: 50}, // out of range
		},
		Questions: []db.ImportQuestion{
			{Content: "q", ChapterID: &missingCID, Answers: []db.ImportAnswer{{Label: "A", Content: "a", IsCorrect: true}}},
		},
	}
	changes := normalizeChapters(&data)
	if len(changes) == 0 {
		t.Fatal("expected change descriptions")
	}
	// name filled from id
	if data.Chapters[0].Name != "1" {
		t.Errorf("missing name should become id: got %q", data.Chapters[0].Name)
	}
	if data.Chapters[0].Importance != db.DefaultImportance {
		t.Errorf("missing importance should default to %d, got %d", db.DefaultImportance, data.Chapters[0].Importance)
	}
	if data.Chapters[1].Importance != 10 {
		t.Errorf("out-of-range importance should clamp to 10, got %d", data.Chapters[1].Importance)
	}
	// undeclared chapter_id auto-created
	found := false
	for _, c := range data.Chapters {
		if c.ID == 99 {
			found = true
		}
	}
	if !found {
		t.Error("undeclared chapter_id should be auto-created")
	}
}

func TestNormalizeChapters_Idempotent(t *testing.T) {
	data := importDataWithChapters("S", 1, 4, 1, 10)
	first := normalizeChapters(&data)
	second := normalizeChapters(&data)
	if len(first) != 0 {
		t.Errorf("clean data should yield no changes, got %v", first)
	}
	if len(second) != 0 {
		t.Errorf("second pass should be no-op, got %v", second)
	}
}

package service

import "testing"

func sumAlloc(m map[int64]int) int {
	s := 0
	for _, v := range m {
		s += v
	}
	return s
}

func TestAllocateQuestions_UserExample(t *testing.T) {
	// 100 questions: ch1 importance 4 (10 câu), ch2 importance 10 (40 câu),
	// ch3 importance 8 (50 câu). Target 30.
	chapters := []chapterAlloc{
		{ID: 1, Importance: 4, Available: 10},
		{ID: 2, Importance: 10, Available: 40},
		{ID: 3, Importance: 8, Available: 50},
	}
	got := allocateQuestions(chapters, 30)

	if s := sumAlloc(got); s != 30 {
		t.Fatalf("sum = %d, want 30", s)
	}
	for id, n := range got {
		if n < 1 {
			t.Errorf("chapter %d got %d, want >= 1", id, n)
		}
	}
	// More important chapters get more questions: ch2(10) > ch3(8) > ch1(4).
	if !(got[2] >= got[3] && got[3] >= got[1]) {
		t.Errorf("expected ch2 >= ch3 >= ch1 by importance, got %v", got)
	}
}

func TestAllocateQuestions_ExactSum(t *testing.T) {
	chapters := []chapterAlloc{
		{ID: 1, Importance: 1, Available: 5},
		{ID: 2, Importance: 7, Available: 20},
		{ID: 3, Importance: 3, Available: 8},
		{ID: 4, Importance: 10, Available: 50},
	}
	for target := 0; target <= 90; target++ {
		got := allocateQuestions(chapters, target)
		total := 5 + 20 + 8 + 50
		want := target
		if want > total {
			want = total
		}
		if s := sumAlloc(got); s != want {
			t.Fatalf("target %d: sum = %d, want %d (%v)", target, s, want, got)
		}
		for id, n := range got {
			var avail int
			for _, c := range chapters {
				if c.ID == id {
					avail = c.Available
				}
			}
			if n > avail {
				t.Fatalf("target %d: chapter %d got %d > available %d", target, id, n, avail)
			}
			if n < 0 {
				t.Fatalf("target %d: chapter %d negative %d", target, id, n)
			}
		}
	}
}

func TestAllocateQuestions_CapRespected(t *testing.T) {
	// Very important chapter but tiny availability — the rest must absorb.
	chapters := []chapterAlloc{
		{ID: 1, Importance: 10, Available: 2},
		{ID: 2, Importance: 5, Available: 40},
	}
	got := allocateQuestions(chapters, 20)
	if got[1] != 2 {
		t.Errorf("chapter 1 capped at available: got %d, want 2", got[1])
	}
	if got[2] != 18 {
		t.Errorf("chapter 2 absorbs remainder: got %d, want 18", got[2])
	}
	if s := sumAlloc(got); s != 20 {
		t.Errorf("sum = %d, want 20", s)
	}
}

func TestAllocateQuestions_TargetSmallerThanChapters(t *testing.T) {
	// Target 2 but 3 chapters: total must be exactly 2 (criterion 3 wins over
	// min-one-each), keeping the most important chapters.
	chapters := []chapterAlloc{
		{ID: 1, Importance: 2, Available: 10},
		{ID: 2, Importance: 9, Available: 10},
		{ID: 3, Importance: 6, Available: 10},
	}
	got := allocateQuestions(chapters, 2)
	if s := sumAlloc(got); s != 2 {
		t.Fatalf("sum = %d, want 2 (%v)", s, got)
	}
	if got[2] != 1 || got[3] != 1 {
		t.Errorf("expected the two most important chapters (2,3) to be kept, got %v", got)
	}
	if got[1] != 0 {
		t.Errorf("least important chapter should be dropped, got %v", got)
	}
}

func TestAllocateQuestions_TargetEqualsTotal(t *testing.T) {
	chapters := []chapterAlloc{
		{ID: 1, Importance: 3, Available: 7},
		{ID: 2, Importance: 8, Available: 12},
	}
	got := allocateQuestions(chapters, 19)
	if got[1] != 7 || got[2] != 12 {
		t.Errorf("each chapter should equal availability: got %v", got)
	}
}

func TestAllocateQuestions_SingleChapter(t *testing.T) {
	chapters := []chapterAlloc{{ID: 1, Importance: 5, Available: 40}}
	got := allocateQuestions(chapters, 25)
	if got[1] != 25 {
		t.Errorf("single chapter got %d, want 25", got[1])
	}
}

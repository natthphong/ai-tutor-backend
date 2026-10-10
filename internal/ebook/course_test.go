package ebook

import (
	"sort"
	"strings"
	"testing"
)

func TestCourseHas145TaughtLessonsWithEightQuizItemsAndListenFirstShadowing(t *testing.T) {
	course, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	if len(course.Lessons) != 145 {
		t.Fatalf("lessons = %d, want 145", len(course.Lessons))
	}
	for index, lesson := range course.Lessons {
		if lesson.Ordinal != index+1 || lesson.ID == "" || lesson.UnitID == "" {
			t.Fatalf("lesson %d has unstable identity: %#v", index+1, lesson)
		}
		if len(lesson.Examples) != 6 {
			t.Fatalf("lesson %s examples = %d, want exactly 6", lesson.ID, len(lesson.Examples))
		}
		if len(lesson.Vocabulary) != 10 {
			t.Fatalf("lesson %s vocabulary = %d, want exactly 10", lesson.ID, len(lesson.Vocabulary))
		}
		kinds := make([]string, 0, len(lesson.Quiz))
		for _, item := range lesson.Quiz {
			if item.ID == "" || len(item.Answers) == 0 {
				t.Fatalf("lesson %s has invalid quiz item: %#v", lesson.ID, item)
			}
			kinds = append(kinds, item.Kind)
		}
		if strings.Join(kinds, ",") != strings.Join(LearnEbookQuizKinds, ",") {
			t.Fatalf("lesson %s quiz kinds = %v, want %v", lesson.ID, kinds, LearnEbookQuizKinds)
		}
		examples := map[string]bool{}
		for _, example := range lesson.Examples {
			examples[normalizeCourseText(example.EN)] = true
		}
		if len(lesson.Shadowing) != 3 || len(lesson.Listening) != 3 {
			t.Fatalf("lesson %s listening/shadowing = %d/%d, want 3/3", lesson.ID, len(lesson.Listening), len(lesson.Shadowing))
		}
		for i, line := range lesson.Shadowing {
			if line != lesson.Listening[i].AudioEN {
				t.Fatalf("lesson %s shadowing line %d is not the line the learner heard: %q", lesson.ID, i+1, line)
			}
			if examples[normalizeCourseText(line)] {
				t.Fatalf("lesson %s listening line was already shown as an example: %q", lesson.ID, line)
			}
		}
	}
}

// The first course version reused one explanation template for every unit and
// 136 terms across 1,450 vocabulary entries. Both are release blockers now.
func TestCourseTeachesEveryUnitDifferentlyWithGloballyNewVocabulary(t *testing.T) {
	course, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	audit := course.Audit()
	if audit.UniqueTeachings != 145 {
		t.Fatalf("unique explanations = %d, want one per lesson", audit.UniqueTeachings)
	}
	if audit.UniqueTerms < MinUniqueTerms {
		t.Fatalf("globally unique vocabulary terms = %d, want at least %d", audit.UniqueTerms, MinUniqueTerms)
	}
	covered := 0
	for _, lesson := range course.Lessons {
		body := ""
		for _, example := range lesson.Examples {
			body += " " + strings.ToLower(example.EN)
		}
		for _, item := range lesson.Listening {
			body += " " + strings.ToLower(item.AudioEN)
		}
		for _, word := range lesson.Vocabulary {
			head := strings.Fields(strings.ToLower(word.Term))[0]
			if len(head) > 4 {
				head = head[:4]
			}
			if strings.Contains(body, head) {
				covered++
			}
		}
	}
	// Vocabulary is practised in the sentences of its own unit, not listed
	// beside unrelated examples.
	if covered*100 < audit.Vocabulary*75 {
		t.Fatalf("only %d of %d vocabulary terms appear in their unit's examples or listening lines", covered, audit.Vocabulary)
	}
}

func TestCoursePathStartsAtA1AndNeverStepsDownALevel(t *testing.T) {
	course, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	path := course.Path()
	if len(path) != 145 || path[0].Level != "A1" || path[0].PathOrder != 1 {
		t.Fatalf("path start = %s %s order %d, want an A1 unit at order 1", path[0].ID, path[0].Level, path[0].PathOrder)
	}
	rank := map[string]int{"A1": 0, "A2": 1, "B1": 2, "B2": 3}
	counts := map[string]int{}
	for i := 1; i < len(path); i++ {
		if rank[path[i].Level] < rank[path[i-1].Level] {
			t.Fatalf("path steps down from %s (%s) to %s (%s)", path[i-1].ID, path[i-1].Level, path[i].ID, path[i].Level)
		}
	}
	for _, lesson := range path {
		counts[lesson.Level]++
	}
	levels := make([]string, 0, len(counts))
	for level := range counts {
		levels = append(levels, level)
	}
	sort.Strings(levels)
	if strings.Join(levels, ",") != "A1,A2,B1,B2" || counts["A1"] < 10 {
		t.Fatalf("level coverage = %v, want all four levels with a real A1 block", counts)
	}
}

func TestCoursePublicLessonHidesAnswerKeysAndScheduledReview(t *testing.T) {
	course, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	public := course.Lessons[0].Public()
	for _, quiz := range public.Quiz {
		if len(quiz.Answers) != 0 || quiz.ExplanationTH != "" {
			t.Fatalf("public quiz leaked its answer key: %#v", quiz)
		}
	}
	for _, item := range public.Listening {
		if item.AnswerTH != "" || item.MeaningTH != "" || item.AudioEN == "" || len(item.OptionsTH) != 3 {
			t.Fatalf("public listening item is wrong: %#v", item)
		}
	}
	if len(public.Review) != 0 {
		t.Fatalf("public lesson leaked scheduled review prompts: %#v", public.Review)
	}
	if len(course.Lessons[0].Quiz[0].Answers) == 0 || course.Lessons[0].Listening[0].AnswerTH == "" || len(course.Lessons[0].Review) != 2 {
		t.Fatal("Public() changed the private lesson")
	}
}

func TestCourseRejectsBrokenLessonContracts(t *testing.T) {
	original, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*Lesson)
	}{
		{"nine vocabulary terms", func(l *Lesson) { l.Vocabulary = l.Vocabulary[:9] }},
		{"duplicate vocabulary term", func(l *Lesson) { l.Vocabulary[1].Term = l.Vocabulary[0].Term }},
		{"quiz without accepted answer", func(l *Lesson) { l.Quiz[0].Answers = nil }},
		{"choice answer that is not an option", func(l *Lesson) { l.Quiz[0].Answers = []string{"not an option"} }},
		{"reorder words that cannot rebuild the answer", func(l *Lesson) { l.Quiz[4].Options = append([]string(nil), l.Quiz[4].Options[1:]...) }},
		{"fix item without a mistake", func(l *Lesson) { l.Quiz[5].PromptEN = l.Quiz[5].Answers[0] }},
		{"listening line copied from the examples", func(l *Lesson) {
			l.Listening[0].AudioEN = l.Examples[0].EN
			l.Shadowing[0] = l.Examples[0].EN
		}},
		{"shadowing line the learner did not hear", func(l *Lesson) { l.Shadowing[0] = "This sentence was never played." }},
		{"listening answer outside the options", func(l *Lesson) { l.Listening[1].AnswerTH = "ไม่มีในตัวเลือก" }},
		{"template explanation", func(l *Lesson) { l.ExplanationTH = "ใช้โครงสร้างนี้" }},
		{"unit without review prompts", func(l *Lesson) { l.Review = nil }},
		{"unknown level", func(l *Lesson) { l.Level = "C2" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := *original
			copy.Lessons = append([]Lesson(nil), original.Lessons...)
			lesson := copy.Lessons[0]
			lesson.Vocabulary = append([]Vocabulary(nil), lesson.Vocabulary...)
			lesson.Quiz = append([]QuizItem(nil), lesson.Quiz...)
			lesson.Listening = append([]ListeningItem(nil), lesson.Listening...)
			lesson.Shadowing = append([]string(nil), lesson.Shadowing...)
			tc.edit(&lesson)
			copy.Lessons[0] = lesson
			if err := copy.Validate(); err == nil {
				t.Fatal("Validate accepted malformed course")
			}
		})
	}
}

func TestCourseLessonLookupUsesStableIDs(t *testing.T) {
	course, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	want := course.Lessons[72]
	got, ok := course.Lesson(want.ID)
	if !ok || got.ID != want.ID || got.Ordinal != want.Ordinal {
		t.Fatalf("Lesson(%q) = %#v, %v", want.ID, got, ok)
	}
	if _, ok := course.Lesson("missing-unit"); ok {
		t.Fatal("Lesson returned a missing unit")
	}
}

package ebook

import (
	"strings"
	"testing"
)

func TestCourseHas145LessonsTenUniqueVocabularyFiveQuizItemsAndVerbatimShadowing(t *testing.T) {
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
		if len(lesson.Vocabulary) != 10 {
			t.Fatalf("lesson %s vocabulary = %d, want exactly 10", lesson.ID, len(lesson.Vocabulary))
		}
		terms := map[string]bool{}
		for _, word := range lesson.Vocabulary {
			term := strings.ToLower(strings.TrimSpace(word.Term))
			if term == "" || terms[term] {
				t.Fatalf("lesson %s has empty or duplicate vocabulary term %q", lesson.ID, word.Term)
			}
			terms[term] = true
		}
		if len(lesson.Quiz) != 5 {
			t.Fatalf("lesson %s quiz = %d, want exactly 5", lesson.ID, len(lesson.Quiz))
		}
		quizIDs := map[string]bool{}
		for _, item := range lesson.Quiz {
			if item.ID == "" || quizIDs[item.ID] || len(item.Answers) == 0 {
				t.Fatalf("lesson %s has invalid quiz item: %#v", lesson.ID, item)
			}
			quizIDs[item.ID] = true
		}
		if len(lesson.Shadowing) != 3 {
			t.Fatalf("lesson %s shadowing = %d, want exactly 3", lesson.ID, len(lesson.Shadowing))
		}
		examples := map[string]bool{}
		for _, example := range lesson.Examples {
			examples[example.EN] = true
		}
		for _, line := range lesson.Shadowing {
			if !examples[line] {
				t.Fatalf("lesson %s shadowing line is not verbatim from examples: %q", lesson.ID, line)
			}
		}
	}
}

func TestCourseRejectsBrokenLessonContracts(t *testing.T) {
	original, err := LoadCourse()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		edit func(*Course)
	}{
		{
			name: "nine vocabulary terms",
			edit: func(course *Course) { course.Lessons[0].Vocabulary = course.Lessons[0].Vocabulary[:9] },
		},
		{
			name: "duplicate vocabulary term",
			edit: func(course *Course) { course.Lessons[0].Vocabulary[1].Term = course.Lessons[0].Vocabulary[0].Term },
		},
		{
			name: "quiz without accepted answer",
			edit: func(course *Course) { course.Lessons[0].Quiz[0].Answers = nil },
		},
		{
			name: "shadow line outside examples",
			edit: func(course *Course) { course.Lessons[0].Shadowing[0] = "This sentence is not in the examples." },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			copy := *original
			copy.Lessons = append([]Lesson(nil), original.Lessons...)
			copy.Lessons[0].Vocabulary = append([]Vocabulary(nil), original.Lessons[0].Vocabulary...)
			copy.Lessons[0].Quiz = append([]QuizItem(nil), original.Lessons[0].Quiz...)
			copy.Lessons[0].Shadowing = append([]string(nil), original.Lessons[0].Shadowing...)
			tc.edit(&copy)
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

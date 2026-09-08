package ebook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func fixtureUnit(id string, n int) Unit {
	return Unit{
		ID: id, Number: n, Title: "Fixture unit", LessonPage: 10, ExercisePage: 11,
		AnswerPages: []int{350},
		Sections:    []Section{{ID: id + ".1", ItemIDs: []string{id + ".1:1", id + ".1:2"}}},
	}
}

func fixturePack(ids ...string) Pack {
	questions := make([]Question, 0, len(ids))
	for _, id := range ids {
		questions = append(questions, Question{
			ID: id, Kind: "choice", Prompt: "Choose the answer", InstructionTH: "เลือกคำตอบ",
			Options: []Option{{ID: "a", Text: "one"}, {ID: "b", Text: "two"}}, Answers: []string{"a"},
		})
	}
	return Pack{
		ExplanationTH: "คำอธิบาย", Questions: questions,
		Vocabulary: []Word{{Term: "term", Meaning: "ความหมาย", Example: "an example"}},
		Pattern:    "I am [adjective].", SpeakingPrompt: "Tell a short story.", SpeakingTH: "เล่าเรื่องสั้น ๆ",
		ListeningPrompt: "Listen and answer.", ListeningTH: "ฟังแล้วตอบ",
	}
}

func TestLoadFixtureManifestKeepsThe145Unit392PageBook(t *testing.T) {
	units := make([]Unit, 145)
	for i := range units {
		units[i] = fixtureUnit(fmt.Sprintf("unit-%03d", i+1), i+1)
		units[i].Sections[0].ID = units[i].ID + ".1"
		units[i].Sections[0].ItemIDs = []string{units[i].ID + ".1:1", units[i].ID + ".1:2"}
	}
	manifest := Book{ID: "private-fixture", Title: "Private fixture", Version: "fixture-v1", PageCount: 392, Units: units}
	dir := t.TempDir()
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0600); err != nil {
		t.Fatal(err)
	}

	book, err := Load(dir)
	if err != nil {
		t.Fatalf("Load fixture manifest: %v", err)
	}
	if book.PageCount != 392 || len(book.Units) != 145 {
		t.Fatalf("book shape = %d pages, %d units", book.PageCount, len(book.Units))
	}
	unit, ok := book.Unit("unit-145")
	if !ok || unit.Number != 145 || len(unit.Sections[0].ItemIDs) != 2 {
		t.Fatalf("last unit was not preserved: %#v, found=%t", unit, ok)
	}
}

func TestPackValidateRequiresEachOriginalItemExactlyOnce(t *testing.T) {
	unit := fixtureUnit("unit-1", 1)
	valid := fixturePack("unit-1.1:1", "unit-1.1:2")
	if err := valid.Validate(unit); err != nil {
		t.Fatalf("valid pack rejected: %v", err)
	}

	for name, pack := range map[string]Pack{
		"omitted source item": fixturePack("unit-1.1:1"),
		"invented item":       fixturePack("unit-1.1:1", "invented:1"),
		"duplicate item":      fixturePack("unit-1.1:1", "unit-1.1:1", "unit-1.1:2"),
	} {
		t.Run(name, func(t *testing.T) {
			if err := pack.Validate(unit); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestPackPublicDoesNotExposeAnswersForUnworkedQuestions(t *testing.T) {
	pack := fixturePack("unit-1.1:1", "unit-1.1:2")
	pack.Questions[1].Example = true
	pack.Questions[1].ExplanationTH = "worked explanation"
	public := pack.Public()
	if len(public.Questions[0].Answers) != 0 || public.Questions[0].ExplanationTH != "" {
		t.Fatalf("ordinary question leaked solution: %#v", public.Questions[0])
	}
	if got := public.Questions[1].Answers; len(got) != 1 || got[0] != "a" {
		t.Fatalf("worked example lost answer: %#v", public.Questions[1])
	}
	if len(pack.Questions[0].Answers) == 0 {
		t.Fatal("Public mutated the stored pack")
	}
}

func TestValidatePagesRejectsIncompleteOrEmptyPrivateAssets(t *testing.T) {
	book := &Book{PageCount: 2}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "page-001.jpg"), []byte("fixture page"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := book.ValidatePages(dir); err == nil {
		t.Fatal("missing private page was accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "page-002.jpg"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := book.ValidatePages(dir); err == nil {
		t.Fatal("empty private page was accepted")
	}
	if err := os.WriteFile(filepath.Join(dir, "page-002.jpg"), []byte("fixture page"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := book.ValidatePages(dir); err != nil {
		t.Fatalf("complete private fixture rejected: %v", err)
	}
}

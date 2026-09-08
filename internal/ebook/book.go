package ebook

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Section struct {
	ID      string   `json:"id"`
	ItemIDs []string `json:"item_ids"`
}
type Unit struct {
	ID           string    `json:"id"`
	Number       int       `json:"number"`
	Title        string    `json:"title"`
	LessonPage   int       `json:"lesson_page"`
	ExercisePage int       `json:"exercise_page"`
	AnswerPages  []int     `json:"answer_pages"`
	Sections     []Section `json:"sections"`
	LessonText   string    `json:"lesson_text,omitempty"`
	ExerciseText string    `json:"exercise_text,omitempty"`
	AnswerText   string    `json:"answer_text,omitempty"`
}
type Book struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Version   string `json:"version"`
	PageCount int    `json:"page_count"`
	Units     []Unit `json:"units"`
}

func Load(dir string) (*Book, error) {
	b, e := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if e != nil {
		return nil, e
	}
	var book Book
	if e = json.Unmarshal(b, &book); e != nil {
		return nil, e
	}
	if book.Version == "" || book.PageCount < 1 || len(book.Units) == 0 {
		return nil, fmt.Errorf("invalid ebook manifest")
	}
	return &book, nil
}
func (b *Book) Unit(id string) (Unit, bool) {
	if b != nil {
		for _, u := range b.Units {
			if u.ID == id {
				return u, true
			}
		}
	}
	return Unit{}, false
}

type Option struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}
type Question struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	Prompt        string   `json:"prompt"`
	InstructionTH string   `json:"instruction_th"`
	Options       []Option `json:"options"`
	Answers       []string `json:"answers,omitempty"`
	ExplanationTH string   `json:"explanation_th,omitempty"`
	Open          bool     `json:"open"`
	Example       bool     `json:"example"`
}
type Word struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
	Example string `json:"example"`
}
type Pack struct {
	ExplanationTH   string     `json:"explanation_th"`
	Questions       []Question `json:"questions"`
	Vocabulary      []Word     `json:"vocabulary"`
	Pattern         string     `json:"pattern"`
	SpeakingPrompt  string     `json:"speaking_prompt"`
	SpeakingTH      string     `json:"speaking_th"`
	ListeningPrompt string     `json:"listening_prompt"`
	ListeningTH     string     `json:"listening_th"`
}

func (p Pack) Public() Pack {
	q := append([]Question(nil), p.Questions...)
	for i := range q {
		if !q[i].Example {
			q[i].Answers = nil
		}
		q[i].ExplanationTH = ""
	}
	p.Questions = q
	return p
}
func (p Pack) Validate(u Unit) error {
	expected := map[string]bool{}
	for _, s := range u.Sections {
		for _, id := range s.ItemIDs {
			expected[id] = true
		}
	}
	if p.ExplanationTH == "" || p.Pattern == "" || p.SpeakingPrompt == "" || p.ListeningPrompt == "" || p.ListeningTH == "" || p.SpeakingTH == "" || len(p.Vocabulary) < 1 {
		return fmt.Errorf("ebook teaching content incomplete")
	}
	for _, q := range p.Questions {
		if !expected[q.ID] || strings.TrimSpace(q.Prompt) == "" || len(q.Answers) == 0 {
			return fmt.Errorf("ebook question missing or duplicated: %s", q.ID)
		}
		delete(expected, q.ID)
		if q.Kind != "write" && q.Kind != "match" && q.Kind != "choice" {
			return fmt.Errorf("invalid ebook question kind")
		}
		if q.Kind != "write" && len(q.Options) < 2 {
			return fmt.Errorf("missing ebook choices")
		}
	}
	if len(expected) > 0 {
		return fmt.Errorf("ebook omitted original questions: %v", expected)
	}
	return nil
}

// ValidatePages prevents switching a deployment to an incomplete private book image.
func (b *Book) ValidatePages(dir string) error {
	if b == nil {
		return fmt.Errorf("ebook manifest unavailable")
	}
	for page := 1; page <= b.PageCount; page++ {
		info, err := os.Stat(filepath.Join(dir, fmt.Sprintf("page-%03d.jpg", page)))
		if err != nil || !info.Mode().IsRegular() || info.Size() == 0 {
			return fmt.Errorf("ebook page %d unavailable", page)
		}
	}
	return nil
}

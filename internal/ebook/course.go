package ebook

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
)

// learnEbookJSON is authored course content. It intentionally contains no
// extracted reference-book prose, exercise text, answer key, or page image.
//
//go:embed learn_ebook_v1.json
var learnEbookJSON []byte

const (
	LearnEbookCourseID      = "learn-ebook"
	LearnEbookCourseVersion = "2026-09-20.v1"
	LearnEbookLessonCount   = 145
)

type CourseReference struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	UnitCount int    `json:"unit_count"`
}

type Course struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Version   string           `json:"version"`
	Reference *CourseReference `json:"reference,omitempty"`
	Lessons   []Lesson         `json:"lessons"`
}

func (c *Course) Lesson(id string) (Lesson, bool) {
	if c == nil {
		return Lesson{}, false
	}
	for _, lesson := range c.Lessons {
		if id == lesson.ID || id == lesson.UnitID || id == fmt.Sprintf("%d", lesson.Ordinal) {
			return lesson, true
		}
	}
	return Lesson{}, false
}

type Example struct {
	EN string `json:"en"`
	TH string `json:"th"`
}

type Vocabulary struct {
	ID        string `json:"id"`
	Term      string `json:"term"`
	MeaningTH string `json:"meaning_th"`
	ExampleEN string `json:"example_en"`
	ExampleTH string `json:"example_th"`
}

type QuizItem struct {
	ID            string   `json:"id"`
	Kind          string   `json:"kind"`
	PromptEN      string   `json:"prompt_en"`
	PromptTH      string   `json:"prompt_th"`
	Options       []string `json:"options,omitempty"`
	Answers       []string `json:"answers"`
	ExplanationTH string   `json:"explanation_th"`
}

type PracticeTask struct {
	PromptEN          string   `json:"prompt_en"`
	PromptTH          string   `json:"prompt_th"`
	TargetVocabulary  []string `json:"target_vocabulary"`
	SuccessCriteriaTH []string `json:"success_criteria_th"`
}

type Lesson struct {
	ID              string       `json:"id"`
	UnitID          string       `json:"unit_id"`
	Ordinal         int          `json:"ordinal"`
	Title           string       `json:"title"`
	ContextKeywords []string     `json:"context_keywords,omitempty"`
	GoalTH          string       `json:"goal_th"`
	ExplanationTH   string       `json:"explanation_th"`
	Pattern         string       `json:"pattern"`
	Examples        []Example    `json:"examples"`
	Vocabulary      []Vocabulary `json:"vocabulary"`
	Quiz            []QuizItem   `json:"quiz"`
	Shadowing       []string     `json:"shadowing"`
	Speaking        PracticeTask `json:"speaking"`
	Listening       PracticeTask `json:"listening"`
	ConceptTags     []string     `json:"concept_tags"`
}

type Audit struct {
	Lessons        int `json:"lessons"`
	Vocabulary     int `json:"vocabulary"`
	QuizItems      int `json:"quiz_items"`
	ShadowingLines int `json:"shadowing_lines"`
}

func (a Audit) String() string {
	return fmt.Sprintf("lessons=%d vocabulary=%d quiz_items=%d shadowing_lines=%d", a.Lessons, a.Vocabulary, a.QuizItems, a.ShadowingLines)
}

// LoadCourse loads the embedded, versioned course and validates it before the
// application opens its database. This keeps malformed content out of a live
// process and makes content deployment deterministic.
func LoadCourse() (*Course, error) {
	return LoadCourseBytes(learnEbookJSON)
}

func LoadCourseBytes(data []byte) (*Course, error) {
	var course Course
	if err := json.Unmarshal(data, &course); err != nil {
		return nil, fmt.Errorf("decode Learn Ebook course: %w", err)
	}
	if err := course.Validate(); err != nil {
		return nil, err
	}
	return &course, nil
}

func (c Course) Audit() Audit {
	a := Audit{Lessons: len(c.Lessons)}
	for _, lesson := range c.Lessons {
		a.Vocabulary += len(lesson.Vocabulary)
		a.QuizItems += len(lesson.Quiz)
		a.ShadowingLines += len(lesson.Shadowing)
	}
	return a
}

func (c Course) Validate() error {
	if c.ID != LearnEbookCourseID {
		return fmt.Errorf("invalid Learn Ebook course id %q", c.ID)
	}
	if c.Version != LearnEbookCourseVersion {
		return fmt.Errorf("invalid Learn Ebook course version %q", c.Version)
	}
	if strings.TrimSpace(c.Title) == "" {
		return fmt.Errorf("Learn Ebook course title is empty")
	}
	if len(c.Lessons) != LearnEbookLessonCount {
		return fmt.Errorf("Learn Ebook course has %d lessons, want %d", len(c.Lessons), LearnEbookLessonCount)
	}
	if c.Reference != nil && c.Reference.UnitCount != 0 && c.Reference.UnitCount != LearnEbookLessonCount {
		return fmt.Errorf("Learn Ebook reference has %d units, want %d", c.Reference.UnitCount, LearnEbookLessonCount)
	}

	seenIDs := map[string]bool{}
	seenUnits := map[string]bool{}
	seenPatterns := map[string]bool{}
	seenExamples := map[string]int{}
	for i, lesson := range c.Lessons {
		if err := lesson.validate(i+1, seenIDs, seenUnits, seenPatterns); err != nil {
			return err
		}
		for _, example := range lesson.Examples {
			key := normalizeCourseText(example.EN)
			seenExamples[key]++
			if seenExamples[key] > 2 {
				return fmt.Errorf("Learn Ebook repeats the same example more than twice: %q", example.EN)
			}
		}
	}
	got := c.Audit()
	if got != (Audit{Lessons: 145, Vocabulary: 1450, QuizItems: 725, ShadowingLines: 435}) {
		return fmt.Errorf("Learn Ebook audit totals are %s, want lessons=145 vocabulary=1450 quiz_items=725 shadowing_lines=435", got.String())
	}
	return nil
}

func (l Lesson) validate(expectedOrdinal int, seenIDs, seenUnits, seenPatterns map[string]bool) error {
	if l.Ordinal != expectedOrdinal || l.ID != fmt.Sprintf("ebook-%03d", expectedOrdinal) || l.UnitID != fmt.Sprintf("%d", expectedOrdinal) {
		return fmt.Errorf("lesson %d has unstable id/unit/ordinal: id=%q unit_id=%q ordinal=%d", expectedOrdinal, l.ID, l.UnitID, l.Ordinal)
	}
	if seenIDs[l.ID] || seenUnits[l.UnitID] {
		return fmt.Errorf("duplicate Learn Ebook lesson identity: %s/%s", l.ID, l.UnitID)
	}
	seenIDs[l.ID] = true
	seenUnits[l.UnitID] = true
	if strings.TrimSpace(l.Title) == "" || strings.TrimSpace(l.GoalTH) == "" || strings.TrimSpace(l.ExplanationTH) == "" || strings.TrimSpace(l.Pattern) == "" {
		return fmt.Errorf("lesson %s has incomplete concept content", l.ID)
	}
	if len(l.ContextKeywords) < 2 {
		return fmt.Errorf("lesson %s needs at least two context keywords", l.ID)
	}
	seenContext := map[string]bool{}
	for _, keyword := range l.ContextKeywords {
		key := normalizeCourseText(keyword)
		if key == "" || seenContext[key] {
			return fmt.Errorf("lesson %s has duplicate or empty context keyword", l.ID)
		}
		seenContext[key] = true
	}
	lessonEvidence := normalizeCourseText(l.GoalTH + " " + l.Pattern + " " + strings.Join(l.ConceptTags, " "))
	if !strings.Contains(lessonEvidence, normalizeCourseText(l.Title)) {
		return fmt.Errorf("lesson %s does not mention its grammar title in goal, pattern, or tags", l.ID)
	}
	patternKey := normalizeCourseText(l.Pattern)
	if seenPatterns[patternKey] {
		return fmt.Errorf("lesson %s repeats a pattern", l.ID)
	}
	seenPatterns[patternKey] = true

	if len(l.Examples) != 3 {
		return fmt.Errorf("lesson %s has %d examples, want 3", l.ID, len(l.Examples))
	}
	seenExamples := map[string]bool{}
	for _, example := range l.Examples {
		if strings.TrimSpace(example.EN) == "" || strings.TrimSpace(example.TH) == "" {
			return fmt.Errorf("lesson %s has an incomplete example", l.ID)
		}
		key := normalizeCourseText(example.EN)
		if seenExamples[key] {
			return fmt.Errorf("lesson %s repeats an example", l.ID)
		}
		seenExamples[key] = true
	}
	contextText := ""
	for _, example := range l.Examples {
		contextText += " " + normalizeCourseText(example.EN)
	}
	contextHits := 0
	for _, keyword := range l.ContextKeywords {
		if strings.Contains(contextText, normalizeCourseText(keyword)) {
			contextHits++
		}
	}
	if contextHits < 2 {
		return fmt.Errorf("lesson %s examples do not contain two context keywords", l.ID)
	}

	if len(l.Vocabulary) != 10 {
		return fmt.Errorf("lesson %s has %d vocabulary items, want 10", l.ID, len(l.Vocabulary))
	}
	seenWords := map[string]bool{}
	wordIDs := map[string]bool{}
	for _, word := range l.Vocabulary {
		if strings.TrimSpace(word.ID) == "" || strings.TrimSpace(word.Term) == "" || strings.TrimSpace(word.MeaningTH) == "" || strings.TrimSpace(word.ExampleEN) == "" || strings.TrimSpace(word.ExampleTH) == "" {
			return fmt.Errorf("lesson %s has incomplete vocabulary", l.ID)
		}
		if wordIDs[word.ID] {
			return fmt.Errorf("lesson %s repeats vocabulary id %q", l.ID, word.ID)
		}
		wordIDs[word.ID] = true
		key := normalizeCourseText(word.Term)
		if seenWords[key] {
			return fmt.Errorf("lesson %s repeats vocabulary term %q", l.ID, word.Term)
		}
		seenWords[key] = true
	}

	if len(l.Quiz) != 5 {
		return fmt.Errorf("lesson %s has %d quiz items, want 5", l.ID, len(l.Quiz))
	}
	seenQuiz := map[string]bool{}
	for i, q := range l.Quiz {
		wantID := fmt.Sprintf("quiz-%02d", i+1)
		if q.ID != wantID || seenQuiz[q.ID] {
			return fmt.Errorf("lesson %s has unstable quiz id %q, want %q", l.ID, q.ID, wantID)
		}
		seenQuiz[q.ID] = true
		if q.Kind != "choice" && q.Kind != "write" {
			return fmt.Errorf("lesson %s quiz %s has invalid kind %q", l.ID, q.ID, q.Kind)
		}
		if strings.TrimSpace(q.PromptEN) == "" || strings.TrimSpace(q.PromptTH) == "" || len(q.Answers) == 0 || strings.TrimSpace(q.ExplanationTH) == "" {
			return fmt.Errorf("lesson %s quiz %s is incomplete", l.ID, q.ID)
		}
		if q.Kind == "choice" {
			if len(q.Options) < 2 {
				return fmt.Errorf("lesson %s quiz %s needs at least two options", l.ID, q.ID)
			}
			options := map[string]bool{}
			for _, option := range q.Options {
				if strings.TrimSpace(option) == "" || options[normalizeCourseText(option)] {
					return fmt.Errorf("lesson %s quiz %s has invalid options", l.ID, q.ID)
				}
				options[normalizeCourseText(option)] = true
			}
			for _, answer := range q.Answers {
				if !options[normalizeCourseText(answer)] {
					return fmt.Errorf("lesson %s quiz %s answer is not an option", l.ID, q.ID)
				}
			}
		}
	}

	if len(l.Shadowing) != 3 {
		return fmt.Errorf("lesson %s has %d shadowing lines, want 3", l.ID, len(l.Shadowing))
	}
	seenShadowing := map[string]bool{}
	for _, line := range l.Shadowing {
		if strings.TrimSpace(line) == "" || seenShadowing[line] {
			return fmt.Errorf("lesson %s has duplicate or empty shadowing line", l.ID)
		}
		seenShadowing[line] = true
		found := false
		for _, example := range l.Examples {
			if line == example.EN {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("lesson %s shadowing line is not verbatim from an example", l.ID)
		}
	}

	if err := validatePracticeTask(l.ID, "speaking", l.Speaking, wordIDs); err != nil {
		return err
	}
	if err := validatePracticeTask(l.ID, "listening", l.Listening, wordIDs); err != nil {
		return err
	}
	if len(l.ConceptTags) == 0 {
		return fmt.Errorf("lesson %s has no concept tags", l.ID)
	}
	if err := validateLessonTextQuality(l); err != nil {
		return err
	}
	return nil
}

func validateLessonTextQuality(l Lesson) error {
	banned := []string{
		"todo", "placeholder", "word1", "vocab-1", "example sentence", "fixture",
		"the lesson form", "an unrelated form", "wrong time", "moonlight", "is ready for",
		"we discussed the",
		"discussed the health", "discussed the participant", "finished the airport map",
		"the the", "some the",
	}
	texts := []string{l.Title, l.GoalTH, l.ExplanationTH, l.Pattern}
	texts = append(texts, l.ContextKeywords...)
	texts = append(texts, l.ConceptTags...)
	for _, example := range l.Examples {
		texts = append(texts, example.EN, example.TH)
	}
	for _, word := range l.Vocabulary {
		texts = append(texts, word.Term, word.MeaningTH, word.ExampleEN, word.ExampleTH)
	}
	for _, quiz := range l.Quiz {
		texts = append(texts, quiz.PromptEN, quiz.PromptTH, quiz.ExplanationTH)
		texts = append(texts, quiz.Options...)
		texts = append(texts, quiz.Answers...)
	}
	texts = append(texts, l.Shadowing...)
	texts = append(texts, l.Speaking.PromptEN, l.Speaking.PromptTH)
	texts = append(texts, l.Speaking.SuccessCriteriaTH...)
	texts = append(texts, l.Listening.PromptEN, l.Listening.PromptTH)
	texts = append(texts, l.Listening.SuccessCriteriaTH...)
	for _, text := range texts {
		lower := normalizeCourseText(text)
		for _, token := range banned {
			if strings.Contains(lower, token) {
				return fmt.Errorf("lesson %s contains banned placeholder or awkward template %q", l.ID, token)
			}
		}
		fields := strings.Fields(lower)
		for i := 0; i+2 < len(fields); i++ {
			if fields[i] == "a" && fields[i+1] == "short" && (fields[i+2] == "a" || fields[i+2] == "an" || fields[i+2] == "the") {
				return fmt.Errorf("lesson %s contains doubled article in listening template", l.ID)
			}
		}
	}
	return nil
}

func validatePracticeTask(lessonID, name string, task PracticeTask, wordIDs map[string]bool) error {
	if strings.TrimSpace(task.PromptEN) == "" || strings.TrimSpace(task.PromptTH) == "" || len(task.TargetVocabulary) == 0 || len(task.SuccessCriteriaTH) < 2 {
		return fmt.Errorf("lesson %s %s task is incomplete", lessonID, name)
	}
	seen := map[string]bool{}
	for _, wordID := range task.TargetVocabulary {
		if !wordIDs[wordID] || seen[wordID] {
			return fmt.Errorf("lesson %s %s task references invalid vocabulary %q", lessonID, name, wordID)
		}
		seen[wordID] = true
	}
	return nil
}

func normalizeCourseText(s string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(s)), " "))
}

// Public returns a deep-enough copy for the API. Accepted answers and answer
// explanations are private assessment data and are always removed.
func (c Course) Public() Course {
	out := c
	if c.Reference != nil {
		reference := *c.Reference
		out.Reference = &reference
	}
	out.Lessons = make([]Lesson, len(c.Lessons))
	for i, lesson := range c.Lessons {
		out.Lessons[i] = lesson.Public()
	}
	return out
}

func (l Lesson) Public() Lesson {
	out := l
	out.ContextKeywords = append([]string(nil), l.ContextKeywords...)
	out.Quiz = make([]QuizItem, len(l.Quiz))
	for i, quiz := range l.Quiz {
		out.Quiz[i] = quiz
		out.Quiz[i].Answers = nil
		out.Quiz[i].ExplanationTH = ""
		if quiz.Options != nil {
			out.Quiz[i].Options = append([]string(nil), quiz.Options...)
		}
	}
	out.Examples = append([]Example(nil), l.Examples...)
	out.Vocabulary = append([]Vocabulary(nil), l.Vocabulary...)
	out.Shadowing = append([]string(nil), l.Shadowing...)
	out.ConceptTags = append([]string(nil), l.ConceptTags...)
	out.Speaking.TargetVocabulary = append([]string(nil), l.Speaking.TargetVocabulary...)
	out.Speaking.SuccessCriteriaTH = append([]string(nil), l.Speaking.SuccessCriteriaTH...)
	out.Listening.TargetVocabulary = append([]string(nil), l.Listening.TargetVocabulary...)
	out.Listening.SuccessCriteriaTH = append([]string(nil), l.Listening.SuccessCriteriaTH...)
	return out
}

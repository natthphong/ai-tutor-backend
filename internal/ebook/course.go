package ebook

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// learnEbookJSON is authored course content. It intentionally contains no
// extracted reference-book prose, exercise text, answer key, or page image.
//
//go:embed learn_ebook_v2.json
var learnEbookJSON []byte

const (
	LearnEbookCourseID      = "learn-ebook"
	LearnEbookCourseVersion = "2026-10-10.v2"
	LearnEbookLessonCount   = 145
)

// LearnEbookPreviousCourseVersions lists earlier bundled course versions whose
// learner progress is carried into the current version. Lesson IDs are stable
// across versions; quiz content is not.
var LearnEbookPreviousCourseVersions = []string{"2026-09-20.v1"}

// LearnEbookLevels is the recommended CEFR order of the course path.
var LearnEbookLevels = []string{"A1", "A2", "B1", "B2"}

// LearnEbookQuizKinds is the fixed order of quiz item kinds in every lesson.
// choice and reorder are checked deterministically, fill against accepted
// fillers, and fix/translate/write fall back to the tutor model only when the
// answer is not one of the accepted answers.
var LearnEbookQuizKinds = []string{"choice", "choice", "fill", "fill", "reorder", "fix", "translate", "write"}

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

// Path returns the lessons in the recommended learning order (A1 first).
func (c *Course) Path() []Lesson {
	if c == nil {
		return nil
	}
	path := append([]Lesson(nil), c.Lessons...)
	sort.SliceStable(path, func(i, j int) bool { return path[i].PathOrder < path[j].PathOrder })
	return path
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
	TargetVocabulary  []string `json:"target_vocabulary,omitempty"`
	SuccessCriteriaTH []string `json:"success_criteria_th"`
}

type Form struct {
	LabelTH   string `json:"label_th"`
	Pattern   string `json:"pattern"`
	ExampleEN string `json:"example_en"`
	ExampleTH string `json:"example_th"`
}

type Mistake struct {
	Wrong string `json:"wrong"`
	Right string `json:"right"`
	WhyTH string `json:"why_th"`
}

// ListeningItem is heard before it is read: the learner answers a Thai
// comprehension question about audio_en, then shadows the same line.
type ListeningItem struct {
	ID         string   `json:"id"`
	AudioEN    string   `json:"audio_en"`
	MeaningTH  string   `json:"meaning_th,omitempty"`
	QuestionTH string   `json:"question_th"`
	OptionsTH  []string `json:"options_th"`
	AnswerTH   string   `json:"answer_th,omitempty"`
}

// ReviewPrompt is a new production task scheduled after the unit is learned.
type ReviewPrompt struct {
	ID       string `json:"id"`
	PromptTH string `json:"prompt_th"`
	TargetEN string `json:"target_en"`
	TargetTH string `json:"target_th"`
}

type Lesson struct {
	ID             string          `json:"id"`
	UnitID         string          `json:"unit_id"`
	Ordinal        int             `json:"ordinal"`
	PathOrder      int             `json:"path_order"`
	Level          string          `json:"level"`
	Title          string          `json:"title"`
	TitleTH        string          `json:"title_th"`
	ThemeEN        string          `json:"theme_en"`
	ThemeTH        string          `json:"theme_th"`
	Domain         string          `json:"domain"`
	GoalTH         string          `json:"goal_th"`
	ExplanationTH  string          `json:"explanation_th"`
	Pattern        string          `json:"pattern"`
	UsageTH        []string        `json:"usage_th"`
	Forms          []Form          `json:"forms"`
	SignalWords    []string        `json:"signal_words"`
	CommonMistakes []Mistake       `json:"common_mistakes"`
	CompareTH      string          `json:"compare_th"`
	Examples       []Example       `json:"examples"`
	Vocabulary     []Vocabulary    `json:"vocabulary"`
	Quiz           []QuizItem      `json:"quiz"`
	Listening      []ListeningItem `json:"listening_items"`
	Shadowing      []string        `json:"shadowing"`
	Speaking       PracticeTask    `json:"speaking"`
	Review         []ReviewPrompt  `json:"review,omitempty"`
	ConceptTags    []string        `json:"concept_tags"`
}

type Audit struct {
	Lessons         int `json:"lessons"`
	Examples        int `json:"examples"`
	Vocabulary      int `json:"vocabulary"`
	UniqueTerms     int `json:"unique_terms"`
	QuizItems       int `json:"quiz_items"`
	ListeningItems  int `json:"listening_items"`
	ShadowingLines  int `json:"shadowing_lines"`
	ReviewPrompts   int `json:"review_prompts"`
	UniqueTeachings int `json:"unique_explanations"`
}

func (a Audit) String() string {
	return fmt.Sprintf("lessons=%d examples=%d vocabulary=%d unique_terms=%d quiz_items=%d listening_items=%d shadowing_lines=%d review_prompts=%d unique_explanations=%d",
		a.Lessons, a.Examples, a.Vocabulary, a.UniqueTerms, a.QuizItems, a.ListeningItems, a.ShadowingLines, a.ReviewPrompts, a.UniqueTeachings)
}

// MinUniqueTerms is the floor for globally distinct vocabulary terms. The
// first course version repeated 136 terms across 1,450 entries.
const MinUniqueTerms = 1400

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
	terms := map[string]bool{}
	teachings := map[string]bool{}
	for _, lesson := range c.Lessons {
		a.Examples += len(lesson.Examples)
		a.Vocabulary += len(lesson.Vocabulary)
		a.QuizItems += len(lesson.Quiz)
		a.ListeningItems += len(lesson.Listening)
		a.ShadowingLines += len(lesson.Shadowing)
		a.ReviewPrompts += len(lesson.Review)
		for _, word := range lesson.Vocabulary {
			terms[normalizeCourseText(word.Term)] = true
		}
		teachings[normalizeCourseText(lesson.ExplanationTH)] = true
	}
	a.UniqueTerms = len(terms)
	a.UniqueTeachings = len(teachings)
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
	seenPath := map[int]bool{}
	seenExamples := map[string]string{}
	levelRank := map[string]int{}
	for index, level := range LearnEbookLevels {
		levelRank[level] = index
	}
	for i, lesson := range c.Lessons {
		if err := lesson.validate(i+1, seenIDs, seenUnits); err != nil {
			return err
		}
		if lesson.PathOrder < 1 || lesson.PathOrder > LearnEbookLessonCount || seenPath[lesson.PathOrder] {
			return fmt.Errorf("lesson %s has an invalid or repeated path_order %d", lesson.ID, lesson.PathOrder)
		}
		seenPath[lesson.PathOrder] = true
		for _, example := range lesson.Examples {
			key := normalizeCourseText(example.EN)
			if other, repeated := seenExamples[key]; repeated {
				return fmt.Errorf("Learn Ebook repeats an example in %s and %s: %q", other, lesson.ID, example.EN)
			}
			seenExamples[key] = lesson.ID
		}
	}
	// The path never steps down a level, so a learner who starts at A1 meets
	// every easier unit before a harder one.
	previous := 0
	for _, lesson := range c.Path() {
		rank := levelRank[lesson.Level]
		if rank < previous {
			return fmt.Errorf("lesson %s (%s) appears after a higher level in the learning path", lesson.ID, lesson.Level)
		}
		previous = rank
	}
	got := c.Audit()
	want := Audit{Lessons: 145, Examples: 870, Vocabulary: 1450, QuizItems: 1160, ListeningItems: 435, ShadowingLines: 435, ReviewPrompts: 290, UniqueTeachings: 145}
	want.UniqueTerms = got.UniqueTerms
	if got != want {
		return fmt.Errorf("Learn Ebook audit totals are %s, want %s", got.String(), want.String())
	}
	if got.UniqueTerms < MinUniqueTerms {
		return fmt.Errorf("Learn Ebook has %d globally unique vocabulary terms, want at least %d", got.UniqueTerms, MinUniqueTerms)
	}
	return nil
}

func hasThai(text string) bool {
	for _, r := range text {
		if r >= 0x0E00 && r <= 0x0E7F {
			return true
		}
	}
	return false
}

func (l Lesson) validate(expectedOrdinal int, seenIDs, seenUnits map[string]bool) error {
	if l.Ordinal != expectedOrdinal || l.ID != fmt.Sprintf("ebook-%03d", expectedOrdinal) || l.UnitID != fmt.Sprintf("%d", expectedOrdinal) {
		return fmt.Errorf("lesson %d has unstable id/unit/ordinal: id=%q unit_id=%q ordinal=%d", expectedOrdinal, l.ID, l.UnitID, l.Ordinal)
	}
	if seenIDs[l.ID] || seenUnits[l.UnitID] {
		return fmt.Errorf("duplicate Learn Ebook lesson identity: %s/%s", l.ID, l.UnitID)
	}
	seenIDs[l.ID] = true
	seenUnits[l.UnitID] = true
	validLevel := false
	for _, level := range LearnEbookLevels {
		validLevel = validLevel || l.Level == level
	}
	if !validLevel {
		return fmt.Errorf("lesson %s has invalid level %q", l.ID, l.Level)
	}
	if strings.TrimSpace(l.Title) == "" || strings.TrimSpace(l.Pattern) == "" || strings.TrimSpace(l.ThemeEN) == "" {
		return fmt.Errorf("lesson %s has incomplete concept content", l.ID)
	}
	if !hasThai(l.TitleTH) || !hasThai(l.GoalTH) || !hasThai(l.ExplanationTH) || !hasThai(l.ThemeTH) {
		return fmt.Errorf("lesson %s is missing Thai teaching text", l.ID)
	}
	if len([]rune(l.ExplanationTH)) < 200 {
		return fmt.Errorf("lesson %s explanation is too short to teach the grammar point", l.ID)
	}
	if len(l.UsageTH) < 2 || len(l.UsageTH) > 4 {
		return fmt.Errorf("lesson %s has %d usage notes, want 2-4", l.ID, len(l.UsageTH))
	}
	if len(l.Forms) < 2 || len(l.Forms) > 4 {
		return fmt.Errorf("lesson %s has %d forms, want 2-4", l.ID, len(l.Forms))
	}
	for _, form := range l.Forms {
		if strings.TrimSpace(form.LabelTH) == "" || strings.TrimSpace(form.Pattern) == "" || strings.TrimSpace(form.ExampleEN) == "" || !hasThai(form.ExampleTH) {
			return fmt.Errorf("lesson %s has an incomplete form", l.ID)
		}
	}
	if len(l.CommonMistakes) < 2 || len(l.CommonMistakes) > 3 {
		return fmt.Errorf("lesson %s has %d common mistakes, want 2-3", l.ID, len(l.CommonMistakes))
	}
	for _, mistake := range l.CommonMistakes {
		if strings.TrimSpace(mistake.Wrong) == "" || normalizeCourseText(mistake.Wrong) == normalizeCourseText(mistake.Right) || !hasThai(mistake.WhyTH) {
			return fmt.Errorf("lesson %s has an invalid common mistake", l.ID)
		}
	}

	if len(l.Examples) != 6 {
		return fmt.Errorf("lesson %s has %d examples, want 6", l.ID, len(l.Examples))
	}
	seenExamples := map[string]bool{}
	for _, example := range l.Examples {
		if strings.TrimSpace(example.EN) == "" || hasThai(example.EN) || !hasThai(example.TH) {
			return fmt.Errorf("lesson %s has an incomplete example", l.ID)
		}
		key := normalizeCourseText(example.EN)
		if seenExamples[key] {
			return fmt.Errorf("lesson %s repeats an example", l.ID)
		}
		seenExamples[key] = true
	}

	if len(l.Vocabulary) != 10 {
		return fmt.Errorf("lesson %s has %d vocabulary items, want 10", l.ID, len(l.Vocabulary))
	}
	seenWords := map[string]bool{}
	wordIDs := map[string]bool{}
	for _, word := range l.Vocabulary {
		if strings.TrimSpace(word.ID) == "" || strings.TrimSpace(word.Term) == "" || !hasThai(word.MeaningTH) || strings.TrimSpace(word.ExampleEN) == "" || !hasThai(word.ExampleTH) {
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

	if len(l.Quiz) != len(LearnEbookQuizKinds) {
		return fmt.Errorf("lesson %s has %d quiz items, want %d", l.ID, len(l.Quiz), len(LearnEbookQuizKinds))
	}
	for i, q := range l.Quiz {
		wantID := fmt.Sprintf("quiz-%02d", i+1)
		if q.ID != wantID {
			return fmt.Errorf("lesson %s has unstable quiz id %q, want %q", l.ID, q.ID, wantID)
		}
		if q.Kind != LearnEbookQuizKinds[i] {
			return fmt.Errorf("lesson %s quiz %s has kind %q, want %q", l.ID, q.ID, q.Kind, LearnEbookQuizKinds[i])
		}
		if strings.TrimSpace(q.PromptEN) == "" || !hasThai(q.PromptTH) || len(q.Answers) == 0 || !hasThai(q.ExplanationTH) {
			return fmt.Errorf("lesson %s quiz %s is incomplete", l.ID, q.ID)
		}
		for _, answer := range q.Answers {
			if strings.TrimSpace(answer) == "" {
				return fmt.Errorf("lesson %s quiz %s has an empty accepted answer", l.ID, q.ID)
			}
		}
		switch q.Kind {
		case "choice":
			if strings.Count(q.PromptEN, "___") != 1 {
				return fmt.Errorf("lesson %s quiz %s needs exactly one blank", l.ID, q.ID)
			}
			if len(q.Options) != 3 {
				return fmt.Errorf("lesson %s quiz %s needs three options", l.ID, q.ID)
			}
			options := map[string]bool{}
			for _, option := range q.Options {
				if strings.TrimSpace(option) == "" || options[normalizeCourseText(option)] {
					return fmt.Errorf("lesson %s quiz %s has invalid options", l.ID, q.ID)
				}
				options[normalizeCourseText(option)] = true
			}
			if len(q.Answers) != 1 || !options[normalizeCourseText(q.Answers[0])] {
				return fmt.Errorf("lesson %s quiz %s answer is not exactly one option", l.ID, q.ID)
			}
		case "fill":
			if strings.Count(q.PromptEN, "___") != 1 || len(q.Options) != 0 {
				return fmt.Errorf("lesson %s quiz %s needs exactly one blank and no options", l.ID, q.ID)
			}
		case "reorder":
			want := strings.Fields(q.Answers[0])
			got := append([]string(nil), q.Options...)
			if len(want) < 4 || len(got) != len(want) {
				return fmt.Errorf("lesson %s quiz %s tokens do not rebuild its answer", l.ID, q.ID)
			}
			if strings.Join(got, " ") == strings.Join(want, " ") {
				return fmt.Errorf("lesson %s quiz %s tokens are not shuffled", l.ID, q.ID)
			}
			sort.Strings(want)
			sort.Strings(got)
			if strings.Join(got, " ") != strings.Join(want, " ") {
				return fmt.Errorf("lesson %s quiz %s tokens do not rebuild its answer", l.ID, q.ID)
			}
		case "fix":
			for _, answer := range q.Answers {
				if normalizeCourseText(answer) == normalizeCourseText(q.PromptEN) {
					return fmt.Errorf("lesson %s quiz %s shows a sentence without a mistake", l.ID, q.ID)
				}
			}
			fallthrough
		default:
			if len(q.Options) != 0 {
				return fmt.Errorf("lesson %s quiz %s must not have options", l.ID, q.ID)
			}
		}
	}

	if len(l.Listening) != 3 {
		return fmt.Errorf("lesson %s has %d listening items, want 3", l.ID, len(l.Listening))
	}
	for i, item := range l.Listening {
		if item.ID != fmt.Sprintf("listen-%02d", i+1) {
			return fmt.Errorf("lesson %s has unstable listening id %q", l.ID, item.ID)
		}
		if strings.TrimSpace(item.AudioEN) == "" || hasThai(item.AudioEN) || !hasThai(item.MeaningTH) || !hasThai(item.QuestionTH) || len(item.OptionsTH) != 3 {
			return fmt.Errorf("lesson %s listening %s is incomplete", l.ID, item.ID)
		}
		if seenExamples[normalizeCourseText(item.AudioEN)] {
			return fmt.Errorf("lesson %s listening %s repeats a lesson example; listening must be heard first", l.ID, item.ID)
		}
		found := 0
		options := map[string]bool{}
		for _, option := range item.OptionsTH {
			if strings.TrimSpace(option) == "" || options[option] {
				return fmt.Errorf("lesson %s listening %s has invalid options", l.ID, item.ID)
			}
			options[option] = true
			if option == item.AnswerTH {
				found++
			}
		}
		if found != 1 {
			return fmt.Errorf("lesson %s listening %s answer is not exactly one option", l.ID, item.ID)
		}
	}

	if len(l.Shadowing) != 3 {
		return fmt.Errorf("lesson %s has %d shadowing lines, want 3", l.ID, len(l.Shadowing))
	}
	for i, line := range l.Shadowing {
		if line != l.Listening[i].AudioEN {
			return fmt.Errorf("lesson %s shadowing line %d is not the listening line the learner just heard", l.ID, i+1)
		}
	}

	if strings.TrimSpace(l.Speaking.PromptEN) == "" || !hasThai(l.Speaking.PromptTH) || len(l.Speaking.SuccessCriteriaTH) < 2 {
		return fmt.Errorf("lesson %s speaking task is incomplete", l.ID)
	}
	for _, wordID := range l.Speaking.TargetVocabulary {
		if !wordIDs[wordID] {
			return fmt.Errorf("lesson %s speaking task references invalid vocabulary %q", l.ID, wordID)
		}
	}
	if len(l.Review) != 2 {
		return fmt.Errorf("lesson %s has %d review prompts, want 2", l.ID, len(l.Review))
	}
	for i, prompt := range l.Review {
		if prompt.ID != fmt.Sprintf("review-%02d", i+1) || !hasThai(prompt.PromptTH) || strings.TrimSpace(prompt.TargetEN) == "" || hasThai(prompt.TargetEN) || !hasThai(prompt.TargetTH) {
			return fmt.Errorf("lesson %s review prompt %d is incomplete", l.ID, i+1)
		}
		if seenExamples[normalizeCourseText(prompt.TargetEN)] {
			return fmt.Errorf("lesson %s review prompt %d repeats a lesson example", l.ID, i+1)
		}
	}
	if len(l.ConceptTags) == 0 {
		return fmt.Errorf("lesson %s has no concept tags", l.ID)
	}
	return validateLessonTextQuality(l)
}

func validateLessonTextQuality(l Lesson) error {
	banned := []string{"todo", "placeholder", "lorem ipsum", "word1", "vocab-1"}
	doubled := map[string]bool{"the": true, "a": true, "an": true, "to": true, "of": true, "in": true, "is": true, "are": true, "and": true}
	texts := []string{l.Title, l.Pattern, l.ThemeEN}
	for _, example := range l.Examples {
		texts = append(texts, example.EN)
	}
	for _, word := range l.Vocabulary {
		texts = append(texts, word.Term, word.ExampleEN)
	}
	for _, quiz := range l.Quiz {
		texts = append(texts, quiz.Answers...)
	}
	for _, item := range l.Listening {
		texts = append(texts, item.AudioEN)
	}
	for _, prompt := range l.Review {
		texts = append(texts, prompt.TargetEN)
	}
	texts = append(texts, l.Speaking.PromptEN)
	for _, text := range texts {
		lower := normalizeCourseText(text)
		for _, token := range banned {
			if strings.Contains(lower, token) {
				return fmt.Errorf("lesson %s contains placeholder text %q in %q", l.ID, token, text)
			}
		}
		// A repeated function word ("the the", "to to") is a writing slip.
		words := strings.Fields(lower)
		for i := 1; i < len(words); i++ {
			word := strings.Trim(words[i], ".,?!;:\"'")
			if doubled[word] && word == strings.Trim(words[i-1], ".,?!;:\"'") && words[i-1] == strings.Trim(words[i-1], ".,?!;:") {
				return fmt.Errorf("lesson %s repeats %q in %q", l.ID, word, text)
			}
		}
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
	out.Quiz = make([]QuizItem, len(l.Quiz))
	for i, quiz := range l.Quiz {
		out.Quiz[i] = quiz
		out.Quiz[i].Answers = nil
		out.Quiz[i].ExplanationTH = ""
		if quiz.Options != nil {
			out.Quiz[i].Options = append([]string(nil), quiz.Options...)
		}
	}
	out.Listening = make([]ListeningItem, len(l.Listening))
	for i, item := range l.Listening {
		out.Listening[i] = item
		out.Listening[i].AnswerTH = ""
		out.Listening[i].MeaningTH = ""
		out.Listening[i].OptionsTH = append([]string(nil), item.OptionsTH...)
	}
	// Review prompts are scheduled server-side after the unit is learned.
	out.Review = nil
	out.UsageTH = append([]string(nil), l.UsageTH...)
	out.Forms = append([]Form(nil), l.Forms...)
	out.SignalWords = append([]string(nil), l.SignalWords...)
	out.CommonMistakes = append([]Mistake(nil), l.CommonMistakes...)
	out.Examples = append([]Example(nil), l.Examples...)
	out.Vocabulary = append([]Vocabulary(nil), l.Vocabulary...)
	out.Shadowing = append([]string(nil), l.Shadowing...)
	out.ConceptTags = append([]string(nil), l.ConceptTags...)
	out.Speaking.TargetVocabulary = append([]string(nil), l.Speaking.TargetVocabulary...)
	out.Speaking.SuccessCriteriaTH = append([]string(nil), l.Speaking.SuccessCriteriaTH...)
	return out
}

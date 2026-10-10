package app

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/google/uuid"
	"tokoloop/internal/ebook"
	"tokoloop/internal/learning"
)

func TestCourseAnswerKeyIgnoresSurfaceFormOnly(t *testing.T) {
	same := [][2]string{
		{"I'm working.", "i am working"},
		{"She doesn't like coffee", "She does not like coffee."},
		{"We can't come", "We cannot come."},
		{"They’ll call you!", "they will call you"},
		{"Where's the bus?", "where is the bus"},
	}
	for _, pair := range same {
		if courseAnswerKey(pair[0]) != courseAnswerKey(pair[1]) {
			t.Fatalf("%q and %q should compare equal: %q vs %q", pair[0], pair[1], courseAnswerKey(pair[0]), courseAnswerKey(pair[1]))
		}
	}
	different := [][2]string{
		{"She works here.", "She work here."},
		{"I went home.", "I go home."},
		{"a", "the"},
	}
	for _, pair := range different {
		if courseAnswerKey(pair[0]) == courseAnswerKey(pair[1]) {
			t.Fatalf("%q and %q must stay different", pair[0], pair[1])
		}
	}
	if got := courseFilledSentence("She ___ (work) from home today.", "is working"); got != "She is working from home today." {
		t.Fatalf("filled sentence = %q", got)
	}
	// A fix item whose only mistake is a comma is not solved by copying it.
	comma := ebook.QuizItem{Kind: "fix", PromptEN: "Mina who leads the team is on leave.", Answers: []string{"Mina, who leads the team, is on leave."}}
	if courseAnswerAccepted(comma, "Mina who leads the team is on leave.") || !courseAnswerAccepted(comma, "mina, who leads the team, is on leave") {
		t.Fatal("a punctuation-only fix item accepted the uncorrected sentence or rejected the correction")
	}
	// A one-letter option is text, never a position.
	article := ebook.QuizItem{Kind: "choice", PromptEN: "It is ___ old car.", Options: []string{"the", "a", "an"}, Answers: []string{"an"}}
	if got := courseAnswerText(article, "a"); got != "a" {
		t.Fatalf("option text %q was read as a position: %q", "a", got)
	}
	if !courseAnswerAccepted(article, courseAnswerText(article, "3")) || courseAnswerAccepted(article, courseAnswerText(article, "a")) {
		t.Fatal("choice answers are not matched by option text and public option number")
	}
}

func TestEbookCourseDeterministicQuizKindsNeverCallTheModel(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("a deterministic quiz kind called the tutor model: %s", r.URL.Path)
		response(w, `{"marks":[]}`)
	})
	defer fake.Close()
	lesson := a.Course.Lessons[0]
	answers := map[string]string{}
	wrong := map[string]string{}
	for _, quiz := range lesson.Quiz {
		switch quiz.Kind {
		case "choice", "fill":
			answers[quiz.ID] = quiz.Answers[0]
			wrong[quiz.ID] = "zzz"
		case "reorder":
			answers[quiz.ID] = quiz.Answers[0]
			wrong[quiz.ID] = strings.Join(quiz.Options, " ")
		}
	}
	if len(answers) != 5 {
		t.Fatalf("lesson has %d deterministic quiz items, want 5", len(answers))
	}
	missed := owner.call(200, "POST", "/ebook/units/"+lesson.ID+"/check", map[string]any{"request_id": uuid.NewString(), "answers": wrong})
	for _, raw := range ebookCourseArray(missed, "marks") {
		mark := ebookCourseObject(raw)
		if mark["correct"] != false || ebookCourseString(mark, "answer") != "" || ebookCourseString(mark, "reason_th") == "" {
			t.Fatalf("wrong deterministic answer was not rejected with an explanation and no answer key: %#v", mark)
		}
	}
	passed := owner.call(200, "POST", "/ebook/units/"+lesson.ID+"/check", map[string]any{"request_id": uuid.NewString(), "answers": answers})
	for _, raw := range ebookCourseArray(passed, "marks") {
		if mark := ebookCourseObject(raw); mark["correct"] != true {
			t.Fatalf("accepted answer was rejected: %#v", mark)
		}
	}
	// A fill answer may also be typed as the whole sentence.
	fill := lesson.Quiz[2]
	whole := owner.call(200, "POST", "/ebook/units/"+lesson.ID+"/check", map[string]any{"request_id": uuid.NewString(), "answers": map[string]string{fill.ID: courseFilledSentence(fill.PromptEN, fill.Answers[0])}})
	if mark := ebookCourseObject(ebookCourseArray(whole, "marks")[0]); mark["correct"] != true {
		t.Fatalf("whole-sentence fill answer was rejected: %#v", mark)
	}
	detail := owner.call(200, "GET", "/ebook/units/"+lesson.ID, nil)
	for _, raw := range ebookCourseArray(ebookCourseObject(detail["lesson"]), "listening_items") {
		item := ebookCourseObject(raw)
		if ebookCourseString(item, "answer_th") != "" || ebookCourseString(item, "meaning_th") != "" {
			t.Fatalf("listening answer or meaning leaked before the learner answered: %#v", item)
		}
	}
	if review, exists := ebookCourseObject(detail["lesson"])["review"]; exists && review != nil {
		t.Fatalf("scheduled review prompts leaked into the public lesson: %#v", review)
	}
}

// ebookCourseFinishUnit completes all five steps of a unit the way a learner
// would: self-paced reading, every quiz item, listening, shadowing, speaking.
func ebookCourseFinishUnit(t *testing.T, a *App, client featureHTTP, lesson ebook.Lesson) {
	t.Helper()
	client.call(200, "PATCH", "/ebook/units/"+lesson.ID+"/progress", map[string]any{"current_step": 3, "completed_steps": map[string]bool{"understand": true, "examples": true}})
	answers := map[string]string{}
	for _, quiz := range lesson.Quiz {
		answers[quiz.ID] = quiz.Answers[0]
	}
	checked := client.call(200, "POST", "/ebook/units/"+lesson.ID+"/check", map[string]any{"request_id": uuid.NewString(), "answers": answers})
	if int(checked["correct"].(float64)) != len(lesson.Quiz) {
		t.Fatalf("answer key did not pass its own quiz: %#v", checked["marks"])
	}
	ebookCoursePassListening(t, a, client, lesson.ID)
	shadow := client.call(201, "POST", "/ebook/units/"+lesson.ID+"/sessions", map[string]any{"mode": "shadowing", "request_id": uuid.NewString()})
	for range lesson.Shadowing {
		client.audioTurn(ebookCourseString(shadow, "id"), uuid.NewString())
	}
	speak := client.call(201, "POST", "/ebook/units/"+lesson.ID+"/sessions", map[string]any{"mode": "speak", "request_id": uuid.NewString()})
	client.audioTurn(ebookCourseString(speak, "id"), uuid.NewString())
	client.audioTurn(ebookCourseString(speak, "id"), uuid.NewString())
}

func ebookCoursePassingAI(a *App) func() {
	fake := featureAI(a, func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "tts") {
			audioResponse(w)
			return
		}
		feedback, _ := json.Marshal(learning.Feedback{
			Transcript: "I work in Bangkok.", Reply: "Good work.", ReplyTH: "ดีมาก", Meaning: "ใช้รูปแบบได้", Correct: true, GoalMet: true, AudioClear: true,
			Corrections: []learning.Correction{}, Weaknesses: []string{}, Vocabulary: []string{}, Level: "A1",
		})
		response(w, string(feedback))
	})
	return fake.Close
}

func TestEbookCourseLearnedUnitEntersSpacedReviewEvenWhenEveryAnswerWasRight(t *testing.T) {
	a, owner, other, _ := ebookFixture(t)
	defer ebookCoursePassingAI(a)()
	path := a.Course.Path()
	first, second := path[0], path[1]
	if first.Level != "A1" {
		t.Fatalf("learning path starts at %s, want A1", first.Level)
	}
	before := owner.call(200, "GET", "/ebook", nil)
	if ebookCourseString(before, "recommended_unit_id") != first.ID {
		t.Fatalf("new learner is not sent to the first A1 unit: %q want %q", ebookCourseString(before, "recommended_unit_id"), first.ID)
	}
	ebookCourseFinishUnit(t, a, owner, first)

	var ownerID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	prefix := "ebook:" + ebook.LearnEbookCourseVersion + ":" + first.UnitID + ":review-%"
	var scheduled, dueNow, quizCards int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*),count(*) FILTER(WHERE due_at<=now()) FROM review_items WHERE user_id=$1 AND key LIKE $2", ownerID, prefix).Scan(&scheduled, &dueNow); err != nil {
		t.Fatal(err)
	}
	if scheduled != len(first.Review) || dueNow != 0 {
		t.Fatalf("learned unit review prompts scheduled=%d due_now=%d, want %d scheduled for later days", scheduled, dueNow, len(first.Review))
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND key LIKE $2", ownerID, "ebook:"+ebook.LearnEbookCourseVersion+":"+first.UnitID+":quiz-%").Scan(&quizCards); err != nil {
		t.Fatal(err)
	}
	if quizCards != 0 {
		t.Fatalf("a fully correct quiz created %d remedial cards", quizCards)
	}
	var words int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM vocabulary WHERE user_id=$1", ownerID).Scan(&words); err != nil {
		t.Fatal(err)
	}
	if words < len(first.Vocabulary) {
		t.Fatalf("learned unit saved %d vocabulary terms, want at least %d", words, len(first.Vocabulary))
	}

	catalog := owner.call(200, "GET", "/ebook", nil)
	unit := ebookCourseUnit(catalog, first.ID)
	if ebookCourseString(unit, "status") != "learned" || ebookCourseString(unit, "level") != first.Level || ebookCourseNumber(unit, "path_order") != 1 {
		t.Fatalf("learned unit summary = %#v", unit)
	}
	if ebookCourseString(catalog, "recommended_unit_id") != second.ID {
		t.Fatalf("recommended unit after finishing the first = %q, want %q", ebookCourseString(catalog, "recommended_unit_id"), second.ID)
	}
	plan := ebookCourseObject(owner.call(200, "GET", "/daily-plan", nil)["ebook"])
	if ebookCourseString(plan["next_unit"], "id") != second.ID || ebookCourseNumber(plan, "learned") != 1 || ebookCourseNumber(plan, "total") != 145 {
		t.Fatalf("daily plan ebook block = %#v", plan)
	}

	// The unit comes back as review once its first prompt is due, and finishing
	// it again never schedules a second copy.
	if _, err := a.DB.Exec(context.Background(), "UPDATE review_items SET due_at=now()-interval '1 hour' WHERE user_id=$1 AND key LIKE $2", ownerID, prefix); err != nil {
		t.Fatal(err)
	}
	// The PATCH is a learner mutation, so it also drops the cached catalog that
	// was read before the due date was moved directly in the database.
	owner.call(200, "PATCH", "/ebook/units/"+first.ID+"/progress", map[string]any{"current_step": 5})
	if status := ebookCourseString(ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), first.ID), "status"); status != "review" {
		t.Fatalf("due unit review did not surface in the catalog: %q", status)
	}
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND key LIKE $2", ownerID, prefix).Scan(&scheduled); err != nil {
		t.Fatal(err)
	}
	if scheduled != len(first.Review) {
		t.Fatalf("review prompts were duplicated: %d", scheduled)
	}
	if status := ebookCourseString(ebookCourseUnit(other.call(200, "GET", "/ebook", nil), first.ID), "status"); status != "unlearned" {
		t.Fatalf("another learner saw this learner's progress: %q", status)
	}
}

func TestEbookCourseCarriesPreviousVersionProgressWithoutRewritingIt(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	var ownerID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	previous := ebook.LearnEbookPreviousCourseVersions[len(ebook.LearnEbookPreviousCourseVersions)-1]
	learned := map[string]any{
		"current_step": 5, "learned": true, "completed_at": "2026-09-28T00:00:00Z",
		"completed_steps": map[string]bool{"understand": true, "examples": true, "quiz": true, "shadowing": true, "speaking": true},
		"quiz_scores":     map[string]bool{"quiz-01": true}, "vocabulary_saved": map[string]bool{"agenda": true},
	}
	halfway := map[string]any{"current_step": 3, "completed_steps": map[string]bool{"understand": true, "examples": true}, "quiz_answers": map[string]string{"quiz-01": "old"}}
	for unit, state := range map[string]map[string]any{"2": learned, "3": halfway} {
		if _, err := a.DB.Exec(context.Background(), "INSERT INTO ebook_progress(user_id,unit_id,version,state) VALUES($1,$2,$3,$4)", ownerID, unit, previous, asJSON(state)); err != nil {
			t.Fatal(err)
		}
	}
	catalog := owner.call(200, "GET", "/ebook", nil)
	carried := ebookCourseUnit(catalog, "ebook-002")
	if ebookCourseString(carried, "status") != "review" || ebookCourseNumber(carried, "percent") != 100 {
		t.Fatalf("a unit learned on the previous course version = %#v, want review at 100%%", carried)
	}
	restarted := ebookCourseUnit(catalog, "ebook-003")
	if ebookCourseString(restarted, "status") != "learning" || ebookCourseNumber(restarted, "percent") != 0 || ebookCourseNumber(restarted, "current_step") != 1 {
		t.Fatalf("an unfinished unit from the previous version = %#v, want a restart at step 1", restarted)
	}
	progress := ebookCourseObject(owner.call(200, "GET", "/ebook/units/ebook-002", nil)["progress"])
	if progress["learned"] != true || ebookCourseString(progress, "migrated_from") != previous || len(ebookCourseObject(progress["quiz_scores"])) != 0 {
		t.Fatalf("carried progress = %#v", progress)
	}
	if ebookCourseObject(progress["vocabulary_saved"])["agenda"] != true {
		t.Fatalf("saved vocabulary flags were dropped: %#v", progress)
	}

	// Passing the rewritten quiz settles the carried unit and schedules review.
	lesson, _ := a.Course.Lesson("ebook-002")
	answers := map[string]string{}
	for _, quiz := range lesson.Quiz {
		answers[quiz.ID] = quiz.Answers[0]
	}
	owner.call(200, "POST", "/ebook/units/ebook-002/check", map[string]any{"request_id": uuid.NewString(), "answers": answers})
	if status := ebookCourseString(ebookCourseUnit(owner.call(200, "GET", "/ebook", nil), "ebook-002"), "status"); status != "learned" {
		t.Fatalf("carried unit after passing the new quiz = %q, want learned", status)
	}
	var scheduled int
	if err := a.DB.QueryRow(context.Background(), "SELECT count(*) FROM review_items WHERE user_id=$1 AND key LIKE $2", ownerID, "ebook:"+ebook.LearnEbookCourseVersion+":2:review-%").Scan(&scheduled); err != nil {
		t.Fatal(err)
	}
	if scheduled != len(lesson.Review) {
		t.Fatalf("carried unit scheduled %d review prompts, want %d", scheduled, len(lesson.Review))
	}
	var raw []byte
	if err := a.DB.QueryRow(context.Background(), "SELECT state FROM ebook_progress WHERE user_id=$1 AND unit_id='2' AND version=$2", ownerID, previous).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if old := ebookCourseState(raw); ebookCourseObject(old["quiz_scores"])["quiz-01"] != true || old["migrated_from"] != nil {
		t.Fatalf("previous-version row was rewritten: %#v", old)
	}
}

func TestReviewSittingIsCappedAndStartsWithTheMostOverdue(t *testing.T) {
	a, owner, _, _ := ebookFixture(t)
	var ownerID string
	if err := a.DB.QueryRow(context.Background(), "SELECT id::text FROM users WHERE username='ebook-owner'").Scan(&ownerID); err != nil {
		t.Fatal(err)
	}
	total := reviewSessionLimit + 6
	for i := 0; i < total; i++ {
		if _, err := a.DB.Exec(context.Background(), `INSERT INTO review_items(id,user_id,key,kind,title,prompt,target,meaning,due_at,cue_version) VALUES($1,$2,$3,'pattern','t',$4,'target','m',now()-make_interval(hours => $5),1)`, uuid.NewString(), ownerID, "cap:"+uuid.NewString(), "prompt "+string(rune('a'+i)), i+1); err != nil {
			t.Fatal(err)
		}
	}
	status, raw, _ := ebookRaw(t, a, owner.token, "GET", "/review", nil)
	if status != 200 {
		t.Fatalf("review status = %d", status)
	}
	var items []map[string]any
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatal(err)
	}
	if len(items) != reviewSessionLimit {
		t.Fatalf("review sitting returned %d items, want the cap %d", len(items), reviewSessionLimit)
	}
	if items[0]["prompt"] != "prompt "+string(rune('a'+total-1)) {
		t.Fatalf("review sitting does not start with the most overdue item: %#v", items[0]["prompt"])
	}
	if due := owner.call(200, "GET", "/daily-plan", nil)["due_count"]; int(due.(float64)) != total {
		t.Fatalf("daily plan hid the real number of due items: %v", due)
	}
}

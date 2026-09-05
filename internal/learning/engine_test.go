package learning

import (
	"testing"
	"time"
)

func TestSpeakingEvidence(t *testing.T) {
	f := Feedback{Correct: true, AudioClear: true, GoalMet: true}
	for _, tc := range []struct {
		kind        string
		hint        int
		retry, want bool
	}{{"audio", 0, false, true}, {"text", 0, false, false}, {"audio", 1, false, false}, {"audio", 0, true, false}} {
		if got := Independent(f, tc.kind, tc.hint, tc.retry); got != tc.want {
			t.Errorf("%+v got %v", tc, got)
		}
	}
	f.AudioClear = false
	if Independent(f, "audio", 0, false) {
		t.Fatal("unclear audio must not count")
	}
}
func TestFeedbackCannotInventPronunciation(t *testing.T) {
	f := Feedback{Reply: "Good", Correct: true, GoalMet: true, AudioClear: true, Pronunciation: "clear"}
	if e := f.Validate(false); e != nil {
		t.Fatal(e)
	}
	if f.Pronunciation != "" || f.AudioClear {
		t.Fatal("typed response retained audio assessment")
	}
	f = Feedback{Reply: "Please record again", Correct: true, GoalMet: true, AudioClear: false, Pronunciation: "bad", Weaknesses: []string{"grammar"}}
	f.Validate(true)
	if f.Correct || f.GoalMet || len(f.Weaknesses) > 0 || f.Pronunciation != "" {
		t.Fatal("noise became learning failure")
	}
}
func TestReviewIntervals(t *testing.T) {
	now := time.Date(2026, 9, 5, 10, 0, 0, 0, time.FixedZone("Bangkok", 7*3600))
	for _, tc := range []struct {
		stage            int
		correct          bool
		hint, next, days int
	}{{0, true, 0, 1, 1}, {2, true, 1, 2, 3}, {4, false, 0, 0, 1}, {5, true, 0, 6, 60}} {
		stage, due := Schedule(tc.stage, tc.correct, tc.hint, now)
		if stage != tc.next || !due.Equal(now.AddDate(0, 0, tc.days)) {
			t.Errorf("%+v got %d %s", tc, stage, due)
		}
	}
}
func TestHintsDoNotRevealEarly(t *testing.T) {
	for i := 1; i < 4; i++ {
		if Hint("I am [name]", "I am Somchai", "แนะนำตัว", i) == "I am Somchai\nแนะนำตัว" {
			t.Fatal("early reveal")
		}
	}
}

func TestMissingFeedbackRejected(t *testing.T) {
	if _, err := ParseFeedback(`{"reply":"Good job!"}`, false); err == nil {
		t.Fatal("missing assessment fields accepted")
	}
}

func TestReviewFullSequence(t *testing.T) {
	now := time.Now()
	stage := 0
	for _, days := range []int{1, 3, 7, 14, 30, 60, 60} {
		next, due := Schedule(stage, true, 0, now)
		if !due.Equal(now.AddDate(0, 0, days)) {
			t.Fatalf("wanted %d day interval, got %v", days, due.Sub(now))
		}
		stage = next
	}
}

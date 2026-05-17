package tutor

import (
	"strings"
	"testing"
)

func TestNormalizeAnswer(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"Sarah is in her car.", "sarah is in her car"},
		{"  SARAH is   in her car!  ", "sarah is in her car"},
		{"Sarah’s car is here.", "sarah's car is here"},
		{"\"Hello, world\"", "hello world"},
		{"", ""},
	}
	for _, c := range cases {
		got := NormalizeAnswer(c.in)
		if got != c.want {
			t.Errorf("NormalizeAnswer(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEvaluateAnswerExact(t *testing.T) {
	r := EvaluateAnswer("Sarah is in her car", "Sarah is in her car")
	if !r.IsCorrect || r.Score != 1.0 || !r.ExactMatch {
		t.Fatalf("expected exact match, got %+v", r)
	}
	r = EvaluateAnswer("Sarah is in her car.", "sarah is in her car!")
	if !r.IsCorrect || r.Score != 1.0 {
		t.Fatalf("expected normalised exact match, got %+v", r)
	}
}

func TestEvaluateAnswerPartial(t *testing.T) {
	r := EvaluateAnswer("Sarah is in her car", "Sarah in her car")
	if r.IsCorrect {
		t.Fatalf("partial answer should not be marked correct: %+v", r)
	}
	if r.Score <= 0 || r.Score >= 1 {
		t.Fatalf("partial score should be between 0 and 1 exclusive, got %v", r.Score)
	}
	if len(r.MissingWords) == 0 {
		t.Fatalf("expected at least one missing word")
	}
}

func TestEvaluateAnswerEmpty(t *testing.T) {
	r := EvaluateAnswer("Sarah is in her car", "")
	if r.Score != 0 || r.IsCorrect {
		t.Fatalf("empty answer must score 0: %+v", r)
	}
}

func TestBuildMaskedHint(t *testing.T) {
	h := BuildMaskedHint("Sarah is in her car", 1)
	expected := "S____ i_ i_ h__ c__"
	if h != expected {
		t.Fatalf("hint level 1 = %q, want %q", h, expected)
	}
	// word count preserved
	if len(strings.Fields(h)) != 5 {
		t.Fatalf("hint should preserve word count, got %q", h)
	}
}

func TestBuildMaskedHintApostrophes(t *testing.T) {
	h := BuildMaskedHint("It's a sunny day", 1)
	if !strings.Contains(h, "'") {
		t.Fatalf("hint should keep apostrophes: %q", h)
	}
}

func TestIsAnswerRevealRequest(t *testing.T) {
	yes := []string{"เฉลย", "ขอเฉลยหน่อย", "show the answer please", "Reveal answer"}
	no := []string{"hello", "I want to learn", ""}
	for _, s := range yes {
		if !IsAnswerRevealRequest(s) {
			t.Errorf("expected reveal=true for %q", s)
		}
	}
	for _, s := range no {
		if IsAnswerRevealRequest(s) {
			t.Errorf("expected reveal=false for %q", s)
		}
	}
}

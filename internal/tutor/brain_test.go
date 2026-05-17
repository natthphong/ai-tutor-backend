package tutor

import "testing"

func TestClassifyTutorIntent(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		clientAction string
		want         string
	}{
		{"answer default", "She went to work yesterday.", "", IntentAnswer},
		{"thai question", "past simple ใช้ยังไง", "", IntentQuestion},
		{"english question", "What does this mean?", "", IntentQuestion},
		{"thai hint", "ขอใบ้หน่อย", "", IntentHintRequest},
		{"english hint", "hint please", "", IntentHintRequest},
		{"repeat", "ฟังอีกครั้ง", "", IntentRepeatRequest},
		{"review", "ขอทวนบทนี้", "", IntentReviewRequest},
		{"restart", "เริ่มใหม่", "", IntentRestartUnit},
		{"client action wins", "She went home.", "hint", IntentHintRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyTutorIntent(tt.text, tt.clientAction, "listening")
			if got.Intent != tt.want {
				t.Fatalf("intent = %s, want %s", got.Intent, tt.want)
			}
			if got.Confidence <= 0 {
				t.Fatalf("confidence should be positive")
			}
		})
	}
}

func TestClassifyTutorIntent_ChangeMode(t *testing.T) {
	got := ClassifyTutorIntent("ขอฝึกพูด", "", "listening")
	if got.Intent != IntentChangeMode {
		t.Fatalf("intent = %s, want %s", got.Intent, IntentChangeMode)
	}
	if got.Mode != "speaking" {
		t.Fatalf("mode = %s, want speaking", got.Mode)
	}
}

func TestNormalizeScore(t *testing.T) {
	tests := []struct {
		score float64
		want  float64
	}{
		{0.8, 0.8},
		{4, 0.8},
		{80, 0.8},
		{-1, 0},
	}
	for _, tt := range tests {
		if got := normalizeScore(tt.score); got != tt.want {
			t.Fatalf("normalizeScore(%v) = %v, want %v", tt.score, got, tt.want)
		}
	}
}

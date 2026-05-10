package tutor

import (
	"testing"
	"time"
)

func TestDecideNextAction_VocabularyDue(t *testing.T) {
	input := DecisionInput{
		UserID: "test", CurrentUnitID: 1,
		DueItems: DueItems{VocabularyDueCount: 5},
	}
	result := DecideNextAction(input)
	if result.Action != "review_vocabulary" {
		t.Errorf("expected review_vocabulary, got %s", result.Action)
	}
}

func TestDecideNextAction_WeaknessDue(t *testing.T) {
	input := DecisionInput{
		UserID: "test", CurrentUnitID: 1,
		DueItems: DueItems{WeaknessDueCount: 3},
	}
	result := DecideNextAction(input)
	if result.Action != "review_weakness" {
		t.Errorf("expected review_weakness, got %s", result.Action)
	}
}

func TestDecideNextAction_ContinueUnit(t *testing.T) {
	input := DecisionInput{
		UserID: "test", CurrentUnitID: 1,
		UnitStatus: "in_progress", CurrentStep: "listening_practice",
	}
	result := DecideNextAction(input)
	if result.Action != "start_speaking" {
		t.Errorf("expected start_speaking, got %s", result.Action)
	}
}

func TestDecideNextAction_StartNextUnit(t *testing.T) {
	input := DecisionInput{
		UserID: "test", CurrentUnitID: 2,
		UnitStatus: "completed", WeaknessThreshold: 5,
	}
	result := DecideNextAction(input)
	if result.Action != "start_next_unit" {
		t.Errorf("expected start_next_unit, got %s", result.Action)
	}
}

func TestDecideNextAction_TooManyWeaknesses(t *testing.T) {
	input := DecisionInput{
		UserID: "test", CurrentUnitID: 2,
		RecentWeaknesses: 10, WeaknessThreshold: 5,
	}
	result := DecideNextAction(input)
	if result.Action != "review_weakness" {
		t.Errorf("expected review_weakness, got %s", result.Action)
	}
}

func TestCalculateNextDue(t *testing.T) {
	tests := []struct {
		score    float64
		expected time.Duration
	}{
		{0.5, 24 * time.Hour},
		{0.7, 2 * 24 * time.Hour},
		{0.8, 4 * 24 * time.Hour},
		{0.9, 7 * 24 * time.Hour},
	}
	for _, tt := range tests {
		due := CalculateNextDue(tt.score, 0, 0)
		diff := due.Sub(time.Now())
		if diff < tt.expected-time.Hour || diff > tt.expected+time.Hour {
			t.Errorf("score %.1f: expected ~%v, got %v", tt.score, tt.expected, diff)
		}
	}
}

func TestUpdateMasteryScore(t *testing.T) {
	tests := []struct {
		current  float64
		score    float64
		expected float64
	}{
		{0.5, 0.95, 0.60},
		{0.5, 0.80, 0.55},
		{0.5, 0.40, 0.40},
		{0.0, 0.30, 0.0},   // clamp at 0
		{0.95, 0.95, 1.0},  // clamp at 1
	}
	for _, tt := range tests {
		result := UpdateMasteryScore(tt.current, tt.score)
		if result != tt.expected {
			t.Errorf("UpdateMastery(%v, %v) = %v, want %v", tt.current, tt.score, result, tt.expected)
		}
	}
}

func TestMasteryLevel(t *testing.T) {
	tests := []struct {
		score    float64
		expected string
	}{
		{0.9, "mastered"},
		{0.7, "improving"},
		{0.4, "learning"},
		{0.1, "weak"},
	}
	for _, tt := range tests {
		result := MasteryLevel(tt.score)
		if result != tt.expected {
			t.Errorf("MasteryLevel(%v) = %s, want %s", tt.score, result, tt.expected)
		}
	}
}

package tutor

import (
	"fmt"
	"time"
)

// NextAction represents what the tutor should do next
type NextAction struct {
	Action      string      `json:"nextAction"`
	Reason      string      `json:"reason"`
	Mode        string      `json:"mode"`
	UnitID      int         `json:"unitId,omitempty"`
	Items       interface{} `json:"items,omitempty"`
	Instruction string      `json:"instruction"`
}

// DueItems holds counts of items due for review
type DueItems struct {
	VocabularyDueCount int `json:"vocabularyDueCount"`
	WeaknessDueCount   int `json:"weaknessDueCount"`
	UnitReviewDueCount int `json:"unitReviewDueCount"`
}

// DecisionInput is the input to the decision engine
type DecisionInput struct {
	UserID            string
	CurrentUnitID     int
	DueItems          DueItems
	RecentWeaknesses  int
	PreferredMode     string
	CurrentStep       string
	UnitStatus        string
	WeaknessThreshold int
}

// DecideNextAction determines what the tutor should do next
// Priority: 1. Vocab flashcards → 2. Weakness review → 3. Unit review → 4. Continue unit → 5. Next unit
func DecideNextAction(input DecisionInput) NextAction {
	// Priority 1: Vocabulary flashcards due today
	if input.DueItems.VocabularyDueCount > 0 {
		return NextAction{
			Action:      "review_vocabulary",
			Reason:      fmt.Sprintf("คุณมี %d flashcard ที่ต้องทวนวันนี้", input.DueItems.VocabularyDueCount),
			Mode:        "vocabulary_review",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาทวนคำศัพท์กันก่อนครับ",
		}
	}

	// Priority 2: Weakness review due today
	if input.DueItems.WeaknessDueCount > 0 {
		return NextAction{
			Action:      "review_weakness",
			Reason:      fmt.Sprintf("มี %d จุดอ่อนที่ต้องทวน", input.DueItems.WeaknessDueCount),
			Mode:        "weakness_review",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาทวนจุดที่ยังไม่แม่นกันครับ",
		}
	}

	// Priority 3: Unit review due today
	if input.DueItems.UnitReviewDueCount > 0 {
		return NextAction{
			Action:      "review_unit",
			Reason:      fmt.Sprintf("มี %d บทเรียนที่ถึงเวลาทวน", input.DueItems.UnitReviewDueCount),
			Mode:        "review",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาทวนบทเรียนที่เรียนไปแล้วกันครับ",
		}
	}

	// Priority 4: Continue current unfinished unit
	if input.UnitStatus == "in_progress" {
		return decideUnitStep(input)
	}

	// Check if too many weaknesses - should review before moving on
	if input.RecentWeaknesses >= input.WeaknessThreshold {
		return NextAction{
			Action:      "review_weakness",
			Reason:      fmt.Sprintf("คุณมีจุดอ่อน %d อัน ควรทวนก่อนไปบทถัดไป", input.RecentWeaknesses),
			Mode:        "weakness_review",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาทวนจุดที่ยังไม่แม่นก่อนไปบทใหม่ครับ",
		}
	}

	// Priority 5: Start next unit
	return NextAction{
		Action:      "start_next_unit",
		Reason:      "พร้อมเรียนบทใหม่แล้ว",
		Mode:        selectMode(input.PreferredMode),
		UnitID:      input.CurrentUnitID,
		Instruction: fmt.Sprintf("มาเริ่มเรียน Unit %d กันครับ", input.CurrentUnitID),
	}
}

// decideUnitStep determines the next step within a unit
func decideUnitStep(input DecisionInput) NextAction {
	switch input.CurrentStep {
	case "intro":
		return NextAction{
			Action:      "continue_unit",
			Mode:        "mixed",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาเรียน Grammar กันครับ",
		}
	case "grammar_explanation":
		return NextAction{
			Action:      "start_listening",
			Mode:        "listening",
			UnitID:      input.CurrentUnitID,
			Instruction: "ฟังดีๆ แล้วพิมพ์สิ่งที่ได้ยินครับ",
		}
	case "listening_practice":
		return NextAction{
			Action:      "start_speaking",
			Mode:        "speaking",
			UnitID:      input.CurrentUnitID,
			Instruction: "ลองพูดตามตัวอย่างครับ",
		}
	case "speaking_practice":
		return NextAction{
			Action:      "start_reading",
			Mode:        "reading",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาอ่านและแปลกันครับ",
		}
	case "reading_practice":
		return NextAction{
			Action:      "mini_quiz",
			Mode:        "mixed",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาทำแบบทดสอบกันครับ",
		}
	case "mini_quiz":
		return NextAction{
			Action:      "review_summary",
			Mode:        "mixed",
			UnitID:      input.CurrentUnitID,
			Instruction: "มาสรุปสิ่งที่เรียนกันครับ",
		}
	case "review_summary":
		return NextAction{
			Action:      "schedule_review",
			Mode:        "mixed",
			UnitID:      input.CurrentUnitID,
			Instruction: "จัดตารางทวนให้เรียบร้อยครับ",
		}
	default:
		return NextAction{
			Action:      "continue_unit",
			Mode:        selectMode(input.PreferredMode),
			UnitID:      input.CurrentUnitID,
			Instruction: "มาเรียนต่อครับ",
		}
	}
}

func selectMode(preferred string) string {
	if preferred == "" || preferred == "mixed" {
		return "mixed"
	}
	return preferred
}

// CalculateNextDue calculates the next review date based on score
func CalculateNextDue(score float64, reviewCount int, consecutiveCorrect int) time.Time {
	now := time.Now()

	if score < 0.60 {
		return now.Add(24 * time.Hour) // +1 day
	}
	if score < 0.75 {
		return now.Add(2 * 24 * time.Hour) // +2 days
	}
	if score < 0.85 {
		return now.Add(4 * 24 * time.Hour) // +4 days
	}
	if score < 1.0 {
		return now.Add(7 * 24 * time.Hour) // +7 days
	}

	// Perfect score - use increasing intervals
	switch {
	case consecutiveCorrect >= 5:
		return now.Add(60 * 24 * time.Hour) // +60 days
	case consecutiveCorrect >= 4:
		return now.Add(30 * 24 * time.Hour) // +30 days
	case consecutiveCorrect >= 3:
		return now.Add(14 * 24 * time.Hour) // +14 days
	default:
		return now.Add(7 * 24 * time.Hour) // +7 days
	}
}

// UpdateMasteryScore updates mastery based on user performance
func UpdateMasteryScore(current float64, score float64) float64 {
	switch {
	case score >= 0.9:
		current += 0.10
	case score >= 0.75:
		current += 0.05
	case score < 0.60:
		current -= 0.10
	}

	// Clamp to 0-1 range
	if current < 0 {
		current = 0
	}
	if current > 1.0 {
		current = 1.0
	}
	return current
}

// MasteryLevel returns a human-readable mastery level
func MasteryLevel(score float64) string {
	switch {
	case score >= 0.81:
		return "mastered"
	case score >= 0.61:
		return "improving"
	case score >= 0.31:
		return "learning"
	default:
		return "weak"
	}
}

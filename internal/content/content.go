package content

import (
	_ "embed"
	"encoding/json"
)

type Word struct {
	Term    string `json:"term"`
	Meaning string `json:"meaning"`
	Example string `json:"example"`
}
type Drill struct {
	Kind            string `json:"kind"`
	Prompt          string `json:"prompt"`
	Target          string `json:"target"`
	TimeGoalSeconds int    `json:"time_goal_seconds,omitempty"`
}
type Lesson struct {
	GrammarFocus       string              `json:"grammar_focus,omitempty"`
	SourceUnits        []int               `json:"source_units,omitempty"`
	ID                 string              `json:"id"`
	Ordinal            int                 `json:"ordinal"`
	Level              string              `json:"level"`
	Unit               int                 `json:"unit"`
	UnitTitle          string              `json:"unit_title"`
	Title              string              `json:"title"`
	Objective          string              `json:"objective"`
	Pattern            string              `json:"pattern"`
	Example            string              `json:"example"`
	Meaning            string              `json:"meaning"`
	Explanation        string              `json:"explanation"`
	Vocabulary         []Word              `json:"vocabulary"`
	Drills             []Drill             `json:"drills"`
	ConversationPrompt string              `json:"conversation_prompt"`
	Acceptance         []string            `json:"acceptance"`
	Version            string              `json:"version"`
	Assessment         bool                `json:"assessment"`
	CoachingNotes      string              `json:"coaching_notes"`
	Slots              []map[string]string `json:"slots"`
}
type Scenario struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Category        string   `json:"category"`
	Level           string   `json:"level"`
	Goal            string   `json:"goal"`
	Roles           []string `json:"roles"`
	Brief           string   `json:"brief"`
	Opening         string   `json:"opening"`
	SuccessCriteria []string `json:"success_criteria"`
	Minutes         int      `json:"minutes"`
	StarterPattern  string   `json:"starter_pattern,omitempty"`
}

//go:embed lessons.json
var lessonJSON []byte

//go:embed expansion.json
var expansionJSON []byte

//go:embed scenarios.json
var scenarioJSON []byte

func Lessons() []Lesson {
	var v []Lesson
	if e := json.Unmarshal(lessonJSON, &v); e != nil {
		panic(e)
	}
	var extra []Lesson
	if e := json.Unmarshal(expansionJSON, &extra); e != nil {
		panic(e)
	}
	return append(v, extra...)
}
func Scenarios() []Scenario {
	var v []Scenario
	if e := json.Unmarshal(scenarioJSON, &v); e != nil {
		panic(e)
	}
	return v
}

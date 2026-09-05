package content

import "testing"

func TestCompleteCurriculum(t *testing.T) {
	ls := Lessons()
	if len(ls) != 525 {
		t.Fatal(len(ls))
	}
	seen := map[string]bool{}
	units := map[string]int{}
	for _, l := range ls {
		if seen[l.ID] || l.Example == "" || l.Pattern == "" || l.Meaning == "" || len(l.Drills) != 4 || len(l.Vocabulary) == 0 {
			t.Fatalf("invalid lesson: %+v", l)
		}
		seen[l.ID] = true
		units[l.Level]++
	}
	for _, level := range []string{"Pre-A1", "A1", "A2", "B1", "B2"} {
		expected := 145
		if level == "Pre-A1" || level == "A1" {
			expected = 45
		}
		if units[level] != expected {
			t.Fatal(level, units[level])
		}
	}
}
func TestScenarioCoverage(t *testing.T) {
	ss := Scenarios()
	if len(ss) != 70 {
		t.Fatal(len(ss))
	}
	counts := map[string]int{}
	for _, s := range ss {
		if s.Goal == "" || s.Opening == "" || len(s.Roles) < 2 || len(s.SuccessCriteria) < 1 {
			t.Fatal(s.ID)
		}
		counts[s.Category]++
	}
	if counts["Everyday"] != 20 {
		t.Fatal("everyday coverage")
	}
	for _, cat := range []string{"Tech", "Banking", "Business", "Interview", "Meeting"} {
		if counts[cat] != 10 {
			t.Fatal(cat)
		}
	}
}

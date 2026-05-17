package tutor

import (
	"math"
	"testing"
)

func TestParseTimestamp(t *testing.T) {
	cases := []struct {
		in   string
		want float64
	}{
		{"00:00:03.500", 3.5},
		{"00:01:30.000", 90},
		{"01:00:00.000", 3600},
		{"00:00.500", 0.5},
		{"05:12.250", 312.25},
		{"00:00:03,500", 3.5}, // comma tolerated
	}
	for _, c := range cases {
		got, err := ParseTimestamp(c.in)
		if err != nil {
			t.Errorf("ParseTimestamp(%q) error: %v", c.in, err)
			continue
		}
		if math.Abs(got-c.want) > 0.0001 {
			t.Errorf("ParseTimestamp(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

const manualVTT = `WEBVTT

1
00:00:00.000 --> 00:00:03.500
Hello and welcome to my channel.

2
00:00:03.500 --> 00:00:07.200
Today we will talk about the Byzantine Empire.

3
00:00:07.200 --> 00:00:11.000
It was the longest-lasting empire in history.
`

func TestParseVTT_Manual(t *testing.T) {
	caps, err := ParseVTT(manualVTT)
	if err != nil {
		t.Fatalf("ParseVTT err: %v", err)
	}
	if len(caps) != 3 {
		t.Fatalf("want 3 captions, got %d (%v)", len(caps), caps)
	}
	if caps[0].StartTime != 0 || math.Abs(caps[0].EndTime-3.5) > 0.001 {
		t.Errorf("first cue timing wrong: %+v", caps[0])
	}
	if caps[0].Text != "Hello and welcome to my channel." {
		t.Errorf("first cue text wrong: %q", caps[0].Text)
	}
}

// Auto-captions use rolling windows — the same words appear in multiple cues
// with extra trailing context. parseVTT + dedupRollingCaptions should keep
// only the fullest snapshot and use the original start time + the last end.
const autoVTT = `WEBVTT
Kind: captions
Language: en

00:00:00.080 --> 00:00:01.520 align:start position:0%
hello<00:00:00.480><c> and</c><00:00:00.720><c> welcome</c>

00:00:01.520 --> 00:00:01.530 align:start position:0%
hello and welcome

00:00:01.530 --> 00:00:03.000 align:start position:0%
hello and welcome<00:00:01.800><c> to</c><00:00:02.000><c> my</c><00:00:02.400><c> channel</c>

00:00:03.000 --> 00:00:03.010 align:start position:0%
hello and welcome to my channel

00:00:03.500 --> 00:00:05.000 align:start position:0%
today we will talk about the Byzantine Empire
`

func TestParseVTT_AutoCaptionsDedup(t *testing.T) {
	caps, err := ParseVTT(autoVTT)
	if err != nil {
		t.Fatalf("ParseVTT err: %v", err)
	}
	if len(caps) != 2 {
		t.Fatalf("want 2 deduped captions, got %d (%+v)", len(caps), caps)
	}
	if caps[0].Text != "hello and welcome to my channel" {
		t.Errorf("first dedup result wrong: %q", caps[0].Text)
	}
	if caps[0].StartTime > 0.1 {
		t.Errorf("first cue should retain earliest start, got %v", caps[0].StartTime)
	}
	if caps[1].Text != "today we will talk about the Byzantine Empire" {
		t.Errorf("second segment wrong: %q", caps[1].Text)
	}
}

func TestCombineToSentences(t *testing.T) {
	raw := []RawCaption{
		{StartTime: 0, EndTime: 3.5, Text: "Hello and welcome to my channel."},
		{StartTime: 3.5, EndTime: 7.2, Text: "Today we will talk about history"},
		{StartTime: 7.2, EndTime: 11, Text: "and the people who made it."},
	}
	segs := CombineToSentences(raw)
	if len(segs) != 2 {
		t.Fatalf("want 2 sentence segments, got %d (%+v)", len(segs), segs)
	}
	if segs[0].StartTime != 0 || math.Abs(segs[0].EndTime-3.5) > 0.001 {
		t.Errorf("segment 0 timing wrong: %+v", segs[0])
	}
	if segs[1].StartTime != 3.5 || math.Abs(segs[1].EndTime-11) > 0.001 {
		t.Errorf("segment 1 timing wrong: %+v", segs[1])
	}
	if segs[1].Text != "Today we will talk about history and the people who made it." {
		t.Errorf("segment 1 text wrong: %q", segs[1].Text)
	}
}

func TestMergeTextOverlap(t *testing.T) {
	cases := []struct {
		a, b string
		want string
	}{
		{"X Y Z", "Y Z W", "X Y Z W"},
		{"hello world", "world peace", "hello world peace"},
		{"There weren't really punishments", "There weren't really punishments not really",
			"There weren't really punishments not really"},
		{"not really for doing bad things", "for doing bad things unless you did something.",
			"not really for doing bad things unless you did something."},
		{"foo bar", "baz", "foo bar baz"},
		{"", "alone", "alone"},
		{"alone", "", "alone"},
	}
	for _, c := range cases {
		got := mergeTextOverlap(c.a, c.b)
		if got != c.want {
			t.Errorf("mergeTextOverlap(%q, %q) = %q, want %q", c.a, c.b, got, c.want)
		}
	}
}

func TestCombineToSentences_OverlapMerge(t *testing.T) {
	// Mirrors the bug report: YouTube auto-captions with rolling phrase
	// overlaps were producing duplicated text in the final segments.
	raw := []RawCaption{
		{StartTime: 0, EndTime: 2, Text: "There weren't really punishments"},
		{StartTime: 2, EndTime: 4, Text: "There weren't really punishments not really"},
		{StartTime: 4, EndTime: 6, Text: "not really for doing bad things"},
		{StartTime: 6, EndTime: 8, Text: "for doing bad things unless you did something."},
	}
	// dedupRollingCaptions collapses the first two via HasPrefix; the
	// remaining captions overlap by trailing words and must be merged with
	// overlap-stripping inside CombineToSentences.
	deduped := dedupRollingCaptions(raw)
	segs := CombineToSentences(deduped)
	if len(segs) != 1 {
		t.Fatalf("want 1 segment, got %d: %+v", len(segs), segs)
	}
	wantText := "There weren't really punishments not really for doing bad things unless you did something."
	if segs[0].Text != wantText {
		t.Errorf("merged text wrong:\n got: %q\nwant: %q", segs[0].Text, wantText)
	}
	if segs[0].StartTime != 0 || segs[0].EndTime != 8 {
		t.Errorf("timing wrong: %+v", segs[0])
	}
}

func TestCombineToSentences_GapBreaks(t *testing.T) {
	raw := []RawCaption{
		{StartTime: 0, EndTime: 2, Text: "first phrase"},
		// Big gap — should break even without sentence terminator.
		{StartTime: 5, EndTime: 7, Text: "second phrase"},
	}
	segs := CombineToSentences(raw)
	if len(segs) != 2 {
		t.Fatalf("gap should break into 2, got %d", len(segs))
	}
}

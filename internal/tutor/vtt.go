package tutor

import (
	"bufio"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// RawCaption is one cue from a WEBVTT subtitle file, with start/end in seconds.
type RawCaption struct {
	StartTime float64
	EndTime   float64
	Text      string
}

// ParseVTT reads the contents of a .vtt file and returns the cleaned cues in
// order. Inline word-level tags (e.g. `<00:00:00.480><c> and</c>`) are
// stripped. Rolling auto-caption windows are deduped so consecutive cues like
//
//	cue#1: "hello"
//	cue#2: "hello and"
//	cue#3: "hello and welcome"
//
// are collapsed to a single cue with the final text + the cue#1 start time +
// the cue#3 end time.
func ParseVTT(content string) ([]RawCaption, error) {
	var captions []RawCaption
	scanner := bufio.NewScanner(strings.NewReader(content))
	// Subtitle lines can be long for auto-captions; bump the scanner buffer.
	scanner.Buffer(make([]byte, 1024*64), 1024*1024)

	var cur *RawCaption
	var textBuf []string
	flush := func() {
		if cur != nil {
			text := cleanCaptionText(strings.Join(textBuf, " "))
			if text != "" {
				cur.Text = text
				captions = append(captions, *cur)
			}
		}
		cur = nil
		textBuf = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		trim := strings.TrimSpace(line)
		if trim == "" {
			flush()
			continue
		}
		if strings.HasPrefix(trim, "WEBVTT") ||
			strings.HasPrefix(trim, "NOTE") ||
			strings.HasPrefix(trim, "Kind:") ||
			strings.HasPrefix(trim, "Language:") ||
			strings.HasPrefix(trim, "STYLE") {
			continue
		}
		if strings.Contains(trim, "-->") {
			flush()
			start, end, err := parseCueLine(trim)
			if err == nil {
				cur = &RawCaption{StartTime: start, EndTime: end}
			}
			continue
		}
		// Numeric cue index (manual subs use these).
		if _, err := strconv.Atoi(trim); err == nil && cur == nil {
			continue
		}
		textBuf = append(textBuf, line)
	}
	flush()
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return dedupRollingCaptions(captions), nil
}

// parseCueLine parses `HH:MM:SS.mmm --> HH:MM:SS.mmm  align:start ...` cue
// timing lines.
func parseCueLine(line string) (float64, float64, error) {
	parts := strings.SplitN(line, "-->", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid cue line")
	}
	leftFields := strings.Fields(parts[0])
	rightFields := strings.Fields(parts[1])
	if len(leftFields) == 0 || len(rightFields) == 0 {
		return 0, 0, fmt.Errorf("invalid cue line")
	}
	start, err := ParseTimestamp(leftFields[len(leftFields)-1])
	if err != nil {
		return 0, 0, err
	}
	end, err := ParseTimestamp(rightFields[0])
	if err != nil {
		return 0, 0, err
	}
	return start, end, nil
}

// ParseTimestamp accepts `HH:MM:SS.mmm` or `MM:SS.mmm` (commas tolerated).
func ParseTimestamp(s string) (float64, error) {
	s = strings.ReplaceAll(s, ",", ".")
	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3:
		h, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		m, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		sec, err := strconv.ParseFloat(parts[2], 64)
		if err != nil {
			return 0, err
		}
		return h*3600 + m*60 + sec, nil
	case 2:
		m, err := strconv.ParseFloat(parts[0], 64)
		if err != nil {
			return 0, err
		}
		sec, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			return 0, err
		}
		return m*60 + sec, nil
	default:
		return 0, fmt.Errorf("invalid timestamp %q", s)
	}
}

var (
	captionTagRE     = regexp.MustCompile(`<[^>]+>`)
	captionWhitespRE = regexp.MustCompile(`\s+`)
)

func cleanCaptionText(s string) string {
	s = captionTagRE.ReplaceAllString(s, "")
	s = captionWhitespRE.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// dedupRollingCaptions collapses YouTube's rolling auto-caption windows by
// merging consecutive cues that extend each other. The result keeps the
// earliest start time, the latest end time, and the longest text. Example
// input → output:
//
//	"hello"                          [0.08-1.52]
//	"hello and welcome"              [1.52-1.53]   ┐
//	"hello and welcome to my channel"[1.53-3.00]   │ merged into ONE cue
//	"hello and welcome to my channel"[3.00-3.01]   ┘ at [0.08-3.01]
//	"today we will talk about..."    [3.50-5.00]
func dedupRollingCaptions(captions []RawCaption) []RawCaption {
	if len(captions) == 0 {
		return captions
	}
	out := make([]RawCaption, 0, len(captions))
	for _, c := range captions {
		if c.Text == "" {
			continue
		}
		if n := len(out); n > 0 {
			prev := &out[n-1]
			// Same text → just extend the span.
			if c.Text == prev.Text {
				if c.EndTime > prev.EndTime {
					prev.EndTime = c.EndTime
				}
				continue
			}
			// Rolling extension: cur extends prev → keep prev start, take cur text + end.
			if strings.HasPrefix(c.Text, prev.Text) || strings.Contains(c.Text, prev.Text) {
				prev.Text = c.Text
				if c.EndTime > prev.EndTime {
					prev.EndTime = c.EndTime
				}
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// CombineToSentences groups raw captions into sentence-level segments so each
// segment is one shadowing unit. Heuristic: combine consecutive cues until we
// see a sentence terminator (`. ! ?`) at the end of a cue, hit a long pause
// (> sentenceGapSec), or reach the maxSegmentSec / maxSegmentChars cap.
func CombineToSentences(raw []RawCaption) []ShadowingSegmentDTO {
	const (
		sentenceGapSec  = 1.0
		maxSegmentSec   = 8.0
		maxSegmentChars = 180
	)
	if len(raw) == 0 {
		return nil
	}
	segs := make([]ShadowingSegmentDTO, 0, len(raw))
	var (
		buf     strings.Builder
		start   float64
		end     float64
		hasAny  bool
		segIdx  int
	)
	flush := func() {
		if !hasAny {
			return
		}
		text := strings.TrimSpace(buf.String())
		if text != "" {
			segs = append(segs, ShadowingSegmentDTO{
				Index:     segIdx,
				StartTime: start,
				EndTime:   end,
				Text:      text,
			})
			segIdx++
		}
		buf.Reset()
		hasAny = false
	}

	for i, c := range raw {
		if !hasAny {
			start = c.StartTime
			hasAny = true
		} else {
			buf.WriteRune(' ')
		}
		buf.WriteString(c.Text)
		end = c.EndTime

		endsSentence := false
		for _, term := range []string{".", "!", "?", "…"} {
			if strings.HasSuffix(strings.TrimRight(c.Text, " \"')"), term) {
				endsSentence = true
				break
			}
		}
		nextGap := math.MaxFloat64
		if i+1 < len(raw) {
			nextGap = raw[i+1].StartTime - c.EndTime
		}
		duration := end - start
		if endsSentence || nextGap >= sentenceGapSec ||
			duration >= maxSegmentSec || buf.Len() >= maxSegmentChars ||
			i == len(raw)-1 {
			flush()
		}
	}
	flush()
	return segs
}

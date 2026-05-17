package tutor

import (
	"strings"
	"unicode"
)

// NormalizeAnswer returns a canonical form of an answer string suitable for
// deterministic comparison. It:
//   - trims surrounding whitespace
//   - lowercases everything
//   - collapses internal whitespace runs into a single space
//   - normalises common smart/curly punctuation to their ASCII equivalents
//   - strips terminal punctuation (.,!?;:)
//   - drops most punctuation aside from intra-word apostrophes
func NormalizeAnswer(s string) string {
	if s == "" {
		return ""
	}
	// Replace common unicode punctuation with ASCII equivalents.
	replacer := strings.NewReplacer(
		"‘", "'", "’", "'", "‛", "'",
		"“", "\"", "”", "\"",
		"–", "-", "—", "-",
		" ", " ",
	)
	s = replacer.Replace(s)
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '\'':
			// keep apostrophes for contractions (don't, it's, sarah's)
			b.WriteRune(r)
		case unicode.IsSpace(r):
			b.WriteRune(' ')
		default:
			// other punctuation -> space, then collapse
			b.WriteRune(' ')
		}
	}
	out := strings.Join(strings.Fields(b.String()), " ")
	// strip stray leading/trailing apostrophes (results from punctuation cleanup)
	out = strings.Trim(out, "'")
	return out
}

// EvaluationResult captures the deterministic comparison outcome.
type EvaluationResult struct {
	Score         float64  `json:"score"`         // 0.0 - 1.0
	IsCorrect     bool     `json:"isCorrect"`     // score >= 0.95 after normalisation
	NormalizedExp string   `json:"normalizedExpected"`
	NormalizedAns string   `json:"normalizedAnswer"`
	MissingWords  []string `json:"missingWords,omitempty"`
	ExtraWords    []string `json:"extraWords,omitempty"`
	MatchRatio    float64  `json:"matchRatio"`
	ExactMatch    bool     `json:"exactMatch"`
}

// EvaluateAnswer compares the user's answer against the expected answer
// deterministically. The returned score is always in [0,1]:
//   - 1.0 when normalised answers match exactly
//   - otherwise a blend of token overlap (weighted 0.7) and order similarity (0.3)
//   - 0.0 if the user answer is empty
func EvaluateAnswer(expected, user string) EvaluationResult {
	res := EvaluationResult{
		NormalizedExp: NormalizeAnswer(expected),
		NormalizedAns: NormalizeAnswer(user),
	}
	if res.NormalizedExp == "" {
		// nothing to compare against; treat any answer as 0 / unknown
		return res
	}
	if strings.TrimSpace(user) == "" {
		return res
	}
	if res.NormalizedAns == res.NormalizedExp {
		res.Score = 1.0
		res.IsCorrect = true
		res.ExactMatch = true
		res.MatchRatio = 1.0
		return res
	}

	expTokens := strings.Fields(res.NormalizedExp)
	ansTokens := strings.Fields(res.NormalizedAns)
	if len(expTokens) == 0 {
		return res
	}

	// Token overlap with multiset semantics.
	expCount := map[string]int{}
	for _, t := range expTokens {
		expCount[t]++
	}
	ansCount := map[string]int{}
	for _, t := range ansTokens {
		ansCount[t]++
	}
	overlap := 0
	for tok, c := range expCount {
		if got, ok := ansCount[tok]; ok {
			if got < c {
				overlap += got
			} else {
				overlap += c
			}
		}
	}
	tokenScore := float64(overlap) / float64(len(expTokens))
	res.MatchRatio = tokenScore

	// Order similarity via longest common subsequence over tokens.
	lcs := lcsLen(expTokens, ansTokens)
	maxLen := len(expTokens)
	if len(ansTokens) > maxLen {
		maxLen = len(ansTokens)
	}
	orderScore := 0.0
	if maxLen > 0 {
		orderScore = float64(lcs) / float64(maxLen)
	}

	// Slight penalty when the user produced significantly more tokens than expected.
	lengthPenalty := 1.0
	if len(ansTokens) > 0 && len(ansTokens) > 2*len(expTokens) {
		lengthPenalty = 0.85
	}

	combined := (0.7*tokenScore + 0.3*orderScore) * lengthPenalty
	if combined < 0 {
		combined = 0
	}
	if combined > 1 {
		combined = 1
	}
	res.Score = combined
	res.IsCorrect = combined >= 0.95

	for tok, c := range expCount {
		got := ansCount[tok]
		for i := 0; i < c-got; i++ {
			res.MissingWords = append(res.MissingWords, tok)
		}
	}
	for tok, c := range ansCount {
		got := expCount[tok]
		for i := 0; i < c-got; i++ {
			res.ExtraWords = append(res.ExtraWords, tok)
		}
	}
	return res
}

func lcsLen(a, b []string) int {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		for j := 1; j <= len(b); j++ {
			if a[i-1] == b[j-1] {
				cur[j] = prev[j-1] + 1
			} else if prev[j] >= cur[j-1] {
				cur[j] = prev[j]
			} else {
				cur[j] = cur[j-1]
			}
		}
		prev, cur = cur, prev
		for i := range cur {
			cur[i] = 0
		}
	}
	return prev[len(b)]
}

// BuildMaskedHint returns a masked, deterministic version of the expected
// answer. It always preserves the word count and structure of the original.
//
// Levels:
//
//	1 -> only the first letter of each word is shown: "S____ i_ i_ h__ c__"
//	2 -> first letter of each word + last letter of words longer than 3 chars
//	3 -> show first and second halves' boundary letters
//
// Words that contain only punctuation/symbols are returned unchanged. Hidden
// letters are replaced with underscores so the word length remains visible.
func BuildMaskedHint(expected string, level int) string {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return ""
	}
	if level < 1 {
		level = 1
	}
	if level > 3 {
		level = 3
	}
	words := strings.Fields(expected)
	masked := make([]string, 0, len(words))
	for _, raw := range words {
		// Detach a single leading/trailing punctuation block (for ., ?, ! etc.)
		leading, core, trailing := splitWordPunctuation(raw)
		if core == "" {
			masked = append(masked, raw)
			continue
		}
		runes := []rune(core)
		out := make([]rune, len(runes))
		for i := range runes {
			out[i] = '_'
		}
		// Always reveal first letter.
		out[0] = runes[0]
		// Apostrophes inside the word are kept visible (don't -> d___'_)
		for i, r := range runes {
			if r == '\'' || r == '-' {
				out[i] = r
			}
		}
		if level >= 2 && len(runes) > 3 {
			out[len(runes)-1] = runes[len(runes)-1]
		}
		if level >= 3 && len(runes) >= 5 {
			mid := len(runes) / 2
			out[mid] = runes[mid]
		}
		masked = append(masked, leading+string(out)+trailing)
	}
	return strings.Join(masked, " ")
}

func splitWordPunctuation(s string) (string, string, string) {
	if s == "" {
		return "", "", ""
	}
	startIdx := 0
	for startIdx < len(s) {
		r := rune(s[startIdx])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			break
		}
		startIdx++
	}
	endIdx := len(s)
	for endIdx > startIdx {
		r := rune(s[endIdx-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			break
		}
		endIdx--
	}
	if startIdx >= endIdx {
		return "", s, ""
	}
	return s[:startIdx], s[startIdx:endIdx], s[endIdx:]
}

// AnswerRevealKeywords are phrases that explicitly ask the tutor to reveal
// the correct answer (in Thai or English).
var AnswerRevealKeywords = []string{
	"เฉลย", "ขอเฉลย", "ขอคำตอบ", "บอกคำตอบ", "ตอบอะไร", "คำตอบคืออะไร",
	"ตอบให้หน่อย", "บอกหน่อย", "show answer", "show the answer",
	"give me the answer", "tell me the answer", "reveal answer", "reveal the answer",
	"what is the answer", "what's the answer",
}

// IsAnswerRevealRequest reports whether the user is asking for the answer.
func IsAnswerRevealRequest(text string) bool {
	if text == "" {
		return false
	}
	lower := strings.ToLower(text)
	for _, kw := range AnswerRevealKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

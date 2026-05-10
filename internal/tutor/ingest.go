package tutor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

type IngestService struct {
	db     *pgxpool.Pool
	logger *zap.Logger
}

func NewIngestService(db *pgxpool.Pool) *IngestService {
	return &IngestService{db: db, logger: zap.L()}
}

// IngestLessons scans lecture/ directory and imports all units
func (s *IngestService) IngestLessons(ctx context.Context, lecturePath string) (int, error) {
	entries, err := os.ReadDir(lecturePath)
	if err != nil {
		return 0, fmt.Errorf("read lecture dir: %w", err)
	}

	count := 0
	unitPattern := regexp.MustCompile(`^unit_(\d+)_`)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		matches := unitPattern.FindStringSubmatch(entry.Name())
		if len(matches) < 2 {
			continue
		}
		unitNo, _ := strconv.Atoi(matches[1])
		unitPath := filepath.Join(lecturePath, entry.Name())

		readmePath := filepath.Join(unitPath, "readme.md")
		content, err := os.ReadFile(readmePath)
		if err != nil {
			s.logger.Warn("skip unit, no readme", zap.Int("unit", unitNo), zap.Error(err))
			continue
		}

		rawContent := string(content)
		title := s.extractTitle(rawContent, entry.Name(), unitNo)
		grammarFocus := s.extractGrammarFocus(rawContent)
		summary := s.generateSummary(rawContent)
		level := s.detectLevel(unitNo)

		// Upsert lesson unit
		var unitID int
		err = s.db.QueryRow(ctx,
			`INSERT INTO lesson_units (unit_no, title, source_path, summary, grammar_focus, level, raw_content)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			ON CONFLICT (unit_no) DO UPDATE SET title = $2, summary = $4, grammar_focus = $5, raw_content = $7, updated_at = now()
			RETURNING id`,
			unitNo, title, unitPath, summary, grammarFocus, level, rawContent,
		).Scan(&unitID)
		if err != nil {
			s.logger.Error("insert unit failed", zap.Int("unit", unitNo), zap.Error(err))
			continue
		}

		// Extract and insert lesson items
		s.extractLessonItems(ctx, unitID, rawContent)
		count++
		s.logger.Info("ingested unit", zap.Int("unit", unitNo), zap.String("title", title))
	}

	return count, nil
}

func (s *IngestService) extractTitle(content string, folderName string, unitNo int) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			title := strings.TrimPrefix(line, "# ")
			title = strings.TrimPrefix(title, "บทเรียน: ")
			if len(title) > 100 {
				title = title[:100]
			}
			return title
		}
	}
	// Clean folder name
	name := strings.TrimPrefix(folderName, fmt.Sprintf("unit_%03d_", unitNo))
	name = strings.ReplaceAll(name, "_", " ")
	name = strings.ReplaceAll(name, "  ", " ")
	if len(name) > 80 {
		name = name[:80]
	}
	return name
}

func (s *IngestService) extractGrammarFocus(content string) string {
	patterns := []string{
		"present continuous", "present simple", "past simple", "past continuous",
		"present perfect", "past perfect", "future", "going to", "will",
		"modal verb", "can", "could", "must", "should", "may", "might",
		"passive", "reported speech", "conditional", "relative clause",
		"preposition", "article", "adjective", "adverb", "comparative", "superlative",
		"phrasal verb", "countable", "uncountable",
	}
	lower := strings.ToLower(content)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return p
		}
	}
	return ""
}

func (s *IngestService) generateSummary(content string) string {
	lines := strings.Split(content, "\n")
	var summary []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if len(line) > 20 && len(summary) < 3 {
			summary = append(summary, line)
		}
	}
	return strings.Join(summary, " ")
}

func (s *IngestService) detectLevel(unitNo int) string {
	if unitNo <= 40 {
		return "A1"
	}
	if unitNo <= 80 {
		return "A2"
	}
	if unitNo <= 120 {
		return "B1"
	}
	return "B2"
}

func (s *IngestService) extractLessonItems(ctx context.Context, unitID int, content string) {
	// Delete existing items for re-ingest
	s.db.Exec(ctx, `DELETE FROM lesson_items WHERE unit_id = $1`, unitID)

	lines := strings.Split(content, "\n")
	order := 0

	// Extract example sentences
	sentencePattern := regexp.MustCompile(`['']([A-Z][^'']{10,80})['']`)
	for _, line := range lines {
		matches := sentencePattern.FindAllStringSubmatch(line, -1)
		for _, m := range matches {
			if len(m) >= 2 {
				sentence := strings.TrimSpace(m[1])
				sentence = strings.ReplaceAll(sentence, "  ", " ")
				if len(sentence) > 15 && !strings.Contains(sentence, "➜") {
					order++
					s.db.Exec(ctx,
						`INSERT INTO lesson_items (id, unit_id, item_type, content, sort_order) VALUES ($1, $2, $3, $4, $5)`,
						uuid.New().String(), unitID, "example_sentence", sentence, order)

					// Also add as listening sentence
					order++
					s.db.Exec(ctx,
						`INSERT INTO lesson_items (id, unit_id, item_type, content, sort_order) VALUES ($1, $2, $3, $4, $5)`,
						uuid.New().String(), unitID, "listening_sentence", sentence, order)
				}
			}
		}
	}

	// Extract explanatory content
	var explanationParts []string
	inExplanation := false
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "## เนื้อหา") {
			inExplanation = true
			continue
		}
		if strings.HasPrefix(line, "## ") && inExplanation {
			break
		}
		if inExplanation && line != "" && len(line) > 10 {
			explanationParts = append(explanationParts, line)
		}
	}
	if len(explanationParts) > 0 {
		explanation := strings.Join(explanationParts, "\n")
		if len(explanation) > 3000 {
			explanation = explanation[:3000]
		}
		order++
		s.db.Exec(ctx,
			`INSERT INTO lesson_items (id, unit_id, item_type, content, sort_order) VALUES ($1, $2, $3, $4, $5)`,
			uuid.New().String(), unitID, "grammar_explanation", explanation, order)
	}
}

package tutor

import (
	"context"
	"encoding/json"
	"strings"
)

// RequiredPassPerSkill is the number of successful rounds the learner has to
// complete inside a single skill (listening / speaking / reading) before the
// tutor advances to the next skill. The product spec asks for at least 3.
const RequiredPassPerSkill = 3

// incrementPassCount atomically bumps resume_state.<key> in tutor_sessions and
// returns the new value. Returns 1 if anything goes wrong, which keeps the
// loop moving forward (worst case = early advancement, never stuck).
func (s *Service) incrementPassCount(ctx context.Context, sessionID, key string) int {
	if sessionID == "" || key == "" {
		return 1
	}
	var newVal int
	err := s.db.QueryRow(ctx, `
		UPDATE tutor_sessions
		SET resume_state = jsonb_set(
			COALESCE(resume_state, '{}'::jsonb),
			ARRAY[$2]::text[],
			to_jsonb(COALESCE((resume_state->>$2)::int, 0) + 1)
		), updated_at = now()
		WHERE id = $1
		RETURNING (resume_state->>$2)::int`,
		sessionID, key,
	).Scan(&newVal)
	if err != nil || newVal <= 0 {
		return 1
	}
	return newVal
}

// markItemUsed appends an item id to resume_state.<listKey> so the next round
// can avoid repeating it. Idempotent; no-op when itemID is empty.
func (s *Service) markItemUsed(ctx context.Context, sessionID, listKey, itemID string) {
	if sessionID == "" || listKey == "" || itemID == "" {
		return
	}
	// Read, mutate, write – the list is bounded by the number of items per
	// unit so this is cheap.
	var raw string
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(resume_state->$1, '[]'::jsonb)::text FROM tutor_sessions WHERE id = $2`, listKey, sessionID).Scan(&raw)
	var ids []string
	_ = json.Unmarshal([]byte(raw), &ids)
	for _, v := range ids {
		if v == itemID {
			return
		}
	}
	ids = append(ids, itemID)
	out, _ := json.Marshal(ids)
	_, _ = s.db.Exec(ctx, `
		UPDATE tutor_sessions
		SET resume_state = jsonb_set(
			COALESCE(resume_state, '{}'::jsonb),
			ARRAY[$2]::text[],
			$3::jsonb
		), updated_at = now()
		WHERE id = $1`, sessionID, listKey, string(out))
}

// getUsedItems returns ids previously consumed in this session for the given
// list key.
func (s *Service) getUsedItems(ctx context.Context, sessionID, listKey string) []string {
	var raw string
	_ = s.db.QueryRow(ctx, `SELECT COALESCE(resume_state->$1, '[]'::jsonb)::text FROM tutor_sessions WHERE id = $2`, listKey, sessionID).Scan(&raw)
	var ids []string
	_ = json.Unmarshal([]byte(raw), &ids)
	return ids
}

// pickNextListeningItem returns the next unused listening sentence for the
// session, falling back to example_sentence when listening items run out.
// Empty string is returned when nothing is available.
func (s *Service) pickNextListeningItem(ctx context.Context, sessionID string, unitID int) (string, string) {
	used := s.getUsedItems(ctx, sessionID, "usedListeningIds")
	args := []interface{}{unitID}
	exclude := ""
	if len(used) > 0 {
		// Postgres: id NOT IN (uuid,uuid,...)
		parts := make([]string, 0, len(used))
		for i, id := range used {
			args = append(args, id)
			parts = append(parts, "$"+itoa(i+2))
		}
		exclude = " AND id::text NOT IN (" + strings.Join(parts, ",") + ")"
	}

	var itemID, content string
	q := `SELECT id::text, content FROM lesson_items
	       WHERE unit_id = $1 AND item_type = 'listening_sentence' AND length(content) BETWEEN 8 AND 180 ` +
		exclude + ` ORDER BY sort_order ASC LIMIT 1`
	if err := s.db.QueryRow(ctx, q, args...).Scan(&itemID, &content); err == nil && content != "" {
		return itemID, content
	}
	// fall back to example sentences
	q2 := `SELECT id::text, content FROM lesson_items
	        WHERE unit_id = $1 AND item_type = 'example_sentence' AND length(content) BETWEEN 8 AND 180 ` +
		exclude + ` ORDER BY sort_order ASC LIMIT 1`
	if err := s.db.QueryRow(ctx, q2, args...).Scan(&itemID, &content); err == nil && content != "" {
		return itemID, content
	}
	return "", ""
}

// itoa is a tiny strconv.Itoa wrapper kept here so the SQL builder stays
// allocation-light and we don't pull strconv into the import set elsewhere.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

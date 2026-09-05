package app

import (
	"context"
	"strings"
)

// These are learning prompts, not a reveal of the saved example answer.
func reviewCue(kind, key, old, meaning, objective string) (string, string) {
	focus := "การสื่อสารให้ชัดเจน"
	labels := map[string]string{"capitalization": "การแนะนำตัวและเขียนชื่อ", "subject_verb_agreement": "ประธานและกริยาให้สอดคล้องกัน", "prepositions": "บอกเวลา สถานที่ หรือความสัมพันธ์", "missing_article": "เลือก a, an และ the", "articles": "เลือก a, an และ the", "past_tense": "เล่าเหตุการณ์ที่ผ่านมา", "word_order": "เรียงคำให้สื่อความหมาย", "verb_tense": "เลือกเวลาให้ตรงเรื่องที่เล่า", "pronunciation": "ออกเสียงให้ผู้ฟังเข้าใจ", "plural": "บอกจำนวนและรูปพหูพจน์"}
	for k, v := range labels {
		if strings.TrimPrefix(key, "mistake:") == k {
			focus = v
			break
		}
	}
	if kind == "vocabulary" {
		return "เรียกใช้คำหรือวลีในสถานการณ์ใหม่", "ลองพูดหนึ่งประโยคที่ใช้คำหรือวลีซึ่งมีความหมายว่า “" + meaning + "” เลือกสถานการณ์และรายละเอียดของคุณเองได้"
	}
	goal := strings.TrimSpace(objective)
	if goal == "" {
		goal = strings.TrimSpace(meaning)
	}
	if goal == "" && kind != "mistake" {
		goal = old
	}
	if goal == "" {
		goal = "เล่าเรื่องใกล้ตัวหนึ่งเรื่องให้ผู้ฟังเข้าใจ โดยเน้น" + focus
	}
	title := goal
	if kind == "mistake" {
		title = focus
	}
	return title, "สถานการณ์ฝึก: " + goal + "\nลองพูดกับคู่สนทนาเป็นภาษาอังกฤษ 1–2 ประโยค ใช้ชื่อ สถานที่ หรือรายละเอียดของคุณหรือข้อมูลสมมติได้ ขอให้สื่อเป้าหมายนี้ชัดเจน ไม่ต้องท่องให้ตรงตัวอย่าง"
}

// Upgrade legacy cards in place; retain IDs, keys, answers, evidence and SRS history.
func (a *App) refreshReviewCues(ctx context.Context, uid string) error {
	rows, e := a.DB.Query(ctx, `SELECT r.id::text,r.kind,r.key,r.prompt,r.meaning,coalesce(l.data->>'objective','') FROM review_items r LEFT JOIN attempts at ON at.id=r.source_attempt LEFT JOIN learning_sessions s ON s.id=at.session_id LEFT JOIN lessons l ON l.id=coalesce(s.lesson_id,CASE WHEN r.key LIKE 'pattern:%' THEN substring(r.key from 9) END) WHERE r.cue_version=0 AND ($1='' OR r.user_id::text=$1)`, uid)
	if e != nil {
		return e
	}
	type update struct{ id, title, prompt string }
	var pending []update
	for rows.Next() {
		var id, kind, key, old, meaning, goal string
		if e = rows.Scan(&id, &kind, &key, &old, &meaning, &goal); e != nil {
			rows.Close()
			return e
		}
		title, prompt := reviewCue(kind, key, old, meaning, goal)
		pending = append(pending, update{id, title, prompt})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, v := range pending {
		if _, e = a.DB.Exec(ctx, "UPDATE review_items SET title=$1,prompt=$2,cue_version=1 WHERE id=$3 AND cue_version=0", v.title, v.prompt, v.id); e != nil {
			return e
		}
	}
	return nil
}

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"time"
	"tokoloop/internal/gemini"
)

func (a *App) reserve(ctx context.Context, uid, sid, role string, amount float64) (string, error) {
	if a.Cfg.GeminiKey == "" {
		return "", fmt.Errorf("ยังไม่ได้ตั้งค่า Gemini API key")
	}
	tx, e := a.DB.Begin(ctx)
	if e != nil {
		return "", e
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7214904)"); e != nil {
		return "", e
	}
	var budget, spent, total float64
	if e = tx.QueryRow(ctx, "SELECT coalesce((profile->>'monthly_budget')::numeric,$2) FROM users WHERE id=$1", uid, a.Cfg.MonthlyBudget).Scan(&budget); e != nil {
		return "", e
	}
	e = tx.QueryRow(ctx, `SELECT coalesce(sum(CASE WHEN status='reserved' THEN reserved_thb ELSE cost_thb END) FILTER(WHERE user_id=$1),0),coalesce(sum(CASE WHEN status='reserved' THEN reserved_thb ELSE cost_thb END),0) FROM usage WHERE created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'`, uid).Scan(&spent, &total)
	if e != nil {
		return "", e
	}
	if spent+amount > budget || total+amount > 1000 {
		return "", fmt.Errorf("วงเงิน AI ไม่พอสำหรับกิจกรรมนี้ ยังฟังเสียงที่บันทึกไว้และดูประวัติได้")
	}
	id := uuid.NewString()
	var session any
	if sid != "" {
		session = sid
	}
	_, e = tx.Exec(ctx, "INSERT INTO usage(id,user_id,session_id,role,model,reserved_thb,price_snapshot) VALUES($1,$2,$3,$4,$5,$6,$7)", id, uid, session, role, a.Cfg.Models[role].ID, amount, asJSON(map[string]any{"model": a.Cfg.Models[role], "usd_thb": a.Cfg.USDTHB, "effective_date": a.Cfg.PriceDate}))
	if e != nil {
		return "", e
	}
	if e = tx.Commit(ctx); e != nil {
		return "", e
	}
	return id, nil
}
func (a *App) settle(id, role string, r gemini.Result, callErr error, seconds int) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	model := a.Cfg.Models[role]
	for _, m := range a.Cfg.Models {
		if m.ID == r.Model {
			model = m
		}
	}
	cost := r.Usage.Cost(model, a.Cfg.USDTHB)
	status := "complete"
	if r.Rejected {
		_, _ = a.DB.Exec(ctx, "UPDATE usage SET status='rejected',cost_thb=0,latency_ms=$2 WHERE id=$1", id, r.Latency)
		return
	}
	if r.Usage.Input == 0 && r.Usage.Output == 0 {
		status = "estimated"
		if role == "tts" && r.EstimatedUsage.Output > 0 {
			estimate := r.EstimatedUsage.Cost(model, a.Cfg.USDTHB)
			_, _ = a.DB.Exec(ctx, "UPDATE usage SET status='estimated',cost_thb=least(reserved_thb,$2),latency_ms=$3 WHERE id=$1", id, estimate, r.Latency)
			return
		}
		if role == "live" && seconds > 0 {
			m := a.Cfg.Models[role]
			estimate := (float64(seconds)*32*(m.AudioInput+m.AudioOutput) + 3000*m.Input) / 1e6 * a.Cfg.USDTHB
			_, _ = a.DB.Exec(ctx, "UPDATE usage SET status='estimated',cost_thb=least(reserved_thb,$2),latency_ms=$3,live_seconds=$4 WHERE id=$1", id, estimate, r.Latency, seconds)
			return
		}
		_, e := a.DB.Exec(ctx, "UPDATE usage SET status=$2,cost_thb=reserved_thb,latency_ms=$3,live_seconds=$4 WHERE id=$1", id, status, r.Latency, seconds)
		if e != nil {
			return
		}
		return
	}
	_, _ = a.DB.Exec(ctx, "UPDATE usage SET status=$2,cost_thb=$3,input_tokens=$4,output_tokens=$5,audio_input_tokens=$6,audio_output_tokens=$7,latency_ms=$8,live_seconds=$9,model=$10 WHERE id=$1", id, status, cost, r.Usage.Input, r.Usage.Output+r.Usage.Thinking, r.Usage.AudioInput(), r.Usage.AudioOutput(), r.Latency, seconds, r.Model)
}
func (a *App) usage(c *fiber.Ctx) error {
	var raw []byte
	e := a.DB.QueryRow(c.UserContext(), `SELECT jsonb_build_object('spent',coalesce(sum(CASE WHEN status='reserved' THEN reserved_thb ELSE cost_thb END),0),'live_seconds',coalesce(sum(live_seconds) FILTER(WHERE created_at >= date_trunc('day',now() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'),0),'calls',count(*),'by_role',coalesce((SELECT jsonb_object_agg(role,cost) FROM(SELECT role,sum(cost_thb) cost FROM usage WHERE user_id=$1 AND created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok' GROUP BY role) r),'{}'::jsonb)) FROM usage WHERE user_id=$1 AND created_at >= date_trunc('month',now() AT TIME ZONE 'Asia/Bangkok') AT TIME ZONE 'Asia/Bangkok'`, user(c).ID).Scan(&raw)
	if e != nil {
		return e
	}
	var v map[string]any
	json.Unmarshal(raw, &v)
	v["budget"] = number(user(c).Profile["monthly_budget"], a.Cfg.MonthlyBudget)
	v["warning"] = number(v["spent"], 0) >= number(v["budget"], 600)*.8
	v["price_date"] = a.Cfg.PriceDate
	v["estimated"] = true
	v["ai_configured"] = a.Cfg.GeminiKey != ""
	return c.JSON(v)
}

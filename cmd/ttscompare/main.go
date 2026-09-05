package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"tokoloop/internal/config"
	"tokoloop/internal/gemini"
)

func main() {
	cfg, e := config.Load()
	if e != nil {
		panic(e)
	}
	cfg.TimeoutSeconds = 75
	ai := gemini.New(cfg)
	out := []map[string]any{}
	for _, id := range []string{"gemini-2.5-flash-preview-tts", "gemini-3.1-flash-tts-preview"} {
		m := cfg.Models["tts"]
		m.ID = id
		m.Temperature = 1
		m.MaxTokens = 2048
		if id == "gemini-3.1-flash-tts-preview" {
			m.Input = 1
			m.Output = 20
			m.AudioOutput = 20
		}
		r, e := ai.Generate(context.Background(), m, "", "Say clearly in a friendly voice: Hello. My name is Maya. I work as a developer. I am from Bangkok.", nil, "", nil, "Kore")
		entry := map[string]any{"model": id, "latency_ms": r.Latency, "audio_bytes": len(r.Audio), "cost_thb": r.Usage.Cost(m, cfg.USDTHB), "error": fmt.Sprint(e)}
		out = append(out, entry)
		fmt.Printf("%s latency=%dms audio=%d bytes error=%v\n", id, r.Latency, len(r.Audio), e)
	}
	os.MkdirAll("reports", 0755)
	b, _ := json.MarshalIndent(out, "", "  ")
	os.WriteFile("reports/tts-comparison.json", append(b, '\n'), 0644)
}

package main

//
//import (
//	"bytes"
//	"context"
//	"encoding/json"
//	"fmt"
//	"io"
//	"net/http"
//	"os"
//
//	"gopkg.in/yaml.v2"
//)
//
//type Config struct {
//	Gemini struct {
//		APIKey string `yaml:"api_key"`
//	} `yaml:"Gemini"`
//}
//
//func testmain() {
//	data, _ := os.ReadFile("config/config.yaml")
//	var cfg Config
//	yaml.Unmarshal(data, &cfg)
//
//	models := []string{
//		"gemini-3.1-flash-tts-preview",
//		"gemini-2.5-flash-preview-tts",
//	}
//
//	for _, model := range models {
//		fmt.Printf("Testing model: %s\n", model)
//		url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, cfg.Gemini.APIKey)
//
//		body := map[string]interface{}{
//			"contents": []map[string]interface{}{
//				{
//					"role": "user",
//					"parts": []map[string]interface{}{
//						{"text": "Hello, this is a test."},
//					},
//				},
//			},
//			"generationConfig": map[string]interface{}{
//				"responseModalities": []string{"AUDIO"},
//				"speechConfig": map[string]interface{}{
//					"voiceConfig": map[string]interface{}{
//						"prebuiltVoiceConfig": map[string]interface{}{
//							"voiceName": "Puck",
//						},
//					},
//				},
//			},
//		}
//
//		jsonBody, _ := json.Marshal(body)
//		httpReq, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(jsonBody))
//		httpReq.Header.Set("Content-Type", "application/json")
//
//		resp, err := http.DefaultClient.Do(httpReq)
//		if err != nil {
//			fmt.Printf("HTTP Error: %v\n", err)
//			continue
//		}
//
//		respBody, _ := io.ReadAll(resp.Body)
//		resp.Body.Close()
//
//		if resp.StatusCode != 200 {
//			fmt.Printf("Status %d: %s\n\n", resp.StatusCode, string(respBody))
//		} else {
//			fmt.Printf("Success!\n")
//			break
//		}
//	}
//}

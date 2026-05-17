package tutor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

type lineNotificationPayload struct {
	Message     string `json:"message"`
	UserID      string `json:"userId"`
	MessageType string `json:"messageType"`
	IndexBot    int    `json:"indexBot"`
}

func SendLineNotification(ctx context.Context, client *http.Client, notifyURL, userID, message string) error {
	if notifyURL == "" || userID == "" || message == "" {
		return nil
	}
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}

	body, err := json.Marshal(lineNotificationPayload{
		Message:     message,
		UserID:      userID,
		MessageType: "text",
		IndexBot:    0,
	})
	if err != nil {
		return fmt.Errorf("line notify marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, notifyURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("line notify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("requestId", uuid.New().String())

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("line notify post: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("line notify status: %d", resp.StatusCode)
	}
	return nil
}

func (s *Service) NotifyLineAsync(message string) {
	if s == nil || s.cfg == nil {
		return
	}
	notifyURL := s.cfg.Line.NotifyURL
	userID := ""
	if len(s.cfg.Line.AllowedUserIDs) > 0 {
		userID = s.cfg.Line.AllowedUserIDs[0]
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := SendLineNotification(ctx, nil, notifyURL, userID, message); err != nil {
			s.logger.Warn("line notification failed", zap.Error(err))
		}
	}()
}

package tutor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendLineNotification(t *testing.T) {
	var gotPayload lineNotificationPayload
	var gotRequestID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		gotRequestID = r.Header.Get("requestId")
		if gotRequestID == "" {
			t.Fatalf("requestId header is required")
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("content-type = %s", r.Header.Get("Content-Type"))
		}
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := SendLineNotification(context.Background(), server.Client(), server.URL, "Ufa2dce3870921d4d4604bf39024bab7f", "hello")
	if err != nil {
		t.Fatalf("SendLineNotification returned error: %v", err)
	}
	if gotPayload.Message != "hello" || gotPayload.UserID != "Ufa2dce3870921d4d4604bf39024bab7f" || gotPayload.MessageType != "text" || gotPayload.IndexBot != 0 {
		t.Fatalf("unexpected payload: %+v", gotPayload)
	}
}

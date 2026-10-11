package infrai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRevokeRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.URL.Path != "/auth/session/revoke" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 1 || body["session_id"] != "session-old" {
			t.Fatalf("body = %#v", body)
		}
		fmt.Fprint(w, `{"ok":true,"data":null,"error":null,"metadata":{}}`)
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	if err := client.Revoke(context.Background(), "session-old"); err != nil {
		t.Fatal(err)
	}
}

func TestRefreshRequestAndRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("authorization header missing")
		}
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"ok":false,"data":null,"error":{"code":"RATE_LIMITED","message":"retry later"},"metadata":{}}`)
			return
		}
		fmt.Fprint(w, `{"ok":true,"data":{"session_id":"session-new","refresh_token":"refresh-new"},"error":null,"metadata":{}}`)
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	client.Sleep = func(context.Context, time.Duration) error { return nil }

	session, err := client.Refresh(context.Background(), "refresh-old")
	if err != nil {
		t.Fatal(err)
	}
	if requests != 2 || session.SessionID != "session-new" || session.RefreshToken != "refresh-new" {
		t.Fatalf("requests=%d session=%+v", requests, session)
	}
}

func TestEnvelopeErrorPrecedesHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"ok":false,"data":null,"error":{"code":"INVALID_ARGUMENT","message":"refresh token required"},"metadata":{}}`)
	}))
	defer server.Close()

	client := NewClient("test-key")
	client.BaseURL = server.URL
	_, err := client.Refresh(context.Background(), "")
	apiErr, ok := err.(*Error)
	if !ok || apiErr.Code != "INVALID_ARGUMENT" || apiErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("error = %#v", err)
	}
}

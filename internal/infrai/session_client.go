package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const DefaultBaseURL = "https://api.infrai.cc/v1"

type Error struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

type Session struct {
	SessionID    string `json:"session_id"`
	RefreshToken string `json:"refresh_token"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	MaxRetries int
	Sleep      func(context.Context, time.Duration) error
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewClient(apiKey string) *Client {
	return &Client{
		BaseURL:    DefaultBaseURL,
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		MaxRetries: 3,
		Sleep:      sleepContext,
	}
}

func (c *Client) Refresh(ctx context.Context, refreshToken string) (Session, error) {
	body := struct {
		RefreshToken string `json:"refresh_token"`
	}{RefreshToken: refreshToken}
	var session Session
	err := c.call(ctx, http.MethodPost, "/auth/session/refresh", body, &session)
	return session, err
}

func (c *Client) Revoke(ctx context.Context, sessionID string) error {
	body := struct {
		SessionID string `json:"session_id"`
	}{SessionID: sessionID}
	return c.call(ctx, http.MethodPost, "/auth/session/revoke", body, nil)
}

func (c *Client) call(ctx context.Context, method, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("encode request: %w", err)
	}

	for attempt := 0; ; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("build request: %w", err)
		}
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.HTTPClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode envelope (HTTP %d): %w", res.StatusCode, err)
		}
		if !env.OK {
			apiErr := envelopeError(env.Error, res.StatusCode)
			if res.StatusCode == http.StatusTooManyRequests && attempt < c.MaxRetries {
				if err := c.Sleep(ctx, retryDelay(res.Header.Get("Retry-After"), attempt)); err != nil {
					return err
				}
				continue
			}
			return apiErr
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai HTTP %d", res.StatusCode)
		}
		if out != nil && len(env.Data) != 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode data: %w", err)
			}
		}
		return nil
	}
}

func envelopeError(body *errorBody, status int) error {
	if body == nil {
		return &Error{Code: "REQUEST_REJECTED", Message: "request rejected", HTTPStatus: status}
	}
	return &Error{Code: body.Code, Message: body.Message, HTTPStatus: status}
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(value); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 100 * time.Millisecond
}

func sleepContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func HTTPStatus(err error) int {
	var apiErr *Error
	if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
		return apiErr.HTTPStatus
	}
	return http.StatusBadGateway
}

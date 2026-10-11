package fieldservice

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/infrai-examples/field-session-rotation/internal/infrai"
)

const DispatchCompleted = "completed"

var ErrCompletionPhotoRequired = errors.New("a completed dispatch requires a work-order photo")

type SessionAPI interface {
	Refresh(context.Context, string) (infrai.Session, error)
	Revoke(context.Context, string) error
}

type FollowUp struct {
	WorkOrderID    string `json:"work_order_id"`
	TechnicianID   string `json:"technician_id"`
	PhotoRef       string `json:"photo_ref"`
	DispatchStatus string `json:"dispatch_status"`
	RefreshToken   string `json:"refresh_token"`
}

type Result struct {
	WorkOrderID    string    `json:"work_order_id"`
	DispatchStatus string    `json:"dispatch_status"`
	SessionID      string    `json:"session_id"`
	RefreshToken   string    `json:"refresh_token"`
	RecordedAt     time.Time `json:"recorded_at"`
}

type Service struct {
	sessions SessionAPI
	now      func() time.Time
	mu       sync.RWMutex
	orders   map[string]Result
}

func NewService(sessions SessionAPI) *Service {
	return &Service{sessions: sessions, now: time.Now, orders: make(map[string]Result)}
}

func (s *Service) RecordFollowUp(ctx context.Context, input FollowUp) (Result, error) {
	if input.DispatchStatus == DispatchCompleted && input.PhotoRef == "" {
		return Result{}, ErrCompletionPhotoRequired
	}
	session, err := s.sessions.Refresh(ctx, input.RefreshToken)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		WorkOrderID:    input.WorkOrderID,
		DispatchStatus: input.DispatchStatus,
		SessionID:      session.SessionID,
		RefreshToken:   session.RefreshToken,
		RecordedAt:     s.now().UTC(),
	}
	s.mu.Lock()
	s.orders[input.WorkOrderID] = result
	s.mu.Unlock()
	return result, nil
}

func (s *Service) RevokeSession(ctx context.Context, sessionID string) error {
	return s.sessions.Revoke(ctx, sessionID)
}

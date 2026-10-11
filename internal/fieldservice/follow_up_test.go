package fieldservice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/infrai-examples/field-session-rotation/internal/infrai"
)

type fakeSessions struct {
	refreshCalls int
}

func (f *fakeSessions) Refresh(context.Context, string) (infrai.Session, error) {
	f.refreshCalls++
	return infrai.Session{SessionID: "session-new", RefreshToken: "refresh-new"}, nil
}

func (f *fakeSessions) Revoke(context.Context, string) error { return nil }

func TestRecordFollowUpDecision(t *testing.T) {
	tests := []struct {
		name          string
		input         FollowUp
		wantErr       error
		wantRefreshes int
	}{
		{
			name:          "completed dispatch rotates session and records photo",
			input:         FollowUp{WorkOrderID: "wo-1042", TechnicianID: "tech-7", PhotoRef: "photo-88", DispatchStatus: DispatchCompleted, RefreshToken: "refresh-old"},
			wantRefreshes: 1,
		},
		{
			name:    "completed dispatch without photo is rejected before rotation",
			input:   FollowUp{WorkOrderID: "wo-1042", TechnicianID: "tech-7", DispatchStatus: DispatchCompleted, RefreshToken: "refresh-old"},
			wantErr: ErrCompletionPhotoRequired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sessions := &fakeSessions{}
			service := NewService(sessions)
			service.now = func() time.Time { return time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC) }
			result, err := service.RecordFollowUp(context.Background(), tt.input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
			if sessions.refreshCalls != tt.wantRefreshes {
				t.Fatalf("refresh calls = %d, want %d", sessions.refreshCalls, tt.wantRefreshes)
			}
			if tt.wantErr == nil && (result.RefreshToken != "refresh-new" || result.DispatchStatus != DispatchCompleted) {
				t.Fatalf("result = %+v", result)
			}
		})
	}
}

package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/infrai-examples/field-session-rotation/internal/fieldservice"
	"github.com/infrai-examples/field-session-rotation/internal/infrai"
)

type server struct {
	service *fieldservice.Service
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	address := os.Getenv("LISTEN_ADDR")
	if address == "" {
		address = ":8080"
	}

	app := &server{service: fieldservice.NewService(infrai.NewClient(apiKey))}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /work-orders/follow-up", app.followUp)
	mux.HandleFunc("POST /sessions/{session_id}/revoke", app.revoke)
	log.Printf("field-session listening on %s", address)
	log.Fatal(http.ListenAndServe(address, mux))
}

func (s *server) followUp(w http.ResponseWriter, r *http.Request) {
	var input fieldservice.FollowUp
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	result, err := s.service.RecordFollowUp(r.Context(), input)
	if err != nil {
		status := infrai.HTTPStatus(err)
		if errors.Is(err, fieldservice.ErrCompletionPhotoRequired) {
			status = http.StatusUnprocessableEntity
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *server) revoke(w http.ResponseWriter, r *http.Request) {
	sessionID := strings.TrimSpace(r.PathValue("session_id"))
	if sessionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "session_id is required"})
		return
	}
	if err := s.service.RevokeSession(r.Context(), sessionID); err != nil {
		writeJSON(w, infrai.HTTPStatus(err), map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"revoked": true})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("write response: %v", err)
	}
}

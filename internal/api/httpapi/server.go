package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sandeepkv93/go-saga-lab/internal/domain"
	"github.com/sandeepkv93/go-saga-lab/internal/orchestrator/runtime"
	"github.com/sandeepkv93/go-saga-lab/internal/store"
	pgstore "github.com/sandeepkv93/go-saga-lab/internal/store/postgres"
	"github.com/sandeepkv93/go-saga-lab/internal/telemetry"
)

type Server struct {
	repository store.SagaRepository
	runtime    *runtime.Service
	mux        *http.ServeMux
}

const traceIDHeader = "X-Trace-Id"

type createSagaRequest struct {
	TemplateID     string          `json:"template_id"`
	IdempotencyKey string          `json:"idempotency_key"`
	Input          json.RawMessage `json:"input"`
}

type stepResultRequest struct {
	Succeeded bool `json:"succeeded"`
}

type sagaResponse struct {
	ID             string            `json:"id"`
	TemplateID     string            `json:"template_id"`
	Status         domain.SagaStatus `json:"status"`
	IdempotencyKey string            `json:"idempotency_key"`
	Input          json.RawMessage   `json:"input"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func NewServer(repository store.SagaRepository, runtimeSvc *runtime.Service) (*Server, error) {
	if repository == nil {
		return nil, errors.New("repository is required")
	}
	if runtimeSvc == nil {
		return nil, errors.New("runtime service is required")
	}

	server := &Server{
		repository: repository,
		runtime:    runtimeSvc,
		mux:        http.NewServeMux(),
	}
	server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", telemetry.InstrumentHTTP("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "go-saga-lab-api",
		})
	}))
	s.mux.Handle("/metrics", telemetry.MetricsHandler())
	s.mux.HandleFunc("/v1/sagas", telemetry.InstrumentHTTP("/v1/sagas", s.handleSagas))
	s.mux.HandleFunc("/v1/sagas/", telemetry.InstrumentHTTP("/v1/sagas/:id", s.handleSagaByID))
}

func (s *Server) handleSagas(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		s.createSaga(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleSagaByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/sagas/")
	path = strings.Trim(path, "/")
	if path == "" {
		http.NotFound(w, r)
		return
	}

	parts := strings.Split(path, "/")
	id := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		s.getSaga(w, r, id)
	case r.Method == http.MethodPost && action == "start":
		s.startSaga(w, r, id)
	case r.Method == http.MethodPost && action == "cancel":
		s.cancelSaga(w, r, id)
	case r.Method == http.MethodPost && action == "step-result":
		s.stepResult(w, r, id)
	case r.Method == http.MethodPost && action == "compensation-result":
		s.compensationResult(w, r, id)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) createSaga(w http.ResponseWriter, r *http.Request) {
	var req createSagaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	if req.TemplateID == "" || req.IdempotencyKey == "" {
		http.Error(w, "template_id and idempotency_key are required", http.StatusBadRequest)
		return
	}
	if len(req.Input) == 0 {
		req.Input = json.RawMessage(`{}`)
	}

	now := time.Now().UTC()
	traceID := requestTraceID(r)
	w.Header().Set(traceIDHeader, traceID)
	instance := domain.SagaInstance{
		ID:             newSagaID(),
		TemplateID:     req.TemplateID,
		Status:         domain.SagaStatusCreated,
		InputJSON:      req.Input,
		IdempotencyKey: req.IdempotencyKey,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	event := domain.OutboxEvent{
		AggregateType: "saga",
		AggregateID:   instance.ID,
		EventType:     "saga.created",
		PayloadJSON:   mustMarshalJSON(map[string]any{"saga_id": instance.ID, "template_id": instance.TemplateID, "trace_id": traceID}),
		DedupeKey:     instance.IdempotencyKey + ":saga.created",
		TraceID:       traceID,
		Status:        "pending",
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	var err error
	if outboxRepo, ok := s.repository.(store.SagaOutboxRepository); ok {
		err = outboxRepo.CreateSagaInstanceWithOutbox(r.Context(), instance, event)
	} else {
		err = s.repository.CreateSagaInstance(r.Context(), instance)
	}
	if err != nil {
		http.Error(w, fmt.Sprintf("create saga: %v", err), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusCreated, toSagaResponse(instance))
}

func (s *Server) getSaga(w http.ResponseWriter, r *http.Request, id string) {
	instance, err := s.repository.GetSagaInstance(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, toSagaResponse(instance))
}

func (s *Server) startSaga(w http.ResponseWriter, r *http.Request, id string) {
	status, err := s.runtime.StartSaga(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	instance, err := s.repository.GetSagaInstance(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	instance.Status = status
	writeJSON(w, http.StatusOK, toSagaResponse(instance))
}

func (s *Server) cancelSaga(w http.ResponseWriter, r *http.Request, id string) {
	status, err := s.runtime.CancelSaga(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	instance, err := s.repository.GetSagaInstance(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	instance.Status = status
	writeJSON(w, http.StatusOK, toSagaResponse(instance))
}

func (s *Server) stepResult(w http.ResponseWriter, r *http.Request, id string) {
	var req stepResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	status, err := s.runtime.HandleStepResult(r.Context(), id, req.Succeeded)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	instance, err := s.repository.GetSagaInstance(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	instance.Status = status
	writeJSON(w, http.StatusOK, toSagaResponse(instance))
}

func (s *Server) compensationResult(w http.ResponseWriter, r *http.Request, id string) {
	var req stepResultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	status, err := s.runtime.CompleteCompensation(r.Context(), id, req.Succeeded)
	if err != nil {
		writeDomainError(w, err)
		return
	}

	instance, err := s.repository.GetSagaInstance(r.Context(), id)
	if err != nil {
		writeDomainError(w, err)
		return
	}
	instance.Status = status
	writeJSON(w, http.StatusOK, toSagaResponse(instance))
}

func toSagaResponse(instance domain.SagaInstance) sagaResponse {
	return sagaResponse{
		ID:             instance.ID,
		TemplateID:     instance.TemplateID,
		Status:         instance.Status,
		IdempotencyKey: instance.IdempotencyKey,
		Input:          json.RawMessage(instance.InputJSON),
		CreatedAt:      instance.CreatedAt,
		UpdatedAt:      instance.UpdatedAt,
	}
}

func writeJSON(w http.ResponseWriter, statusCode int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeDomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pgstore.ErrSagaNotFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case strings.Contains(err.Error(), "not found"):
		http.Error(w, err.Error(), http.StatusNotFound)
	case strings.Contains(err.Error(), "illegal transition"):
		http.Error(w, err.Error(), http.StatusConflict)
	case strings.Contains(err.Error(), "no transitions configured"):
		http.Error(w, err.Error(), http.StatusConflict)
	default:
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func newSagaID() string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return "saga_" + hex.EncodeToString(raw[:])
}

func requestTraceID(r *http.Request) string {
	if traceID := strings.TrimSpace(r.Header.Get(traceIDHeader)); traceID != "" {
		return traceID
	}
	return "trace_" + randomHex(8)
}

func randomHex(n int) string {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		panic(err)
	}
	return hex.EncodeToString(raw)
}

func mustMarshalJSON(payload any) []byte {
	raw, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return raw
}

func NewDefaultServer(ctx context.Context, repository store.SagaRepository) (*Server, error) {
	_ = ctx

	runtimeSvc, err := runtime.NewService(repository)
	if err != nil {
		return nil, err
	}

	return NewServer(repository, runtimeSvc)
}

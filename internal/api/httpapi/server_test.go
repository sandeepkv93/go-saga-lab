package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sandeepkv93/go-saga-lab/internal/orchestrator/runtime"
	"github.com/sandeepkv93/go-saga-lab/internal/store/memory"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()

	repo := memory.New()
	server, err := NewDefaultServer(context.Background(), repo)
	if err != nil {
		t.Fatalf("NewDefaultServer() error = %v", err)
	}
	return server
}

func TestCreateAndGetSaga(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	body := bytes.NewBufferString(`{"template_id":"order-flow","idempotency_key":"idem-1","input":{"order_id":"o-1"}}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/sagas", body)
	rec := httptest.NewRecorder()

	server.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST /v1/sagas status = %d, want %d", rec.Code, http.StatusCreated)
	}

	var created sagaResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/sagas/"+created.ID, nil)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)

	if getRec.Code != http.StatusOK {
		t.Fatalf("GET /v1/sagas/{id} status = %d, want %d", getRec.Code, http.StatusOK)
	}
}

func TestStartSaga(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	body := bytes.NewBufferString(`{"template_id":"order-flow","idempotency_key":"idem-2","input":{"order_id":"o-2"}}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/sagas", body)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	var created sagaResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)

	if startRec.Code != http.StatusOK {
		t.Fatalf("POST /v1/sagas/{id}/start status = %d, want %d", startRec.Code, http.StatusOK)
	}

	var started sagaResponse
	if err := json.Unmarshal(startRec.Body.Bytes(), &started); err != nil {
		t.Fatalf("json.Unmarshal(start) error = %v", err)
	}
	if started.Status != "running" {
		t.Fatalf("started.Status = %q, want %q", started.Status, "running")
	}
}

func TestCancelCompletedSagaReturnsConflict(t *testing.T) {
	t.Parallel()

	repo := memory.New()
	runtimeSvc, err := runtime.NewService(repo)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	server, err := NewServer(repo, runtimeSvc)
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	body := bytes.NewBufferString(`{"template_id":"order-flow","idempotency_key":"idem-3","input":{"order_id":"o-3"}}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/sagas", body)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	var created sagaResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)
	if err := repo.UpdateSagaStatus(context.Background(), created.ID, "completed"); err != nil {
		t.Fatalf("UpdateSagaStatus() error = %v", err)
	}

	cancelReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/cancel", nil)
	cancelRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(cancelRec, cancelReq)

	if cancelRec.Code != http.StatusConflict {
		t.Fatalf("POST /v1/sagas/{id}/cancel status = %d, want %d", cancelRec.Code, http.StatusConflict)
	}
}

func TestStepResultCompletesSaga(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	body := bytes.NewBufferString(`{"template_id":"order-flow","idempotency_key":"idem-4","input":{"order_id":"o-4"}}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/sagas", body)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	var created sagaResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)

	stepReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/step-result", bytes.NewBufferString(`{"succeeded":true}`))
	stepRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(stepRec, stepReq)

	if stepRec.Code != http.StatusOK {
		t.Fatalf("POST /v1/sagas/{id}/step-result status = %d, want %d", stepRec.Code, http.StatusOK)
	}

	var completed sagaResponse
	if err := json.Unmarshal(stepRec.Body.Bytes(), &completed); err != nil {
		t.Fatalf("json.Unmarshal(step-result) error = %v", err)
	}
	if completed.Status != "completed" {
		t.Fatalf("completed.Status = %q, want %q", completed.Status, "completed")
	}
}

func TestCompensationResultCancelsSaga(t *testing.T) {
	t.Parallel()

	server := newTestServer(t)

	body := bytes.NewBufferString(`{"template_id":"order-flow","idempotency_key":"idem-5","input":{"order_id":"o-5"}}`)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/sagas", body)
	createRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(createRec, createReq)

	var created sagaResponse
	if err := json.Unmarshal(createRec.Body.Bytes(), &created); err != nil {
		t.Fatalf("json.Unmarshal(create) error = %v", err)
	}

	startReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/start", nil)
	startRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(startRec, startReq)

	failReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/step-result", bytes.NewBufferString(`{"succeeded":false}`))
	failRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(failRec, failReq)

	compReq := httptest.NewRequest(http.MethodPost, "/v1/sagas/"+created.ID+"/compensation-result", bytes.NewBufferString(`{"succeeded":true}`))
	compRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(compRec, compReq)

	if compRec.Code != http.StatusOK {
		t.Fatalf("POST /v1/sagas/{id}/compensation-result status = %d, want %d", compRec.Code, http.StatusOK)
	}

	var cancelled sagaResponse
	if err := json.Unmarshal(compRec.Body.Bytes(), &cancelled); err != nil {
		t.Fatalf("json.Unmarshal(compensation-result) error = %v", err)
	}
	if cancelled.Status != "cancelled" {
		t.Fatalf("cancelled.Status = %q, want %q", cancelled.Status, "cancelled")
	}
}

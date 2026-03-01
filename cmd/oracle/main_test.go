package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOracleServer_Health(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	eval := &CombinedEvaluator{Static: se}
	srv := newOracleServer(eval, "static")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "ok", body["status"])
	require.Equal(t, "static", body["tier"])
}

func TestOracleServer_Evaluate(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	eval := &CombinedEvaluator{Static: se}
	srv := newOracleServer(eval, "static")

	payload := `{"claim":"Electromagnetic waves propagate at the speed of light in vacuum","domain":"physics","claim_type":"fact"}`
	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp EvaluateResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Verdict)
	require.NotEmpty(t, resp.Reasoning)
}

func TestOracleServer_EvaluateBadRequest(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	eval := &CombinedEvaluator{Static: se}
	srv := newOracleServer(eval, "static")

	req := httptest.NewRequest(http.MethodPost, "/evaluate", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestOracleServer_Prefetch(t *testing.T) {
	se, err := NewStaticEvaluator()
	require.NoError(t, err)

	eval := &CombinedEvaluator{Static: se}
	srv := newOracleServer(eval, "static")

	payload := `{"claim":"Water boils at 100 degrees Celsius at sea level","domain":"physics","claim_type":"fact"}`
	req := httptest.NewRequest(http.MethodPost, "/prefetch", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)

	require.Equal(t, http.StatusAccepted, rec.Code)

	var body map[string]string
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.Equal(t, "prefetching", body["status"])
}

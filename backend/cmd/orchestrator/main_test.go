package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Y-vQv-Y/DevLoom/backend/pkg/taskflow"
)

func TestRegisterRunnerExchangesInstallTokenForSessionCredential(t *testing.T) {
	center := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/register" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer install-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var req registration
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.MachineID != "machine-1" || req.Host.MachineID != "machine-1" || req.Endpoint != "http://runner.test:8890" {
			t.Fatalf("registration = %#v", req)
		}
		_ = json.NewEncoder(w).Encode(taskflow.Resp[registrationResponse]{
			Data: registrationResponse{Registered: true, HostID: "host-1", Credential: "session-credential"},
		})
	}))
	defer center.Close()

	result, err := registerRunner(context.Background(), center.URL, "install-token", "http://runner.test:8890", "machine-1", "machine-1", "worker", "192.0.2.10")
	if err != nil {
		t.Fatalf("registerRunner() error = %v", err)
	}
	if result.HostID != "host-1" || result.Credential != "session-credential" {
		t.Fatalf("result = %#v", result)
	}
}

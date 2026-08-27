package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMethodSwitchTreatsHeadAsRead(t *testing.T) {
	called := false
	handler := methodSwitch(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}, nil)
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodHead, "/", nil))
	if !called || recorder.Code != http.StatusNoContent {
		t.Fatalf("HEAD dispatch called=%t status=%d", called, recorder.Code)
	}
}

func TestMethodSwitchAdvertisesAllowedMethods(t *testing.T) {
	handler := methodSwitch(nil, func(http.ResponseWriter, *http.Request) {})
	recorder := httptest.NewRecorder()
	handler(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusMethodNotAllowed || recorder.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("method switch response = %d Allow=%q", recorder.Code, recorder.Header().Get("Allow"))
	}
}

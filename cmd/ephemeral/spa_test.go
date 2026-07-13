package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSPAHandlerServesIndexWithoutCaching(t *testing.T) {
	recorder := httptest.NewRecorder()
	spaHandler([]byte("<main>SPA</main>"))(recorder, httptest.NewRequest(http.MethodGet, "/history", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-cache" {
		t.Fatalf("Cache-Control = %q", got)
	}
	if got := recorder.Body.String(); got != "<main>SPA</main>" {
		t.Fatalf("body = %q", got)
	}
}

func TestImmutableAssetsAddsLongLivedCacheHeader(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := immutableAssets(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/assets/app-hash.js", nil))

	if got := recorder.Header().Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("Cache-Control = %q", got)
	}
}

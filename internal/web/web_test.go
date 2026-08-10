package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerDoesNotFallbackForMissingStaticAsset(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/assets/missing-module.js", nil)
	w := httptest.NewRecorder()

	Handler().ServeHTTP(w, r)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
	if strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("missing asset was served as HTML: %q", w.Header().Get("Content-Type"))
	}
}

func TestHandlerFallsBackForClientSideRoute(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/providers/42", nil)
	w := httptest.NewRecorder()

	Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}
	if !strings.HasPrefix(w.Header().Get("Content-Type"), "text/html") {
		t.Fatalf("content type = %q, want text/html", w.Header().Get("Content-Type"))
	}
}

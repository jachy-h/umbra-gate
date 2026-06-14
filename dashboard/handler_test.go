package dashboard

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anomalyco/llm-gateway/db"
)

func TestHomeRendersIconStatsAndUsageBreakdowns(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "router.db"))
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer database.Close()

	handler := New(database)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, `<h1 style="margin-bottom:24px;font-size:24px;">Sessions</h1>`) || strings.Contains(body, `<h1 style="margin-bottom:24px;font-size:24px;">Models</h1>`) {
		t.Fatalf("home rendered the wrong page content: %s", body)
	}
	for _, want := range []string{
		`class="page-title"`,
		`class="icon"`,
		"Tokens by Provider",
		"Tokens by Model",
		"/api/providers",
		"/api/models",
		"No usage yet",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("home body does not contain %q", want)
		}
	}
}

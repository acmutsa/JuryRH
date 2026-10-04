package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRateLimitQRRegistration(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		router := gin.New()
		router.Use(rateLimit(CreateLimiter(1, blocked)))
		router.POST("/api/qr/add", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
		router.GET("/api/judge", func(ctx *gin.Context) { ctx.Status(http.StatusOK) })
		request := func(method, path string) int {
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
			return recorder.Code
		}
		expected := http.StatusOK
		if blocked {
			expected = http.StatusTooManyRequests
		}
		if status := request(http.MethodPost, "/api/qr/add"); status != expected {
			t.Fatalf("blocked=%v: first registration status=%d, want %d", blocked, status, expected)
		}
		if status := request(http.MethodPost, "/api/qr/add"); status != http.StatusTooManyRequests {
			t.Fatalf("second registration should be rate limited, got %d", status)
		}
		if status := request(http.MethodGet, "/api/judge"); status != http.StatusOK {
			t.Fatalf("registration limit must not block existing judge sessions, got %d", status)
		}
	}
}

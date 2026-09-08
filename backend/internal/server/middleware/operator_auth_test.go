package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOperatorCredentialBodiesNeverEnterAudit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := &auditCaptureRepository{}
	svc := service.NewAuditLogService(repo, nil)
	svc.Start()
	r := gin.New()
	r.Use(gin.HandlerFunc(NewAuditLogMiddleware(svc)))
	for _, path := range []string{"/api/v1/operator/accounts", "/api/v1/operator/openai/create-account"} {
		r.POST(path, func(c *gin.Context) {
			var body map[string]any
			require.NoError(t, c.ShouldBindJSON(&body))
			require.Equal(t, "operator-secret-canary", body["code"])
			c.Status(204)
		})
		w := httptest.NewRecorder()
		req := httptest.NewRequest("POST", path, bytes.NewBufferString(`{"code":"operator-secret-canary"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		require.Equal(t, 204, w.Code)
	}
	svc.Stop()
	require.Len(t, repo.logs, 2)
	for _, entry := range repo.logs {
		require.Equal(t, "<credential-bearing body omitted>", entry.RequestBody)
		require.NotContains(t, entry.RequestBody, "operator-secret-canary")
	}
}

func TestOperatorAccessFailsClosed(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		enabled, role string
		status        int
	}{{"", service.RoleOperator, 404}, {"tru", service.RoleOperator, 404}, {"true", service.RoleUser, 403}, {"true", service.RoleAdmin, 403}, {"true", service.RoleOperator, 204}} {
		t.Run(tc.enabled+"/"+tc.role, func(t *testing.T) {
			t.Setenv("OPERATOR_RESOURCES_ENABLED", tc.enabled)
			r := gin.New()
			r.Use(func(c *gin.Context) { c.Set(string(ContextKeyUserRole), tc.role) }, OperatorAccess())
			r.GET("/", func(c *gin.Context) { c.Status(http.StatusNoContent) })
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			require.Equal(t, tc.status, w.Code)
		})
	}
}

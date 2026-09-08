package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOperatorRoutesExplicitAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("OPERATOR_RESOURCES_ENABLED", "true")
	r := gin.New()
	h := &handler.Handlers{Admin: &handler.AdminHandlers{}}
	RegisterOperatorRoutes(r.Group("/api/v1"), h, middleware.JWTAuthMiddleware(func(c *gin.Context) { c.Next() }), middleware.AuditLogMiddleware(func(c *gin.Context) { c.Next() }), nil, middleware.NewPanelRateLimiter(nil, nil))
	expected := map[string]bool{"GET /api/v1/operator/compliance": true, "POST /api/v1/operator/compliance/accept": true, "GET /api/v1/operator/accounts": true, "POST /api/v1/operator/accounts": true, "PUT /api/v1/operator/accounts/:id": true, "GET /api/v1/operator/groups": true, "POST /api/v1/operator/groups": true, "PUT /api/v1/operator/groups/:id": true, "GET /api/v1/operator/group-options": true, "GET /api/v1/operator/proxy-options": true, "POST /api/v1/operator/openai/auth-url": true, "POST /api/v1/operator/openai/create-account": true}
	for _, route := range r.Routes() {
		key := route.Method + " " + route.Path
		require.True(t, expected[key], key)
		delete(expected, key)
	}
	require.Empty(t, expected)
}

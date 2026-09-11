package routes

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/handler"
	adminhandler "github.com/Wei-Shaw/sub2api/internal/handler/admin"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestInsightsRoutesAreExplicitlyReadOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handlers := &handler.Handlers{Admin: &handler.AdminHandlers{
		Dashboard: adminhandler.NewDashboardHandler(nil, nil),
		Ops:       adminhandler.NewOpsHandler(nil),
	}}

	registerInsightsRoutes(router.Group("/api/v1"), handlers)

	expected := map[string]bool{
		"GET /api/v1/insights/dashboard/snapshot-v2":            true,
		"GET /api/v1/insights/dashboard/users-trend":            true,
		"GET /api/v1/insights/dashboard/users-ranking":          true,
		"GET /api/v1/insights/ops/realtime-traffic":             true,
		"GET /api/v1/insights/ops/dashboard/snapshot-v2":        true,
		"GET /api/v1/insights/ops/dashboard/overview":           true,
		"GET /api/v1/insights/ops/dashboard/throughput-trend":   true,
		"GET /api/v1/insights/ops/dashboard/latency-histogram":  true,
		"GET /api/v1/insights/ops/dashboard/error-trend":        true,
		"GET /api/v1/insights/ops/dashboard/error-distribution": true,
	}

	for _, route := range router.Routes() {
		key := route.Method + " " + route.Path
		require.True(t, expected[key], key)
		delete(expected, key)
	}
	require.Empty(t, expected)
}

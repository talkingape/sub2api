package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"

	"github.com/gin-gonic/gin"
)

// registerInsightsRoutes exposes an explicit read-only allowlist for the shared
// system dashboard and operations overview. Authentication and rate limiting
// are inherited from the parent user route group. Mutating endpoints remain
// under /admin only.
func registerInsightsRoutes(authenticated *gin.RouterGroup, h *handler.Handlers) {
	insights := authenticated.Group("/insights")

	dashboard := insights.Group("/dashboard")
	{
		dashboard.GET("/snapshot-v2", h.Admin.Dashboard.GetSnapshotV2)
		dashboard.GET("/users-trend", h.Admin.Dashboard.GetUserUsageTrend)
		dashboard.GET("/users-ranking", h.Admin.Dashboard.GetUserSpendingRanking)
	}

	ops := insights.Group("/ops")
	{
		ops.GET("/realtime-traffic", h.Admin.Ops.GetRealtimeTrafficSummary)
		ops.GET("/dashboard/snapshot-v2", h.Admin.Ops.GetDashboardSnapshotV2)
		ops.GET("/dashboard/overview", h.Admin.Ops.GetDashboardOverview)
		ops.GET("/dashboard/throughput-trend", h.Admin.Ops.GetDashboardThroughputTrend)
		ops.GET("/dashboard/latency-histogram", h.Admin.Ops.GetDashboardLatencyHistogram)
		ops.GET("/dashboard/error-trend", h.Admin.Ops.GetDashboardErrorTrend)
		ops.GET("/dashboard/error-distribution", h.Admin.Ops.GetDashboardErrorDistribution)
	}
}

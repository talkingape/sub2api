package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Explicit registration is the allowlist. In particular no subscription, user,
// export, bulk mutation, system setting or arbitrary upstream probe is exposed.
func RegisterOperatorRoutes(v1 *gin.RouterGroup, h *handler.Handlers, jwt middleware.JWTAuthMiddleware,
	audit middleware.AuditLogMiddleware, settings *service.SettingService, limiter *middleware.PanelRateLimiter) {
	op := v1.Group("/operator")
	op.Use(gin.HandlerFunc(jwt), middleware.OperatorAccess(), limiter.Global(), gin.HandlerFunc(audit))
	op.GET("/compliance", h.Admin.Compliance.GetStatus)
	op.POST("/compliance/accept", h.Admin.Compliance.Accept)
	op.Use(middleware.AdminComplianceGuard(settings))
	op.GET("/accounts", h.Admin.Account.OperatorList)
	op.POST("/accounts", h.Admin.Account.OperatorCreate)
	op.PUT("/accounts/:id", h.Admin.Account.OperatorUpdate)
	op.GET("/groups", h.Admin.Group.OperatorList)
	op.POST("/groups", h.Admin.Group.OperatorCreate)
	op.PUT("/groups/:id", h.Admin.Group.OperatorUpdate)
	op.GET("/group-options", h.Admin.Group.OperatorOptions)
	op.GET("/proxy-options", h.Admin.Account.OperatorProxyOptions)
	op.POST("/openai/auth-url", h.Admin.Account.OperatorOpenAIAuthURL)
	op.POST("/openai/create-account", h.Admin.Account.OperatorOpenAICreate)
}

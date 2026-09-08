package middleware

import (
	"os"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// OperatorAccess is deliberately separate from AdminAuth: adding a resource
// maintainer must never grant access to existing administrator endpoints.
// The rollout switch fails closed, including for misspelled values.
func OperatorAccess() gin.HandlerFunc {
	enabled := strings.EqualFold(strings.TrimSpace(os.Getenv("OPERATOR_RESOURCES_ENABLED")), "true")
	return func(c *gin.Context) {
		if !enabled {
			AbortWithError(c, 404, "OPERATOR_DISABLED", "Operator resource management is disabled")
			return
		}
		role, ok := GetUserRoleFromContext(c)
		if !ok || role != service.RoleOperator {
			AbortWithError(c, 403, "FORBIDDEN", "Operator access required")
			return
		}
		c.Next()
	}
}

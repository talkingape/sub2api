package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// Only administrator handlers call this helper; operator payloads do not accept
// manager IDs. Zero clears an assignment without deleting the resource.
func validateResourceManagerRequest(c *gin.Context, svc service.AdminService, id *int64) bool {
	if id == nil || *id == 0 {
		return true
	}
	if *id < 0 {
		response.BadRequest(c, "Invalid manager user ID")
		return false
	}
	u, err := svc.GetUser(c.Request.Context(), *id)
	if err != nil || u == nil || !u.IsActive() || u.Role != service.RoleOperator {
		response.BadRequest(c, "Manager must be an active operator")
		return false
	}
	return true
}

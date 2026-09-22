package router

import (
	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/handler"
	"food-allergen-crosscontact-analyzer/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerMitigationRoutes(group *gin.RouterGroup, h *handler.MitigationHandler) {
	mitigations := group.Group("/mitigations")
	mitigations.GET("", h.List)
	mitigations.GET("/:id", h.Get)
	mitigations.POST("", middleware.RBAC(constants.RoleQualityAnalyst, constants.RoleAdmin), h.Create)
	mitigations.PUT("/:id", middleware.RBAC(constants.RoleQualityAnalyst, constants.RoleAdmin), h.Resubmit)
	mitigations.POST("/:id/review", middleware.RBAC(constants.RoleReviewer, constants.RoleAdmin), h.Review)
}

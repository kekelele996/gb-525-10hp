package router

import (
	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/handler"
	"food-allergen-crosscontact-analyzer/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

func registerMitigationRoutes(group *gin.RouterGroup, h *handler.MitigationHandler) {
	measures := group.Group("/mitigations")
	measures.GET("", h.List)
	measures.GET("/:id", h.Get)
	measures.POST("", middleware.RBAC(constants.RoleQualityAnalyst, constants.RoleAdmin), h.Create)
	measures.POST("/:id/review", middleware.RBAC(constants.RoleReviewer, constants.RoleAdmin), h.Review)
}

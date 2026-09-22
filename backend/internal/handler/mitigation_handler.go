package handler

import (
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/service"
	"food-allergen-crosscontact-analyzer/backend/internal/util"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type MitigationHandler struct {
	service  *service.MitigationService
	validate *validator.Validate
}

func NewMitigationHandler(svc *service.MitigationService, validate *validator.Validate) *MitigationHandler {
	return &MitigationHandler{service: svc, validate: validate}
}

func (h *MitigationHandler) List(c *gin.Context) {
	var query dto.MitigationQuery
	if !util.BindQuery(c, &query, h.validate) {
		return
	}
	items, total, err := h.service.List(c.Request.Context(), query)
	if err != nil {
		util.Error(c, err)
		return
	}
	page, size := util.PageValues(query.Page, query.PageSize)
	util.Page(c, items, page, size, total)
}

func (h *MitigationHandler) Get(c *gin.Context) {
	id, ok := util.PathID(c, "id")
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), id)
	if err != nil {
		util.Error(c, err)
		return
	}
	util.OK(c, item)
}

func (h *MitigationHandler) Create(c *gin.Context) {
	principal, ok := util.Principal(c)
	if !ok {
		return
	}
	var request dto.CreateMitigationRequest
	if !util.BindJSON(c, &request, h.validate) {
		return
	}
	item, err := h.service.Create(c.Request.Context(), request, principal, RequestID(c))
	if err != nil {
		util.Error(c, err)
		return
	}
	util.Created(c, item)
}

func (h *MitigationHandler) Review(c *gin.Context) {
	id, ok := util.PathID(c, "id")
	if !ok {
		return
	}
	principal, ok := util.Principal(c)
	if !ok {
		return
	}
	var request dto.ReviewMitigationRequest
	if !util.BindJSON(c, &request, h.validate) {
		return
	}
	item, err := h.service.Review(c.Request.Context(), id, request, principal, RequestID(c))
	if err != nil {
		util.Error(c, err)
		return
	}
	util.OK(c, item)
}

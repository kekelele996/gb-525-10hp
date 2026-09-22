package dto

type CreateMitigationRequest struct {
	RouteID        uint   `json:"route_id" validate:"required,min=1"`
	Allergen       string `json:"allergen" validate:"required,min=1,max=64"`
	SourceStepCode string `json:"source_step_code" validate:"required,min=2,max=64,nefield=TargetStepCode"`
	TargetStepCode string `json:"target_step_code" validate:"required,min=2,max=64"`
	MeasureType    string `json:"measure_type" validate:"required,oneof=cleaning line_change"`
	CompletedAt    string `json:"completed_at" validate:"required,datetime=2006-01-02"`
	EvidenceNote   string `json:"evidence_note" validate:"required,min=4,max=1000"`
}

type ReviewMitigationRequest struct {
	Decision string `json:"decision" validate:"required,oneof=approved rejected"`
	Reason   string `json:"reason" validate:"required,min=4,max=1000"`
}

type MitigationQuery struct {
	Page     int    `form:"page" validate:"omitempty,min=1"`
	PageSize int    `form:"page_size" validate:"omitempty,min=1,max=100"`
	RouteID  uint   `form:"route_id"`
	Status   string `form:"status" validate:"omitempty,oneof=pending_review approved rejected invalidated"`
}

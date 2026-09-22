package dto

type CreateMitigationRequest struct {
	RouteID        uint   `json:"route_id" validate:"required,min=1"`
	Allergen       string `json:"allergen" validate:"required,min=1,max=120"`
	SourceStepCode string `json:"source_step_code" validate:"required,min=2,max=64"`
	TargetStepCode string `json:"target_step_code" validate:"required,min=2,max=64,nefield=SourceStepCode"`
	MeasureType    string `json:"measure_type" validate:"required,oneof=cleaning line_change"`
	CompletedOn    string `json:"completed_on" validate:"required,len=10"`
	EvidenceNote   string `json:"evidence_note" validate:"required,min=4,max=1000"`
}

type ResubmitMitigationRequest struct {
	MeasureType  string `json:"measure_type" validate:"required,oneof=cleaning line_change"`
	CompletedOn  string `json:"completed_on" validate:"required,len=10"`
	EvidenceNote string `json:"evidence_note" validate:"required,min=4,max=1000"`
}

type ReviewMitigationRequest struct {
	Decision string `json:"decision" validate:"required,oneof=approved rejected"`
	Reason   string `json:"reason" validate:"required,min=4,max=1000"`
}

type MitigationQuery struct {
	Page     int    `form:"page" validate:"omitempty,min=1"`
	PageSize int    `form:"page_size" validate:"omitempty,min=1,max=100"`
	RouteID  uint   `form:"route_id"`
	Status   string `form:"status" validate:"omitempty,oneof=pending pending_review approved rejected"`
	Allergen string `form:"allergen" validate:"omitempty,max=120"`
}

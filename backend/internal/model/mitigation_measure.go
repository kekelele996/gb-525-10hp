package model

import (
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
)

// MitigationMeasure is one registered cleaning or line-change action for a
// high-risk path (route version + allergen + source to target step). Only one
// pending_review measure may exist per path; input version changes before
// reviewer approval move it to invalidated.
type MitigationMeasure struct {
	ID               uint                       `gorm:"primaryKey" json:"id"`
	RouteID          uint                       `gorm:"index:idx_mitigation_path,priority:1;not null" json:"route_id"`
	RouteVersion     uint                       `gorm:"not null" json:"route_version"`
	Allergen         string                     `gorm:"size:64;index:idx_mitigation_path,priority:2;not null" json:"allergen"`
	SourceStepCode   string                     `gorm:"size:64;index:idx_mitigation_path,priority:3;not null" json:"source_step_code"`
	TargetStepCode   string                     `gorm:"size:64;index:idx_mitigation_path,priority:4;not null" json:"target_step_code"`
	MeasureType      constants.MeasureType      `gorm:"size:24;not null;check:measure_type IN ('cleaning','line_change')" json:"measure_type"`
	CompletedAt      time.Time                  `gorm:"type:date;not null" json:"completed_at"`
	EvidenceNote     string                     `gorm:"type:text;not null" json:"evidence_note"`
	MitigationStatus constants.MitigationStatus `gorm:"size:24;index;not null;check:mitigation_status IN ('pending_review','approved','rejected','invalidated')" json:"mitigation_status"`
	CreatedBy        uint                       `gorm:"index;not null" json:"created_by"`
	ReviewedBy       *uint                      `gorm:"index" json:"reviewed_by"`
	ReviewReason     string                     `gorm:"type:text" json:"review_reason"`
	ReviewedAt       *time.Time                 `json:"reviewed_at"`
	InvalidatedAt    *time.Time                 `json:"invalidated_at"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

func (MitigationMeasure) TableName() string { return "mitigation_measures" }

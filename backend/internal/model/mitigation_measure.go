package model

import (
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
)

// MitigationMeasure is one registered cleaning or line-change measure for a
// high-risk matrix path (route version + allergen + source to target step).
// The raw score and risk level of the path are snapshotted at submission so
// the matrix can keep showing the original figures after approval.
type MitigationMeasure struct {
	ID               uint                       `gorm:"primaryKey" json:"id"`
	RouteID          uint                       `gorm:"index:idx_mitigation_path,priority:1;not null" json:"route_id"`
	RouteVersion     uint                       `gorm:"not null" json:"route_version"`
	Allergen         string                     `gorm:"size:120;index:idx_mitigation_path,priority:2;not null" json:"allergen"`
	SourceStepCode   string                     `gorm:"size:64;index:idx_mitigation_path,priority:3;not null" json:"source_step_code"`
	TargetStepCode   string                     `gorm:"size:64;index:idx_mitigation_path,priority:4;not null" json:"target_step_code"`
	MeasureType      string                     `gorm:"size:32;not null;check:measure_type IN ('cleaning','line_change')" json:"measure_type"`
	CompletedOn      time.Time                  `gorm:"type:date;not null" json:"completed_on"`
	EvidenceNote     string                     `gorm:"type:text;not null" json:"evidence_note"`
	RawScore         float64                    `gorm:"not null" json:"raw_score"`
	RiskLevel        constants.RiskLevel        `gorm:"size:16;not null;check:risk_level IN ('low','medium','high','critical')" json:"risk_level"`
	MitigationStatus constants.MitigationStatus `gorm:"size:24;index;not null;check:mitigation_status IN ('pending','pending_review','approved','rejected')" json:"mitigation_status"`
	CreatedBy        uint                       `gorm:"index;not null" json:"created_by"`
	ReviewedBy       *uint                      `gorm:"index" json:"reviewed_by"`
	ReviewReason     string                     `gorm:"type:text" json:"review_reason"`
	ReviewedAt       *time.Time                 `json:"reviewed_at"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

func (MitigationMeasure) TableName() string { return "mitigation_measures" }

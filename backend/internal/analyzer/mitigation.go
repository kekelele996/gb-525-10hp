package analyzer

import (
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
)

// MitigationInfo is the approved measure attached to a propagated path. The
// raw score, risk level and cleaning evidence of the path stay untouched.
type MitigationInfo struct {
	MeasureID    uint                  `json:"measure_id"`
	MeasureType  constants.MeasureType `json:"measure_type"`
	CompletedAt  string                `json:"completed_at"`
	EvidenceNote string                `json:"evidence_note"`
	RouteVersion uint                  `json:"route_version"`
	ReviewedAt   *time.Time            `json:"reviewed_at"`
}

// ApplyMitigations marks risk items covered by an approved measure and
// recomputes the mitigated path count of every matrix cell. Matching uses the
// path identity: allergen + source step + target step.
func ApplyMitigations(result *Result, measures []model.MitigationMeasure) {
	if len(measures) == 0 || result == nil {
		return
	}
	type pathKey struct{ allergen, source, target string }
	approved := make(map[pathKey]MitigationInfo, len(measures))
	for _, measure := range measures {
		if measure.MitigationStatus != constants.MitigationApproved {
			continue
		}
		approved[pathKey{measure.Allergen, measure.SourceStepCode, measure.TargetStepCode}] = MitigationInfo{
			MeasureID: measure.ID, MeasureType: measure.MeasureType,
			CompletedAt: measure.CompletedAt.Format("2006-01-02"), EvidenceNote: measure.EvidenceNote,
			RouteVersion: measure.RouteVersion, ReviewedAt: measure.ReviewedAt,
		}
	}
	type cellKey struct{ step, allergen string }
	mitigated := make(map[cellKey]int)
	for index := range result.RiskItems {
		item := &result.RiskItems[index]
		info, ok := approved[pathKey{item.Allergen, item.SourceStepCode, item.TargetStepCode}]
		if !ok {
			continue
		}
		item.Mitigation = &info
		mitigated[cellKey{item.TargetStepCode, item.Allergen}]++
	}
	for index := range result.Matrix {
		result.Matrix[index].MitigatedPaths = mitigated[cellKey{result.Matrix[index].TargetStepCode, result.Matrix[index].Allergen}]
	}
}

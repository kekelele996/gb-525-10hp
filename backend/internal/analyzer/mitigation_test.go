package analyzer

import (
	"testing"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
)

func TestApplyMitigations(t *testing.T) {
	reviewed := time.Date(2026, 9, 20, 8, 30, 0, 0, time.UTC)
	result := Result{
		RiskItems: []RiskItem{
			{Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", RawScore: 0.7, RiskLevel: constants.RiskCritical},
			{Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "PACK-03", RawScore: 0.4, RiskLevel: constants.RiskHigh},
			{Allergen: "Milk", SourceStepCode: "FILL-02", TargetStepCode: "PACK-03", RawScore: 0.2, RiskLevel: constants.RiskMedium},
		},
		Matrix: []MatrixCell{
			{TargetStepCode: "FILL-02", Allergen: "Peanut", PathCount: 1},
			{TargetStepCode: "PACK-03", Allergen: "Peanut", PathCount: 1},
			{TargetStepCode: "PACK-03", Allergen: "Milk", PathCount: 1},
		},
	}
	measures := []model.MitigationMeasure{
		{ID: 9, RouteID: 3, RouteVersion: 4, Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", MeasureType: constants.MeasureCleaning, CompletedAt: time.Date(2026, 9, 18, 0, 0, 0, 0, time.UTC), EvidenceNote: "Wet clean WC-18 verified", MitigationStatus: constants.MitigationApproved, ReviewedAt: &reviewed},
		{ID: 10, RouteID: 3, RouteVersion: 4, Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "PACK-03", MeasureType: constants.MeasureLineChange, CompletedAt: time.Date(2026, 9, 19, 0, 0, 0, 0, time.UTC), EvidenceNote: "pending measure must not apply", MitigationStatus: constants.MitigationPendingReview},
	}

	ApplyMitigations(&result, measures)

	first := result.RiskItems[0]
	if first.Mitigation == nil {
		t.Fatal("approved measure should attach to the matching path")
	}
	if first.Mitigation.MeasureID != 9 || first.Mitigation.MeasureType != constants.MeasureCleaning || first.Mitigation.CompletedAt != "2026-09-18" || first.Mitigation.RouteVersion != 4 {
		t.Fatalf("unexpected mitigation info: %+v", first.Mitigation)
	}
	if first.RawScore != 0.7 || first.RiskLevel != constants.RiskCritical {
		t.Fatalf("mitigation must preserve raw score and level, got score=%v level=%s", first.RawScore, first.RiskLevel)
	}
	if result.RiskItems[1].Mitigation != nil {
		t.Fatal("non-approved measure must not mitigate the path")
	}
	if result.RiskItems[2].Mitigation != nil {
		t.Fatal("unregistered path must stay unmitigated")
	}
	want := map[string]int{"FILL-02/Peanut": 1, "PACK-03/Peanut": 0, "PACK-03/Milk": 0}
	for _, cell := range result.Matrix {
		key := cell.TargetStepCode + "/" + cell.Allergen
		if cell.MitigatedPaths != want[key] {
			t.Fatalf("cell %s mitigated paths = %d, want %d", key, cell.MitigatedPaths, want[key])
		}
	}
}

func TestApplyMitigationsEmpty(t *testing.T) {
	result := Result{RiskItems: []RiskItem{{Allergen: "Peanut"}}, Matrix: []MatrixCell{{TargetStepCode: "FILL-02", Allergen: "Peanut"}}}
	ApplyMitigations(&result, nil)
	if result.RiskItems[0].Mitigation != nil || result.Matrix[0].MitigatedPaths != 0 {
		t.Fatal("empty measures must leave the result untouched")
	}
	ApplyMitigations(nil, []model.MitigationMeasure{{ID: 1}})
}

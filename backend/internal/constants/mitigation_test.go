package constants

import "testing"

func TestCanTransitionMitigation(t *testing.T) {
	tests := []struct {
		name     string
		from, to MitigationStatus
		want     bool
	}{
		{name: "review approves", from: MitigationPendingReview, to: MitigationApproved, want: true},
		{name: "review rejects", from: MitigationPendingReview, to: MitigationRejected, want: true},
		{name: "input change invalidates", from: MitigationPendingReview, to: MitigationInvalidated, want: true},
		{name: "approved is terminal", from: MitigationApproved, to: MitigationInvalidated, want: false},
		{name: "rejected is terminal", from: MitigationRejected, to: MitigationApproved, want: false},
		{name: "invalidated cannot be reviewed", from: MitigationInvalidated, to: MitigationApproved, want: false},
		{name: "invalidated cannot reopen", from: MitigationInvalidated, to: MitigationPendingReview, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanTransitionMitigation(test.from, test.to); got != test.want {
				t.Fatalf("CanTransitionMitigation(%s,%s) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestMitigationStatusValid(t *testing.T) {
	for _, status := range []MitigationStatus{MitigationPendingReview, MitigationApproved, MitigationRejected, MitigationInvalidated} {
		if !status.Valid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if MitigationStatus("stale").Valid() {
		t.Fatal("unexpected valid status")
	}
	if !MeasureCleaning.Valid() || !MeasureLineChange.Valid() {
		t.Fatal("measure types should be valid")
	}
	if MeasureType("label_change").Valid() {
		t.Fatal("unexpected valid measure type")
	}
}

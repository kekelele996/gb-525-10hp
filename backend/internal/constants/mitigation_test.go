package constants

import "testing"

func TestCanMitigationTransition(t *testing.T) {
	tests := []struct {
		name     string
		from, to MitigationStatus
		want     bool
	}{
		{name: "analyst resubmits pending", from: MitigationPending, to: MitigationPendingReview, want: true},
		{name: "reviewer approves", from: MitigationPendingReview, to: MitigationApproved, want: true},
		{name: "reviewer rejects", from: MitigationPendingReview, to: MitigationRejected, want: true},
		{name: "input change invalidates", from: MitigationPendingReview, to: MitigationPending, want: true},
		{name: "pending cannot skip review", from: MitigationPending, to: MitigationApproved, want: false},
		{name: "approved is terminal", from: MitigationApproved, to: MitigationPending, want: false},
		{name: "rejected is terminal", from: MitigationRejected, to: MitigationPendingReview, want: false},
		{name: "approved cannot be rejected", from: MitigationApproved, to: MitigationRejected, want: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := CanMitigationTransition(test.from, test.to); got != test.want {
				t.Fatalf("CanMitigationTransition(%s,%s) = %v, want %v", test.from, test.to, got, test.want)
			}
		})
	}
}

func TestMitigationStatusValid(t *testing.T) {
	for _, status := range []MitigationStatus{MitigationPending, MitigationPendingReview, MitigationApproved, MitigationRejected} {
		if !status.Valid() {
			t.Fatalf("status %q should be valid", status)
		}
	}
	if MitigationStatus("stale").Valid() {
		t.Fatal("unknown status unexpectedly valid")
	}
}

package constants

// MitigationStatus tracks the review lifecycle of a cleaning or line-change
// measure registered for one high-risk propagation path.
type MitigationStatus string

const (
	MitigationPending       MitigationStatus = "pending"
	MitigationPendingReview MitigationStatus = "pending_review"
	MitigationApproved      MitigationStatus = "approved"
	MitigationRejected      MitigationStatus = "rejected"
)

func (s MitigationStatus) Valid() bool {
	switch s {
	case MitigationPending, MitigationPendingReview, MitigationApproved, MitigationRejected:
		return true
	default:
		return false
	}
}

// CanMitigationTransition documents the allowed lifecycle moves: analysts
// (re)submit pending measures for review, reviewers approve or reject, and
// input version changes push a measure awaiting review back to pending.
func CanMitigationTransition(from, to MitigationStatus) bool {
	switch from {
	case MitigationPending:
		return to == MitigationPendingReview
	case MitigationPendingReview:
		return to == MitigationApproved || to == MitigationRejected || to == MitigationPending
	default:
		return false
	}
}

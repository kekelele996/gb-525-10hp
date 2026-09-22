package constants

// MitigationStatus tracks the review lifecycle of a registered cleaning or
// line-change measure for one high-risk propagation path.
type MitigationStatus string

const (
	MitigationPendingReview MitigationStatus = "pending_review"
	MitigationApproved      MitigationStatus = "approved"
	MitigationRejected      MitigationStatus = "rejected"
	MitigationInvalidated   MitigationStatus = "invalidated"
)

func (s MitigationStatus) Valid() bool {
	switch s {
	case MitigationPendingReview, MitigationApproved, MitigationRejected, MitigationInvalidated:
		return true
	default:
		return false
	}
}

func CanTransitionMitigation(from, to MitigationStatus) bool {
	switch from {
	case MitigationPendingReview:
		return to == MitigationApproved || to == MitigationRejected || to == MitigationInvalidated
	default:
		return false
	}
}

// MeasureType is the physical mitigation action registered for a path.
type MeasureType string

const (
	MeasureCleaning   MeasureType = "cleaning"
	MeasureLineChange MeasureType = "line_change"
)

func (t MeasureType) Valid() bool {
	return t == MeasureCleaning || t == MeasureLineChange
}

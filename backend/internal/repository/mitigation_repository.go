package repository

import (
	"context"
	"fmt"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
	"gorm.io/gorm"
)

// openMitigationStatuses are the states that occupy a path: at most one
// measure per path may be pending or pending review at any time.
var openMitigationStatuses = []constants.MitigationStatus{constants.MitigationPending, constants.MitigationPendingReview}

type MitigationRepository interface {
	Create(context.Context, *model.MitigationMeasure, AuditContext) error
	Get(context.Context, uint) (model.MitigationMeasure, error)
	List(context.Context, dto.MitigationQuery) ([]model.MitigationMeasure, int64, error)
	Resubmit(context.Context, *model.MitigationMeasure, AuditContext) error
	Review(context.Context, uint, constants.MitigationStatus, uint, string, AuditContext) error
	ActiveForRoute(context.Context, uint) ([]model.MitigationMeasure, error)
}

type mitigationRepository struct{ db *gorm.DB }

func NewMitigationRepository(db *gorm.DB) MitigationRepository { return &mitigationRepository{db: db} }

func (r *mitigationRepository) Create(ctx context.Context, measure *model.MitigationMeasure, scope AuditContext) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var open int64
		if err := tx.Model(&model.MitigationMeasure{}).Where("route_id = ? AND allergen = ? AND source_step_code = ? AND target_step_code = ? AND mitigation_status IN ?", measure.RouteID, measure.Allergen, measure.SourceStepCode, measure.TargetStepCode, openMitigationStatuses).Count(&open).Error; err != nil {
			return fmt.Errorf("count open mitigations: %w", err)
		}
		if open > 0 {
			return fmt.Errorf("create mitigation measure: %w", ErrDuplicate)
		}
		if err := tx.Create(measure).Error; err != nil {
			if err == gorm.ErrDuplicatedKey {
				return fmt.Errorf("create mitigation measure: %w", ErrDuplicate)
			}
			return fmt.Errorf("create mitigation measure: %w", err)
		}
		audit, err := makeAudit(scope, "mitigation.submitted", "mitigation_measure", measure.ID, "", mitigationSummary(*measure), map[string]any{"route_id": measure.RouteID, "route_version": measure.RouteVersion})
		if err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("audit mitigation creation: %w", err)
		}
		return nil
	})
}

func (r *mitigationRepository) Get(ctx context.Context, id uint) (model.MitigationMeasure, error) {
	var measure model.MitigationMeasure
	if err := r.db.WithContext(ctx).First(&measure, id).Error; err != nil {
		return model.MitigationMeasure{}, fmt.Errorf("get mitigation measure: %w", err)
	}
	return measure, nil
}

func (r *mitigationRepository) List(ctx context.Context, query dto.MitigationQuery) ([]model.MitigationMeasure, int64, error) {
	db := r.db.WithContext(ctx).Model(&model.MitigationMeasure{})
	if query.RouteID > 0 {
		db = db.Where("route_id = ?", query.RouteID)
	}
	if query.Status != "" {
		db = db.Where("mitigation_status = ?", query.Status)
	}
	if query.Allergen != "" {
		db = db.Where("LOWER(allergen) = LOWER(?)", query.Allergen)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count mitigation measures: %w", err)
	}
	page, size := pageValues(query.Page, query.PageSize)
	var measures []model.MitigationMeasure
	if err := db.Order("created_at DESC, id DESC").Offset((page - 1) * size).Limit(size).Find(&measures).Error; err != nil {
		return nil, 0, fmt.Errorf("list mitigation measures: %w", err)
	}
	return measures, total, nil
}

func (r *mitigationRepository) Resubmit(ctx context.Context, measure *model.MitigationMeasure, scope AuditContext) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before model.MitigationMeasure
		if err := tx.First(&before, measure.ID).Error; err != nil {
			return fmt.Errorf("load mitigation before resubmit: %w", err)
		}
		updates := map[string]any{"measure_type": measure.MeasureType, "completed_on": measure.CompletedOn, "evidence_note": measure.EvidenceNote, "route_version": measure.RouteVersion, "raw_score": measure.RawScore, "risk_level": measure.RiskLevel, "mitigation_status": constants.MitigationPendingReview}
		result := tx.Model(&model.MitigationMeasure{}).Where("id = ? AND mitigation_status = ?", measure.ID, constants.MitigationPending).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("resubmit mitigation: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("resubmit mitigation: %w", ErrStateConflict)
		}
		if err := tx.First(measure, measure.ID).Error; err != nil {
			return fmt.Errorf("reload mitigation: %w", err)
		}
		audit, err := makeAudit(scope, "mitigation.resubmitted", "mitigation_measure", measure.ID, mitigationSummary(before), mitigationSummary(*measure), map[string]any{"route_version": measure.RouteVersion})
		if err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("audit mitigation resubmit: %w", err)
		}
		return nil
	})
}

func (r *mitigationRepository) Review(ctx context.Context, id uint, target constants.MitigationStatus, reviewerID uint, reason string, scope AuditContext) error {
	if target != constants.MitigationApproved && target != constants.MitigationRejected {
		return fmt.Errorf("review mitigation target %q: %w", target, ErrStateConflict)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		updates := map[string]any{"mitigation_status": target, "reviewed_by": reviewerID, "review_reason": reason, "reviewed_at": &now}
		result := tx.Model(&model.MitigationMeasure{}).Where("id = ? AND mitigation_status = ?", id, constants.MitigationPendingReview).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("review mitigation: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("review mitigation: %w", ErrStateConflict)
		}
		audit, err := makeAudit(scope, "mitigation.reviewed", "mitigation_measure", id, string(constants.MitigationPendingReview), string(target), map[string]any{"reviewer_id": reviewerID, "reason": reason})
		if err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("audit mitigation review: %w", err)
		}
		return nil
	})
}

func (r *mitigationRepository) ActiveForRoute(ctx context.Context, routeID uint) ([]model.MitigationMeasure, error) {
	statuses := []constants.MitigationStatus{constants.MitigationPending, constants.MitigationPendingReview, constants.MitigationApproved}
	var measures []model.MitigationMeasure
	if err := r.db.WithContext(ctx).Where("route_id = ? AND mitigation_status IN ?", routeID, statuses).Order("id").Find(&measures).Error; err != nil {
		return nil, fmt.Errorf("list active route mitigations: %w", err)
	}
	return measures, nil
}

func mitigationSummary(measure model.MitigationMeasure) string {
	return fmt.Sprintf("route=%d v%d allergen=%s %s->%s type=%s completed=%s status=%s score=%.6f", measure.RouteID, measure.RouteVersion, measure.Allergen, measure.SourceStepCode, measure.TargetStepCode, measure.MeasureType, measure.CompletedOn.Format("2006-01-02"), measure.MitigationStatus, measure.RawScore)
}

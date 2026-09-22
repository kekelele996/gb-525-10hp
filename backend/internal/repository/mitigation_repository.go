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

type MitigationRepository interface {
	Create(context.Context, *model.MitigationMeasure, AuditContext) error
	Get(context.Context, uint) (model.MitigationMeasure, error)
	List(context.Context, dto.MitigationQuery) ([]model.MitigationMeasure, int64, error)
	PendingExists(context.Context, uint, string, string, string) (bool, error)
	ApprovedForRoute(context.Context, uint) ([]model.MitigationMeasure, error)
	Review(context.Context, uint, constants.MitigationStatus, uint, string, AuditContext) error
}

type mitigationRepository struct{ db *gorm.DB }

func NewMitigationRepository(db *gorm.DB) MitigationRepository {
	return &mitigationRepository{db: db}
}

func (r *mitigationRepository) Create(ctx context.Context, measure *model.MitigationMeasure, scope AuditContext) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(measure).Error; err != nil {
			if err == gorm.ErrDuplicatedKey {
				return fmt.Errorf("create mitigation measure: %w", ErrDuplicate)
			}
			return fmt.Errorf("create mitigation measure: %w", err)
		}
		audit, err := makeAudit(scope, "mitigation.registered", "mitigation_measure", measure.ID, "", mitigationSummary(*measure), map[string]any{"route_id": measure.RouteID, "route_version": measure.RouteVersion})
		if err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("audit mitigation registration: %w", err)
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

func (r *mitigationRepository) PendingExists(ctx context.Context, routeID uint, allergen, source, target string) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&model.MitigationMeasure{}).Where("route_id = ? AND allergen = ? AND source_step_code = ? AND target_step_code = ? AND mitigation_status = ?", routeID, allergen, source, target, constants.MitigationPendingReview).Count(&count).Error; err != nil {
		return false, fmt.Errorf("check pending mitigation: %w", err)
	}
	return count > 0, nil
}

func (r *mitigationRepository) ApprovedForRoute(ctx context.Context, routeID uint) ([]model.MitigationMeasure, error) {
	var measures []model.MitigationMeasure
	if err := r.db.WithContext(ctx).Where("route_id = ? AND mitigation_status = ?", routeID, constants.MitigationApproved).Order("id").Find(&measures).Error; err != nil {
		return nil, fmt.Errorf("list approved mitigations: %w", err)
	}
	return measures, nil
}

func (r *mitigationRepository) Review(ctx context.Context, id uint, target constants.MitigationStatus, reviewerID uint, reason string, scope AuditContext) error {
	if target != constants.MitigationApproved && target != constants.MitigationRejected {
		return fmt.Errorf("review mitigation target %q: %w", target, ErrStateConflict)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var before model.MitigationMeasure
		if err := tx.First(&before, id).Error; err != nil {
			return fmt.Errorf("load mitigation before review: %w", err)
		}
		now := time.Now().UTC()
		updates := map[string]any{"mitigation_status": target, "reviewed_by": reviewerID, "review_reason": reason, "reviewed_at": &now}
		result := tx.Model(&model.MitigationMeasure{}).Where("id = ? AND mitigation_status = ?", id, constants.MitigationPendingReview).Updates(updates)
		if result.Error != nil {
			return fmt.Errorf("review mitigation measure: %w", result.Error)
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("review mitigation measure: %w", ErrStateConflict)
		}
		audit, err := makeAudit(scope, "mitigation.reviewed", "mitigation_measure", id, mitigationSummary(before), fmt.Sprintf("status=%s reviewer=%d", target, reviewerID), map[string]any{"route_id": before.RouteID, "reviewer_id": reviewerID, "reason": reason})
		if err != nil {
			return err
		}
		if err := tx.Create(&audit).Error; err != nil {
			return fmt.Errorf("audit mitigation review: %w", err)
		}
		return nil
	})
}

func mitigationSummary(measure model.MitigationMeasure) string {
	return fmt.Sprintf("route=%d v%d %s %s->%s type=%s status=%s completed=%s", measure.RouteID, measure.RouteVersion, measure.Allergen, measure.SourceStepCode, measure.TargetStepCode, measure.MeasureType, measure.MitigationStatus, measure.CompletedAt.Format("2006-01-02"))
}

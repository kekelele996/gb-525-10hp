package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/analyzer"
	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
	"food-allergen-crosscontact-analyzer/backend/internal/repository"
)

// MatrixPreview computes the live cross-contact matrix for a route. It is
// satisfied by AssessmentService and kept as an interface for tests.
type MatrixPreview interface {
	Preview(context.Context, uint) (analyzer.Result, error)
}

type MitigationService struct {
	mitigations repository.MitigationRepository
	routes      repository.RouteRepository
	matrix      MatrixPreview
}

func NewMitigationService(mitigations repository.MitigationRepository, routes repository.RouteRepository, matrix MatrixPreview) *MitigationService {
	return &MitigationService{mitigations: mitigations, routes: routes, matrix: matrix}
}

func (s *MitigationService) Create(ctx context.Context, request dto.CreateMitigationRequest, actor Principal, requestID string) (model.MitigationMeasure, error) {
	if !CanEdit(actor.Role) {
		return model.MitigationMeasure{}, NewError(http.StatusForbidden, "forbidden", "仅质量分析员或管理员可登记缓解措施", nil)
	}
	completedOn, err := parseCompletedOn(request.CompletedOn)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	route, err := s.routes.Get(ctx, request.RouteID)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	if route.RouteStatus != "active" {
		return model.MitigationMeasure{}, NewError(http.StatusConflict, "route_inactive", "只有 active 路线可以登记缓解措施", nil)
	}
	source, target := normalizeStepCode(request.SourceStepCode), normalizeStepCode(request.TargetStepCode)
	path, err := s.highRiskPath(ctx, route.ID, request.Allergen, source, target)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	measure := model.MitigationMeasure{RouteID: route.ID, RouteVersion: route.Version, Allergen: path.Allergen, SourceStepCode: source, TargetStepCode: target, MeasureType: request.MeasureType, CompletedOn: completedOn, EvidenceNote: strings.TrimSpace(request.EvidenceNote), RawScore: path.RawScore, RiskLevel: path.RiskLevel, MitigationStatus: constants.MitigationPendingReview, CreatedBy: actor.ID}
	if err := s.mitigations.Create(ctx, &measure, AuditScope(actor, requestID)); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return model.MitigationMeasure{}, NewError(http.StatusConflict, "duplicate_pending_measure", "同一路径已存在待复核或待处理的缓解措施", err)
		}
		return model.MitigationMeasure{}, err
	}
	return measure, nil
}

func (s *MitigationService) Resubmit(ctx context.Context, id uint, request dto.ResubmitMitigationRequest, actor Principal, requestID string) (model.MitigationMeasure, error) {
	if !CanEdit(actor.Role) {
		return model.MitigationMeasure{}, NewError(http.StatusForbidden, "forbidden", "仅质量分析员或管理员可重新提交缓解措施", nil)
	}
	measure, err := s.mitigations.Get(ctx, id)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	if measure.MitigationStatus != constants.MitigationPending {
		return model.MitigationMeasure{}, NewError(http.StatusConflict, "state_conflict", "只有待处理的措施可以重新提交复核", nil)
	}
	completedOn, err := parseCompletedOn(request.CompletedOn)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	route, err := s.routes.Get(ctx, measure.RouteID)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	if route.RouteStatus != "active" {
		return model.MitigationMeasure{}, NewError(http.StatusConflict, "route_inactive", "只有 active 路线可以重新提交缓解措施", nil)
	}
	path, err := s.highRiskPath(ctx, measure.RouteID, measure.Allergen, measure.SourceStepCode, measure.TargetStepCode)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	measure.MeasureType = request.MeasureType
	measure.CompletedOn = completedOn
	measure.EvidenceNote = strings.TrimSpace(request.EvidenceNote)
	measure.RouteVersion = route.Version
	measure.RawScore = path.RawScore
	measure.RiskLevel = path.RiskLevel
	if err := s.mitigations.Resubmit(ctx, &measure, AuditScope(actor, requestID)); err != nil {
		return model.MitigationMeasure{}, err
	}
	return measure, nil
}

func (s *MitigationService) Review(ctx context.Context, id uint, request dto.ReviewMitigationRequest, actor Principal, requestID string) (model.MitigationMeasure, error) {
	if !CanReview(actor.Role) {
		return model.MitigationMeasure{}, NewError(http.StatusForbidden, "forbidden", "仅 reviewer 或 admin 可复核缓解措施", nil)
	}
	target := constants.MitigationStatus(request.Decision)
	if target != constants.MitigationApproved && target != constants.MitigationRejected {
		return model.MitigationMeasure{}, NewError(http.StatusBadRequest, "invalid_decision", "复核决定无效", nil)
	}
	if err := s.mitigations.Review(ctx, id, target, actor.ID, strings.TrimSpace(request.Reason), AuditScope(actor, requestID)); err != nil {
		return model.MitigationMeasure{}, err
	}
	return s.mitigations.Get(ctx, id)
}

func (s *MitigationService) Get(ctx context.Context, id uint) (model.MitigationMeasure, error) {
	return s.mitigations.Get(ctx, id)
}
func (s *MitigationService) List(ctx context.Context, query dto.MitigationQuery) ([]model.MitigationMeasure, int64, error) {
	return s.mitigations.List(ctx, query)
}

// highRiskPath locates the highest-scoring live matrix path for the allergen
// and step pair and requires it to be a high-risk path worth mitigating.
func (s *MitigationService) highRiskPath(ctx context.Context, routeID uint, allergen, source, target string) (analyzer.RiskItem, error) {
	result, err := s.matrix.Preview(ctx, routeID)
	if err != nil {
		return analyzer.RiskItem{}, err
	}
	var best *analyzer.RiskItem
	for index := range result.RiskItems {
		item := &result.RiskItems[index]
		if item.SourceStepCode != source || item.TargetStepCode != target || !strings.EqualFold(item.Allergen, strings.TrimSpace(allergen)) {
			continue
		}
		if best == nil || item.RawScore > best.RawScore {
			best = item
		}
	}
	if best == nil {
		return analyzer.RiskItem{}, NewError(http.StatusUnprocessableEntity, "mitigation_path_missing", "所选过敏原与步骤路径在当前矩阵中不存在", nil)
	}
	if constants.RiskRank(best.RiskLevel) < constants.RiskRank(constants.RiskHigh) {
		return analyzer.RiskItem{}, NewError(http.StatusUnprocessableEntity, "mitigation_not_high_risk", "仅 high 或 critical 风险路径可以登记缓解措施", nil)
	}
	return *best, nil
}

// parseCompletedOn enforces that the recorded completion date never sits
// after the day the measure is submitted for review.
func parseCompletedOn(value string) (time.Time, error) {
	completed, err := time.Parse("2006-01-02", strings.TrimSpace(value))
	if err != nil {
		return time.Time{}, NewError(http.StatusUnprocessableEntity, "invalid_completed_on", "完成日期格式应为 YYYY-MM-DD", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if completed.After(today) {
		return time.Time{}, NewError(http.StatusUnprocessableEntity, "completion_after_submission", "完成日期不能晚于复核提交日", nil)
	}
	return completed, nil
}

func normalizeStepCode(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

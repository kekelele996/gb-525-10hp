package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
	"food-allergen-crosscontact-analyzer/backend/internal/repository"
)

type MitigationService struct {
	measures repository.MitigationRepository
	routes   repository.RouteRepository
	profiles repository.ProfileRepository
}

func NewMitigationService(measures repository.MitigationRepository, routes repository.RouteRepository, profiles repository.ProfileRepository) *MitigationService {
	return &MitigationService{measures: measures, routes: routes, profiles: profiles}
}

func (s *MitigationService) Create(ctx context.Context, request dto.CreateMitigationRequest, actor Principal, requestID string) (model.MitigationMeasure, error) {
	if !CanEdit(actor.Role) {
		return model.MitigationMeasure{}, NewError(http.StatusForbidden, "forbidden", "仅 quality_analyst 或 admin 可登记缓解措施", nil)
	}
	route, err := s.routes.Get(ctx, request.RouteID)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	if route.RouteStatus != "active" {
		return model.MitigationMeasure{}, NewError(http.StatusConflict, "route_inactive", "只有 active 路线可以登记缓解措施", nil)
	}
	steps, err := DecodeRouteSteps(route)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	source, target := strings.ToUpper(strings.TrimSpace(request.SourceStepCode)), strings.ToUpper(strings.TrimSpace(request.TargetStepCode))
	if source == target {
		return model.MitigationMeasure{}, NewError(http.StatusBadRequest, "self_loop", "缓解路径的来源和目标步骤不能相同", nil)
	}
	codes := make(map[string]bool, len(steps))
	profileIDs := make([]uint, 0, len(steps))
	seen := make(map[uint]bool)
	for _, step := range steps {
		codes[step.StepCode] = true
		if !seen[step.ProfileID] {
			seen[step.ProfileID] = true
			profileIDs = append(profileIDs, step.ProfileID)
		}
	}
	if !codes[source] || !codes[target] {
		return model.MitigationMeasure{}, NewError(http.StatusBadRequest, "unknown_step", "缓解路径端点必须属于所选路线", nil)
	}
	allergen, err := s.canonicalAllergen(ctx, profileIDs, request.Allergen)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	completed, err := time.Parse("2006-01-02", request.CompletedAt)
	if err != nil {
		return model.MitigationMeasure{}, NewError(http.StatusUnprocessableEntity, "invalid_completed_at", "完成日期格式应为 YYYY-MM-DD", err)
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if completed.After(today) {
		return model.MitigationMeasure{}, NewError(http.StatusUnprocessableEntity, "completion_after_submission", "完成日期不能晚于复核提交日", nil)
	}
	exists, err := s.measures.PendingExists(ctx, route.ID, allergen, source, target)
	if err != nil {
		return model.MitigationMeasure{}, err
	}
	if exists {
		return model.MitigationMeasure{}, NewError(http.StatusConflict, "duplicate_pending_mitigation", "同一路径已存在待复核的缓解措施", nil)
	}
	measure := model.MitigationMeasure{RouteID: route.ID, RouteVersion: route.Version, Allergen: allergen, SourceStepCode: source, TargetStepCode: target, MeasureType: constants.MeasureType(request.MeasureType), CompletedAt: completed, EvidenceNote: strings.TrimSpace(request.EvidenceNote), MitigationStatus: constants.MitigationPendingReview, CreatedBy: actor.ID}
	if err := s.measures.Create(ctx, &measure, AuditScope(actor, requestID)); err != nil {
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
	if err := s.measures.Review(ctx, id, target, actor.ID, request.Reason, AuditScope(actor, requestID)); err != nil {
		return model.MitigationMeasure{}, err
	}
	return s.measures.Get(ctx, id)
}

func (s *MitigationService) Get(ctx context.Context, id uint) (model.MitigationMeasure, error) {
	return s.measures.Get(ctx, id)
}
func (s *MitigationService) List(ctx context.Context, query dto.MitigationQuery) ([]model.MitigationMeasure, int64, error) {
	return s.measures.List(ctx, query)
}

// canonicalAllergen resolves the registered allergen against the spectra of
// the profiles referenced by the route, returning the canonical casing.
func (s *MitigationService) canonicalAllergen(ctx context.Context, profileIDs []uint, input string) (string, error) {
	profiles, err := s.profiles.GetMany(ctx, profileIDs)
	if err != nil {
		return "", err
	}
	wanted := strings.ToLower(strings.TrimSpace(input))
	for _, profile := range profiles {
		var allergens []string
		if err := json.Unmarshal(profile.AllergensJSON, &allergens); err != nil {
			return "", NewError(http.StatusUnprocessableEntity, "profile_json_invalid", "过敏原谱内容无法解析", err)
		}
		for _, allergen := range allergens {
			if strings.ToLower(strings.TrimSpace(allergen)) == wanted {
				return strings.TrimSpace(allergen), nil
			}
		}
	}
	return "", NewError(http.StatusUnprocessableEntity, "unknown_allergen", "过敏原不在该路线引用的谱中", nil)
}

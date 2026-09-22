package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/analyzer"
	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
	"food-allergen-crosscontact-analyzer/backend/internal/repository"
)

type mitigationRepoStub struct {
	measure   model.MitigationMeasure
	createErr error
	created   *model.MitigationMeasure
	resubmit  *model.MitigationMeasure
	reviewed  struct {
		id     uint
		target constants.MitigationStatus
	}
}

func (r *mitigationRepoStub) Create(_ context.Context, measure *model.MitigationMeasure, _ repository.AuditContext) error {
	if r.createErr != nil {
		return r.createErr
	}
	measure.ID = 11
	copy := *measure
	r.created = &copy
	return nil
}
func (r *mitigationRepoStub) Get(_ context.Context, id uint) (model.MitigationMeasure, error) {
	if r.measure.ID == id {
		return r.measure, nil
	}
	return model.MitigationMeasure{}, NewError(404, "not_found", "不存在", nil)
}
func (r *mitigationRepoStub) List(context.Context, dto.MitigationQuery) ([]model.MitigationMeasure, int64, error) {
	return nil, 0, nil
}
func (r *mitigationRepoStub) Resubmit(_ context.Context, measure *model.MitigationMeasure, _ repository.AuditContext) error {
	if r.measure.MitigationStatus != constants.MitigationPending {
		return repository.ErrStateConflict
	}
	measure.MitigationStatus = constants.MitigationPendingReview
	copy := *measure
	r.resubmit = &copy
	return nil
}
func (r *mitigationRepoStub) Review(_ context.Context, id uint, target constants.MitigationStatus, _ uint, _ string, _ repository.AuditContext) error {
	r.reviewed.id, r.reviewed.target = id, target
	return nil
}
func (r *mitigationRepoStub) ActiveForRoute(context.Context, uint) ([]model.MitigationMeasure, error) {
	return nil, nil
}

type mitigationRouteStub struct{ route model.ProcessRoute }

func (r *mitigationRouteStub) Create(context.Context, *model.ProcessRoute, repository.AuditContext) error {
	return nil
}
func (r *mitigationRouteStub) Get(context.Context, uint) (model.ProcessRoute, error) {
	return r.route, nil
}
func (r *mitigationRouteStub) List(context.Context, dto.RouteQuery) ([]model.ProcessRoute, int64, error) {
	return nil, 0, nil
}
func (r *mitigationRouteStub) Update(context.Context, *model.ProcessRoute, uint, repository.AuditContext) error {
	return nil
}

type matrixPreviewStub struct{ result analyzer.Result }

func (s matrixPreviewStub) Preview(context.Context, uint) (analyzer.Result, error) {
	return s.result, nil
}

func highRiskResult() analyzer.Result {
	return analyzer.Result{RiskItems: []analyzer.RiskItem{
		{Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", RawScore: 0.451, RiskLevel: constants.RiskHigh},
		{Allergen: "Milk", SourceStepCode: "FILL-02", TargetStepCode: "PACK-03", RawScore: 0.2, RiskLevel: constants.RiskMedium},
	}}
}

func mitigationService(repo repository.MitigationRepository, route model.ProcessRoute, result analyzer.Result) *MitigationService {
	return NewMitigationService(repo, &mitigationRouteStub{route: route}, matrixPreviewStub{result: result})
}

func analyst() Principal { return Principal{ID: 3, Role: constants.RoleQualityAnalyst} }

func yesterday() string { return time.Now().UTC().AddDate(0, 0, -1).Format("2006-01-02") }

func TestMitigationCreateValidatesInput(t *testing.T) {
	route := model.ProcessRoute{ID: 5, RouteStatus: "active", Version: 4}
	future := time.Now().UTC().AddDate(0, 0, 2).Format("2006-01-02")
	tests := []struct {
		name    string
		request dto.CreateMitigationRequest
		result  analyzer.Result
		code    string
	}{
		{name: "completion after submission", request: dto.CreateMitigationRequest{RouteID: 5, Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", MeasureType: "cleaning", CompletedOn: future, EvidenceNote: "wet clean"}, result: highRiskResult(), code: "completion_after_submission"},
		{name: "unknown path", request: dto.CreateMitigationRequest{RouteID: 5, Allergen: "Soy", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", MeasureType: "cleaning", CompletedOn: yesterday(), EvidenceNote: "wet clean"}, result: highRiskResult(), code: "mitigation_path_missing"},
		{name: "medium risk path", request: dto.CreateMitigationRequest{RouteID: 5, Allergen: "Milk", SourceStepCode: "FILL-02", TargetStepCode: "PACK-03", MeasureType: "line_change", CompletedOn: yesterday(), EvidenceNote: "line change"}, result: highRiskResult(), code: "mitigation_not_high_risk"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			svc := mitigationService(&mitigationRepoStub{}, route, test.result)
			_, err := svc.Create(context.Background(), test.request, analyst(), "req-1")
			var appErr *AppError
			if !errors.As(err, &appErr) || appErr.Code != test.code {
				t.Fatalf("error = %v, want code %q", err, test.code)
			}
		})
	}
}

func TestMitigationCreateSnapshotsPath(t *testing.T) {
	repo := &mitigationRepoStub{}
	route := model.ProcessRoute{ID: 5, RouteStatus: "active", Version: 4}
	svc := mitigationService(repo, route, highRiskResult())
	request := dto.CreateMitigationRequest{RouteID: 5, Allergen: "peanut", SourceStepCode: "mix-01", TargetStepCode: "fill-02", MeasureType: "cleaning", CompletedOn: yesterday(), EvidenceNote: "  WC-18 记录  "}
	measure, err := svc.Create(context.Background(), request, analyst(), "req-2")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if measure.MitigationStatus != constants.MitigationPendingReview {
		t.Fatalf("status = %q, want %q", measure.MitigationStatus, constants.MitigationPendingReview)
	}
	if repo.created == nil {
		t.Fatal("repository create not called")
	}
	if repo.created.Allergen != "Peanut" || repo.created.SourceStepCode != "MIX-01" || repo.created.TargetStepCode != "FILL-02" {
		t.Fatalf("path not normalized: %+v", repo.created)
	}
	if repo.created.RouteVersion != 4 || repo.created.RawScore != 0.451 || repo.created.RiskLevel != constants.RiskHigh {
		t.Fatalf("snapshot mismatch: %+v", repo.created)
	}
	if repo.created.EvidenceNote != "WC-18 记录" {
		t.Fatalf("evidence not trimmed: %q", repo.created.EvidenceNote)
	}
}

func TestMitigationCreateRejectsDuplicateAndRoles(t *testing.T) {
	route := model.ProcessRoute{ID: 5, RouteStatus: "active", Version: 4}
	request := dto.CreateMitigationRequest{RouteID: 5, Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", MeasureType: "cleaning", CompletedOn: yesterday(), EvidenceNote: "wet clean"}
	svc := mitigationService(&mitigationRepoStub{createErr: repository.ErrDuplicate}, route, highRiskResult())
	if _, err := svc.Create(context.Background(), request, analyst(), "req-3"); err == nil || err.(*AppError).Code != "duplicate_pending_measure" {
		t.Fatalf("duplicate error = %v", err)
	}
	svc = mitigationService(&mitigationRepoStub{}, route, highRiskResult())
	if _, err := svc.Create(context.Background(), request, Principal{ID: 9, Role: constants.RoleReviewer}, "req-4"); err == nil || err.(*AppError).Code != "forbidden" {
		t.Fatalf("reviewer create error = %v", err)
	}
}

func TestMitigationResubmitStateMachine(t *testing.T) {
	route := model.ProcessRoute{ID: 5, RouteStatus: "active", Version: 6}
	request := dto.ResubmitMitigationRequest{MeasureType: "line_change", CompletedOn: yesterday(), EvidenceNote: "换线清场记录"}
	pending := model.MitigationMeasure{ID: 7, RouteID: 5, Allergen: "Peanut", SourceStepCode: "MIX-01", TargetStepCode: "FILL-02", MitigationStatus: constants.MitigationPending}
	repo := &mitigationRepoStub{measure: pending}
	svc := mitigationService(repo, route, highRiskResult())
	measure, err := svc.Resubmit(context.Background(), 7, request, analyst(), "req-5")
	if err != nil {
		t.Fatalf("resubmit: %v", err)
	}
	if measure.MitigationStatus != constants.MitigationPendingReview || measure.RouteVersion != 6 || measure.RawScore != 0.451 {
		t.Fatalf("resubmitted measure mismatch: %+v", measure)
	}
	repo.measure.MitigationStatus = constants.MitigationPendingReview
	if _, err := svc.Resubmit(context.Background(), 7, request, analyst(), "req-6"); err == nil || err.(*AppError).Code != "state_conflict" {
		t.Fatalf("resubmit from pending_review error = %v", err)
	}
}

func TestMitigationReviewGuards(t *testing.T) {
	repo := &mitigationRepoStub{measure: model.MitigationMeasure{ID: 7, MitigationStatus: constants.MitigationPendingReview}}
	svc := mitigationService(repo, model.ProcessRoute{ID: 5, RouteStatus: "active"}, highRiskResult())
	if _, err := svc.Review(context.Background(), 7, dto.ReviewMitigationRequest{Decision: "approved", Reason: "证据充分"}, analyst(), "req-7"); err == nil || err.(*AppError).Code != "forbidden" {
		t.Fatalf("analyst review error = %v", err)
	}
	if _, err := svc.Review(context.Background(), 7, dto.ReviewMitigationRequest{Decision: "stale", Reason: "证据充分"}, Principal{ID: 2, Role: constants.RoleReviewer}, "req-8"); err == nil || err.(*AppError).Code != "invalid_decision" {
		t.Fatalf("invalid decision error = %v", err)
	}
	if _, err := svc.Review(context.Background(), 7, dto.ReviewMitigationRequest{Decision: "approved", Reason: "证据充分"}, Principal{ID: 2, Role: constants.RoleReviewer}, "req-9"); err != nil {
		t.Fatalf("review: %v", err)
	}
	if repo.reviewed.id != 7 || repo.reviewed.target != constants.MitigationApproved {
		t.Fatalf("review target = %+v", repo.reviewed)
	}
}

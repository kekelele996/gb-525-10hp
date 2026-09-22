package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"food-allergen-crosscontact-analyzer/backend/internal/constants"
	"food-allergen-crosscontact-analyzer/backend/internal/dto"
	"food-allergen-crosscontact-analyzer/backend/internal/model"
	"food-allergen-crosscontact-analyzer/backend/internal/repository"
	"gorm.io/datatypes"
)

type mitigationRepoStub struct {
	pending  bool
	created  *model.MitigationMeasure
	reviewed struct {
		id     uint
		target constants.MitigationStatus
	}
}

func (r *mitigationRepoStub) Create(_ context.Context, measure *model.MitigationMeasure, _ repository.AuditContext) error {
	measure.ID = 41
	r.created = measure
	return nil
}
func (r *mitigationRepoStub) Get(_ context.Context, id uint) (model.MitigationMeasure, error) {
	return model.MitigationMeasure{ID: id, MitigationStatus: constants.MitigationPendingReview}, nil
}
func (r *mitigationRepoStub) List(context.Context, dto.MitigationQuery) ([]model.MitigationMeasure, int64, error) {
	return nil, 0, nil
}
func (r *mitigationRepoStub) PendingExists(context.Context, uint, string, string, string) (bool, error) {
	return r.pending, nil
}
func (r *mitigationRepoStub) ApprovedForRoute(context.Context, uint) ([]model.MitigationMeasure, error) {
	return nil, nil
}
func (r *mitigationRepoStub) Review(_ context.Context, id uint, target constants.MitigationStatus, _ uint, _ string, _ repository.AuditContext) error {
	r.reviewed.id, r.reviewed.target = id, target
	return nil
}

type routeRepoStub struct{ route model.ProcessRoute }

func (r *routeRepoStub) Create(context.Context, *model.ProcessRoute, repository.AuditContext) error {
	return nil
}
func (r *routeRepoStub) Get(context.Context, uint) (model.ProcessRoute, error) { return r.route, nil }
func (r *routeRepoStub) List(context.Context, dto.RouteQuery) ([]model.ProcessRoute, int64, error) {
	return nil, 0, nil
}
func (r *routeRepoStub) Update(context.Context, *model.ProcessRoute, uint, repository.AuditContext) error {
	return nil
}

type profileRepoStub struct{ profiles []model.AllergenProfile }

func (r *profileRepoStub) Create(context.Context, *model.AllergenProfile, repository.AuditContext) error {
	return nil
}
func (r *profileRepoStub) Get(context.Context, uint) (model.AllergenProfile, error) {
	return model.AllergenProfile{}, nil
}
func (r *profileRepoStub) List(context.Context, dto.ProfileQuery) ([]model.AllergenProfile, int64, error) {
	return nil, 0, nil
}
func (r *profileRepoStub) Update(context.Context, *model.AllergenProfile, uint, repository.AuditContext) error {
	return nil
}
func (r *profileRepoStub) Usage(context.Context, uint) ([]dto.ProfileUsage, error) { return nil, nil }
func (r *profileRepoStub) GetMany(_ context.Context, ids []uint) ([]model.AllergenProfile, error) {
	result := make([]model.AllergenProfile, 0, len(ids))
	for _, profile := range r.profiles {
		for _, id := range ids {
			if profile.ID == id {
				result = append(result, profile)
			}
		}
	}
	return result, nil
}

func mitigationFixture() (*MitigationService, *mitigationRepoStub) {
	steps := datatypes.JSON([]byte(`[{"step_code":"MIX-01","step_name":"Mixing","profile_id":1},{"step_code":"FILL-02","step_name":"Filling","profile_id":2},{"step_code":"PACK-03","step_name":"Packing","profile_id":2}]`))
	route := model.ProcessRoute{ID: 5, RouteCode: "RT-NUT", RouteStatus: "active", Version: 7, OrderedStepsJSON: steps, DeclaredAllergensJSON: datatypes.JSON([]byte(`["Milk"]`))}
	profiles := []model.AllergenProfile{
		{ID: 1, ProfileCode: "MAT-PEANUT", AllergensJSON: datatypes.JSON([]byte(`["Peanut"]`))},
		{ID: 2, ProfileCode: "MAT-MILK", AllergensJSON: datatypes.JSON([]byte(`["Milk"]`))},
	}
	measures := &mitigationRepoStub{}
	return NewMitigationService(measures, &routeRepoStub{route: route}, &profileRepoStub{profiles: profiles}), measures
}

func mitigationRequest(completedAt string) dto.CreateMitigationRequest {
	return dto.CreateMitigationRequest{RouteID: 5, Allergen: "peanut", SourceStepCode: "mix-01", TargetStepCode: "fill-02", MeasureType: "cleaning", CompletedAt: completedAt, EvidenceNote: "Wet clean WC-18 completed and inspected"}
}

func TestMitigationCreateRegistersPendingMeasure(t *testing.T) {
	service, measures := mitigationFixture()
	actor := Principal{ID: 3, Role: constants.RoleQualityAnalyst}
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	measure, err := service.Create(context.Background(), mitigationRequest(yesterday), actor, "req-1")
	if err != nil {
		t.Fatalf("create mitigation: %v", err)
	}
	if measure.ID != 41 || measure.MitigationStatus != constants.MitigationPendingReview {
		t.Fatalf("unexpected measure: %+v", measure)
	}
	stored := measures.created
	if stored.Allergen != "Peanut" || stored.SourceStepCode != "MIX-01" || stored.TargetStepCode != "FILL-02" {
		t.Fatalf("path not normalized: %+v", stored)
	}
	if stored.RouteVersion != 7 || stored.CreatedBy != 3 || stored.MeasureType != constants.MeasureCleaning {
		t.Fatalf("route version or actor missing: %+v", stored)
	}
}

func TestMitigationCreateRejectsFutureCompletion(t *testing.T) {
	service, _ := mitigationFixture()
	actor := Principal{ID: 3, Role: constants.RoleQualityAnalyst}
	tomorrow := time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02")
	_, err := service.Create(context.Background(), mitigationRequest(tomorrow), actor, "req-2")
	app, ok := err.(*AppError)
	if !ok || app.Code != "completion_after_submission" {
		t.Fatalf("expected completion_after_submission, got %v", err)
	}
}

func TestMitigationCreateRejectsDuplicatePending(t *testing.T) {
	service, measures := mitigationFixture()
	measures.pending = true
	actor := Principal{ID: 3, Role: constants.RoleQualityAnalyst}
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	_, err := service.Create(context.Background(), mitigationRequest(yesterday), actor, "req-3")
	app, ok := err.(*AppError)
	if !ok || app.Code != "duplicate_pending_mitigation" {
		t.Fatalf("expected duplicate_pending_mitigation, got %v", err)
	}
}

func TestMitigationCreateValidatesPathAndAllergen(t *testing.T) {
	service, _ := mitigationFixture()
	actor := Principal{ID: 3, Role: constants.RoleQualityAnalyst}
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")

	unknownStep := mitigationRequest(yesterday)
	unknownStep.TargetStepCode = "OVEN-09"
	if _, err := service.Create(context.Background(), unknownStep, actor, "req-4"); err == nil || !strings.Contains(err.Error(), "端点") {
		t.Fatalf("expected unknown step error, got %v", err)
	}

	unknownAllergen := mitigationRequest(yesterday)
	unknownAllergen.Allergen = "Sesame"
	if _, err := service.Create(context.Background(), unknownAllergen, actor, "req-5"); err == nil {
		t.Fatal("expected unknown allergen error")
	}

	selfLoop := mitigationRequest(yesterday)
	selfLoop.SourceStepCode, selfLoop.TargetStepCode = "MIX-01", "MIX-01"
	if _, err := service.Create(context.Background(), selfLoop, actor, "req-6"); err == nil {
		t.Fatal("expected self loop error")
	}
}

func TestMitigationCreateRequiresAnalystRole(t *testing.T) {
	service, _ := mitigationFixture()
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	_, err := service.Create(context.Background(), mitigationRequest(yesterday), Principal{ID: 4, Role: constants.RoleReviewer}, "req-7")
	app, ok := err.(*AppError)
	if !ok || app.Status != 403 {
		t.Fatalf("expected forbidden, got %v", err)
	}
}

func TestMitigationReviewRequiresReviewerRole(t *testing.T) {
	service, measures := mitigationFixture()
	request := dto.ReviewMitigationRequest{Decision: "approved", Reason: "证据充分，清洗记录完整"}
	if _, err := service.Review(context.Background(), 41, request, Principal{ID: 3, Role: constants.RoleQualityAnalyst}, "req-8"); err == nil {
		t.Fatal("analyst must not review mitigations")
	}
	measure, err := service.Review(context.Background(), 41, request, Principal{ID: 8, Role: constants.RoleReviewer}, "req-9")
	if err != nil {
		t.Fatalf("review mitigation: %v", err)
	}
	if measure.ID != 41 || measures.reviewed.target != constants.MitigationApproved {
		t.Fatalf("unexpected review outcome: %+v %+v", measure, measures.reviewed)
	}
}

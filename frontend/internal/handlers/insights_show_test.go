package handlers

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
)

// TestShowInsightsSkipsGetCurrentBabyOnHtmxPartialRequest is a regression
// test for the redundant-refetch fix: a range-pill click or health-history
// toggle is an htmx request that only swaps #insights-workspace, which never
// renders the baby's profile (that's page-shell-only — nav, timezone
// attribute, avatar). Before this fix, ShowInsights fetched it anyway on
// every such request even though nothing in the response used it.
func TestShowInsightsSkipsGetCurrentBabyOnHtmxPartialRequest(t *testing.T) {
	fake := &insightsShowFakeBackend{}
	h := &Handlers{Backend: fake, Templates: insightsShowTestTemplates(t)}

	req := httptest.NewRequest(http.MethodGet, "/insights?category=overview&range=7", nil)
	req.Header.Set("HX-Request", "true")
	recorder := httptest.NewRecorder()

	h.ShowInsights(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if fake.getCurrentBabyCalls != 0 {
		t.Fatalf("GetCurrentBaby called %d times on an htmx partial request, want 0", fake.getCurrentBabyCalls)
	}
	if fake.getOverviewInsightsCalls != 1 {
		t.Fatalf("GetOverviewInsights called %d times, want 1", fake.getOverviewInsightsCalls)
	}
}

// TestShowInsightsFetchesBabyOnFullPageRequest is the companion case: a
// full page load (no HX-Request header) still needs the baby profile for
// the page shell, so GetCurrentBaby must still be called there.
func TestShowInsightsFetchesBabyOnFullPageRequest(t *testing.T) {
	fake := &insightsShowFakeBackend{}
	h := &Handlers{Backend: fake, Templates: insightsShowTestTemplates(t)}

	req := httptest.NewRequest(http.MethodGet, "/insights?category=overview&range=7", nil)
	recorder := httptest.NewRecorder()

	h.ShowInsights(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if fake.getCurrentBabyCalls != 1 {
		t.Fatalf("GetCurrentBaby called %d times on a full page request, want 1", fake.getCurrentBabyCalls)
	}
}

// TestShowInsightsRedirectsToOnboardingWhenBabyNotFound is a regression test:
// ShowInsights stopped fetching the baby profile eagerly (that's the whole
// point of the redundant-refetch fix above), so the no-baby-yet condition —
// previously caught by GetCurrentBaby's own ErrNotFound check before any
// category data was fetched — now first surfaces on the category fetch
// itself. Every Insights category endpoint 404s the same way GetCurrentBaby
// used to (both go through backend-api's currentBabyForRequest), so losing
// that check here silently turned "redirect to onboarding" into a 502 for
// any user without a baby yet.
func TestShowInsightsRedirectsToOnboardingWhenBabyNotFound(t *testing.T) {
	fake := &insightsShowFakeBackend{overviewInsightsErr: backendclient.ErrNotFound}
	h := &Handlers{Backend: fake, Templates: insightsShowTestTemplates(t)}

	req := httptest.NewRequest(http.MethodGet, "/insights?category=overview&range=7", nil)
	recorder := httptest.NewRecorder()

	h.ShowInsights(recorder, req)

	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d (redirect to onboarding)", recorder.Code, http.StatusSeeOther)
	}
	if got := recorder.Header().Get("Location"); got != "/onboarding" {
		t.Fatalf("Location = %q, want /onboarding", got)
	}
	if fake.getCurrentBabyCalls != 0 {
		t.Fatalf("GetCurrentBaby called %d times, want 0 — the not-found signal came from the category fetch itself", fake.getCurrentBabyCalls)
	}
}

// insightsShowTestTemplates stands in for the real templates: ShowInsights
// only needs "insights-workspace" and "insights" to exist and execute
// without erroring, not to produce the real page — the call-count behavior
// under test happens entirely before either is rendered.
func insightsShowTestTemplates(t *testing.T) *template.Template {
	t.Helper()
	templates := template.Must(template.New("insights-workspace").Parse("workspace"))
	template.Must(templates.New("insights").Parse("page"))
	return templates
}

// insightsShowFakeBackend implements Backend, tracking the two calls this
// test cares about and erroring on everything else it doesn't stub out.
type insightsShowFakeBackend struct {
	getCurrentBabyCalls      int
	getOverviewInsightsCalls int
	// overviewInsightsErr, when set, is returned by GetOverviewInsights
	// instead of a successful response — used to simulate backend-api 404ing
	// (surfaced as backendclient.ErrNotFound) when the family has no baby.
	overviewInsightsErr error
}

func (f *insightsShowFakeBackend) GetCurrentBaby(context.Context) (backendclient.Baby, error) {
	f.getCurrentBabyCalls++
	return backendclient.Baby{Name: "Test Baby", Timezone: "UTC"}, nil
}

func (f *insightsShowFakeBackend) GetOverviewInsights(context.Context, int) (backendclient.OverviewInsights, error) {
	f.getOverviewInsightsCalls++
	if f.overviewInsightsErr != nil {
		return backendclient.OverviewInsights{}, f.overviewInsightsErr
	}
	return backendclient.OverviewInsights{}, nil
}

func (f *insightsShowFakeBackend) GetCurrentUser(context.Context) (backendclient.User, error) {
	return backendclient.User{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) UpdateCurrentUser(context.Context, string) (backendclient.User, error) {
	return backendclient.User{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) CreateBaby(context.Context, string) (backendclient.Baby, error) {
	return backendclient.Baby{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) UpdateCurrentBaby(context.Context, backendclient.Baby) (backendclient.Baby, error) {
	return backendclient.Baby{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) ArchiveCurrentBaby(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) ListEvents(context.Context, string, string, any) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetDailyReport(context.Context, string) (backendclient.DailyReport, error) {
	return backendclient.DailyReport{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetSleepInsights(context.Context, int) (backendclient.SleepInsights, error) {
	return backendclient.SleepInsights{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetGrowthInsights(context.Context, string, int) (backendclient.GrowthInsights, error) {
	return backendclient.GrowthInsights{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetNappyInsights(context.Context, int) (backendclient.NappyInsights, error) {
	return backendclient.NappyInsights{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetFeedInsights(context.Context, int) (backendclient.FeedInsights, error) {
	return backendclient.FeedInsights{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) GetPumpInsights(context.Context, int) (backendclient.PumpInsights, error) {
	return backendclient.PumpInsights{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) CreateEvent(context.Context, string, map[string]any) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) UpdateEvent(context.Context, string, map[string]any) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) DeleteEvent(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) InviteHelper(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) ListTimelineMembers(context.Context) (backendclient.TimelineMembersResult, error) {
	return backendclient.TimelineMembersResult{}, errors.New("not implemented")
}

func (f *insightsShowFakeBackend) UpdateTimelineMemberRelationship(context.Context, string, string) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) UpdateTimelineMemberReportPreferences(context.Context, string, bool) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) RemoveTimelineMember(context.Context, string) error {
	return errors.New("not implemented")
}

func (f *insightsShowFakeBackend) Unsubscribe(context.Context, string, string, string) error {
	return errors.New("not implemented")
}

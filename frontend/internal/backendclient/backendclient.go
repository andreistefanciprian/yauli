// Package backendclient holds the backend-api response types and the HTTP
// client that fetches them. See internal/handlers for the interface that
// consumes this package.
package backendclient

import (
	"errors"
	"time"
)

var ErrForbidden = errors.New("forbidden")
var ErrNotFound = errors.New("not found")

// APIError carries backend-api's own {"error": "..."} message through to the
// caller, so a 400 validation failure (e.g. "occurred_at cannot be in the
// future") can be shown to the user instead of a generic failure message.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return e.Message
}

type Baby struct {
	ID               string `json:"id"`
	FamilyID         string `json:"family_id"`
	Name             string `json:"name"`
	Timezone         string `json:"timezone"`
	BirthDate        string `json:"birth_date,omitempty"`
	BirthWeightKg    string `json:"birth_weight_kg,omitempty"`
	BirthLengthCm    string `json:"birth_length_cm,omitempty"`
	Sex              string `json:"sex,omitempty"`
	CanInvite        bool   `json:"can_invite"`
	HasPendingInvite bool   `json:"has_pending_invite"`
}

type User struct {
	ID          string `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
}

type TimelineMember struct {
	UserID                  string `json:"user_id"`
	Email                   string `json:"email"`
	Role                    string `json:"role"`
	Status                  string `json:"status"`
	Relationship            string `json:"relationship,omitempty"`
	DailyReportEmailEnabled bool   `json:"daily_report_email_enabled"`
}

type TimelineMembersResult struct {
	Members []TimelineMember `json:"members"`
}

// Event is a generic event exactly as backend-api's combined /events
// endpoint returns it: event_type plus its type-specific attributes, not a
// typed per-event shape. Interpreting Attributes is internal/handlers' job,
// same division of responsibility as backend-api's own store.Event.
type Event struct {
	ID         string         `json:"id"`
	BabyID     string         `json:"baby_id"`
	EventType  string         `json:"event_type"`
	Attributes map[string]any `json:"attributes"`
	OccurredAt time.Time      `json:"occurred_at"`
	CreatedAt  time.Time      `json:"created_at"`
}

type DailyReport struct {
	Title string           `json:"title"`
	Card  *DailyReportCard `json:"card,omitempty"`
}

type DailyReportCard struct {
	Metrics []DailyReportMetric `json:"metrics"`
}

type DailyReportMetric struct {
	Key    string `json:"key"`
	Count  int    `json:"count"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// SleepInsights is backend-api's fully-computed Sleep Insights payload for a
// 7/30/90-day range ending today — per-day breakdowns plus range aggregates,
// with every display string already formatted. The frontend only lays it out.
type SleepInsights struct {
	RangeDays          int                   `json:"range_days"`
	RangeLabel         string                `json:"range_label"`
	RangeStartsAtBirth bool                  `json:"range_starts_at_birth"`
	Days               []SleepInsightDay     `json:"days"`
	Aggregate          SleepInsightAggregate `json:"aggregate"`
	Observations       []string              `json:"observations"`
}

type SleepInsightDay struct {
	LocalDate      string               `json:"local_date"`
	Label          string               `json:"label"`
	ShowLabel      bool                 `json:"show_label"`
	FullLabel      string               `json:"full_label"`
	HasData        bool                 `json:"has_data"`
	TotalMinutes   int                  `json:"total_minutes"`
	TotalLabel     string               `json:"total_label,omitempty"`
	CompletedCount int                  `json:"completed_count"`
	LongestMinutes int                  `json:"longest_minutes"`
	LongestLabel   string               `json:"longest_label,omitempty"`
	NapMinutes     int                  `json:"nap_minutes"`
	NightMinutes   int                  `json:"night_minutes"`
	NapNightLabel  string               `json:"nap_night_label,omitempty"`
	CarryoverNote  string               `json:"carryover_note,omitempty"`
	Periods        []SleepInsightPeriod `json:"periods"`
}

type SleepInsightPeriod struct {
	Type               string `json:"type"` // "nap" or "night"
	StartMinutes       int    `json:"start_minutes"`
	EndMinutes         int    `json:"end_minutes"`
	DurationMinutes    int    `json:"duration_minutes"`
	Ongoing            bool   `json:"ongoing"`
	StartedPreviousDay bool   `json:"started_previous_day"`
	ContinuesNextDay   bool   `json:"continues_next_day"`
	TimeRangeLabel     string `json:"time_range_label"`
	DurationLabel      string `json:"duration_label"`
}

type SleepInsightAggregate struct {
	HasAnyData               bool   `json:"has_any_data"`
	RecordedDays             int    `json:"recorded_days"`
	PeriodCount              int    `json:"period_count"`
	PeriodsWithDurationCount int    `json:"periods_with_duration_count"`
	AverageTotalLabel        string `json:"average_total_label,omitempty"`
	AverageTotalBasisLabel   string `json:"average_total_basis_label,omitempty"`
	AverageCompletedLabel    string `json:"average_completed_label,omitempty"`
	LongestOverallLabel      string `json:"longest_overall_label,omitempty"`
	HasWakeWindow            bool   `json:"has_wake_window"`
	AverageWakeWindowLabel   string `json:"average_wake_window_label,omitempty"`
	AverageWakeWindowCaption string `json:"average_wake_window_caption,omitempty"`
	NapPercent               *int   `json:"nap_percent,omitempty"`
	NightPercent             *int   `json:"night_percent,omitempty"`
}

// GrowthInsights is backend-api's fully-computed Growth Insights payload for
// one metric (weight/length/head circumference) over a range ending today —
// one entry per recorded measurement plus range aggregates, with every
// display string already formatted. The frontend only lays it out.
type GrowthInsights struct {
	Metric       string                 `json:"metric"`
	MetricLabel  string                 `json:"metric_label"`
	Unit         string                 `json:"unit"`
	RangeDays    int                    `json:"range_days"`
	RangeLabel   string                 `json:"range_label"`
	RangeStart   *time.Time             `json:"range_start,omitempty"`
	RangeEnd     time.Time              `json:"range_end"`
	HasAnyData   bool                   `json:"has_any_data"`
	Points       []GrowthInsightPoint   `json:"points"`
	Aggregate    GrowthInsightAggregate `json:"aggregate"`
	Observations []string               `json:"observations"`
}

type GrowthInsightPoint struct {
	ID          string    `json:"id"`
	OccurredAt  time.Time `json:"occurred_at"`
	LocalDate   string    `json:"local_date"`
	Label       string    `json:"label"`
	ShowLabel   bool      `json:"show_label"`
	FullLabel   string    `json:"full_label"`
	Value       float64   `json:"value"`
	ValueLabel  string    `json:"value_label"`
	ChangeLabel string    `json:"change_label"`
}

type GrowthInsightAggregate struct {
	Count                    int    `json:"count"`
	LatestValueLabel         string `json:"latest_value_label,omitempty"`
	AverageIntervalDaysLabel string `json:"average_interval_days_label,omitempty"`
	AverageIntervalCaption   string `json:"average_interval_caption,omitempty"`
	ChangeOverallLabel       string `json:"change_overall_label,omitempty"`
	ChangeOverallCaption     string `json:"change_overall_caption,omitempty"`
}

// NappyInsights is backend-api's fully-computed Nappy Insights payload for a
// 7/30/90-day range ending today, mirroring SleepInsights' own shape and
// window so the two categories share the same range pills. The frontend
// only lays it out.
type NappyInsights struct {
	RangeDays          int                   `json:"range_days"`
	RangeLabel         string                `json:"range_label"`
	RangeStartsAtBirth bool                  `json:"range_starts_at_birth"`
	Days               []NappyInsightDay     `json:"days"`
	Aggregate          NappyInsightAggregate `json:"aggregate"`
	Observations       []string              `json:"observations"`
}

type NappyInsightDay struct {
	LocalDate  string              `json:"local_date"`
	Label      string              `json:"label"`
	ShowLabel  bool                `json:"show_label"`
	FullLabel  string              `json:"full_label"`
	HasData    bool                `json:"has_data"`
	TotalCount int                 `json:"total_count"`
	WeeCount   int                 `json:"wee_count"`
	PooCount   int                 `json:"poo_count"`
	MixedCount int                 `json:"mixed_count"`
	Events     []NappyInsightEvent `json:"events"`
}

type NappyInsightEvent struct {
	Kind      string `json:"kind"`           // "wee", "poo", or "mixed"
	Size      string `json:"size,omitempty"` // "smear", "small", "medium", "large", or "blowout"; empty for wee-only events
	TimeLabel string `json:"time_label"`
}

type NappyInsightAggregate struct {
	HasAnyData         bool   `json:"has_any_data"`
	RecordedDays       int    `json:"recorded_days"`
	TotalCount         int    `json:"total_count"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	HasAverageGap      bool   `json:"has_average_gap"`
	AverageGapLabel    string `json:"average_gap_label,omitempty"`
	AverageGapCaption  string `json:"average_gap_caption,omitempty"`
	WeePercent         *int   `json:"wee_percent,omitempty"`
	PooPercent         *int   `json:"poo_percent,omitempty"`
	MixedPercent       *int   `json:"mixed_percent,omitempty"`
	BlowoutCount       int    `json:"blowout_count"`
	LargeCount         int    `json:"large_count"`
}

// FeedInsights is backend-api's fully-computed Feed Insights payload for a
// 7/30/90-day range ending today, mirroring SleepInsights' and
// NappyInsights' own shape and window so all three categories share the
// same range pills. Each day carries both breast (time-based) and
// formula/expressed (volume-based) totals so the frontend can switch
// between the two metric views without a second request. The frontend only
// lays it out.
type FeedInsights struct {
	RangeDays          int                  `json:"range_days"`
	RangeLabel         string               `json:"range_label"`
	RangeStartsAtBirth bool                 `json:"range_starts_at_birth"`
	Days               []FeedInsightDay     `json:"days"`
	Aggregate          FeedInsightAggregate `json:"aggregate"`
	Observations       []string             `json:"observations"`
}

type FeedInsightDay struct {
	LocalDate      string             `json:"local_date"`
	Label          string             `json:"label"`
	ShowLabel      bool               `json:"show_label"`
	FullLabel      string             `json:"full_label"`
	HasData        bool               `json:"has_data"`
	TotalCount     int                `json:"total_count"`
	BreastCount    int                `json:"breast_count"`
	FormulaCount   int                `json:"formula_count"`
	ExpressedCount int                `json:"expressed_count"`
	BreastMinutes  int                `json:"breast_minutes"`
	FormulaMl      int                `json:"formula_ml"`
	ExpressedMl    int                `json:"expressed_ml"`
	BottleMl       int                `json:"bottle_ml"`
	Events         []FeedInsightEvent `json:"events"`
}

type FeedInsightEvent struct {
	Kind        string `json:"kind"` // "breast", "formula", or "expressed"
	TimeLabel   string `json:"time_label"`
	DetailLabel string `json:"detail_label"`
}

type FeedInsightAggregate struct {
	HasAnyData                   bool   `json:"has_any_data"`
	RecordedDays                 int    `json:"recorded_days"`
	TotalCount                   int    `json:"total_count"`
	BreastCount                  int    `json:"breast_count"`
	FormulaCount                 int    `json:"formula_count"`
	ExpressedCount               int    `json:"expressed_count"`
	AveragePerDayLabel           string `json:"average_per_day_label,omitempty"`
	HasAverageGap                bool   `json:"has_average_gap"`
	AverageGapLabel              string `json:"average_gap_label,omitempty"`
	AverageGapCaption            string `json:"average_gap_caption,omitempty"`
	BreastTotalMinutes           int    `json:"breast_total_minutes"`
	BreastTotalLabel             string `json:"breast_total_label,omitempty"`
	BreastFeedsWithDurationCount int    `json:"breast_feeds_with_duration_count"`
	BreastDurationBasisLabel     string `json:"breast_duration_basis_label,omitempty"`
	BottleTotalMl                int    `json:"bottle_total_ml"`
	BottleTotalLabel             string `json:"bottle_total_label,omitempty"`
	BreastPercent                *int   `json:"breast_percent,omitempty"`
	FormulaPercent               *int   `json:"formula_percent,omitempty"`
	ExpressedPercent             *int   `json:"expressed_percent,omitempty"`
}

// PumpInsights is backend-api's fully-computed Pump Insights payload for a
// 7/30/90-day range ending today, mirroring FeedInsights' own shape and
// window so all Insights categories share the same range pills. Each day
// carries both total volume (ml) and total duration (minutes) so the
// frontend can switch between the two metric views without a second
// request. The frontend only lays it out.
type PumpInsights struct {
	RangeDays          int                  `json:"range_days"`
	RangeLabel         string               `json:"range_label"`
	RangeStartsAtBirth bool                 `json:"range_starts_at_birth"`
	Days               []PumpInsightDay     `json:"days"`
	Aggregate          PumpInsightAggregate `json:"aggregate"`
	Observations       []string             `json:"observations"`
}

type PumpInsightDay struct {
	LocalDate     string             `json:"local_date"`
	Label         string             `json:"label"`
	ShowLabel     bool               `json:"show_label"`
	FullLabel     string             `json:"full_label"`
	HasData       bool               `json:"has_data"`
	SessionCount  int                `json:"session_count"`
	TotalMl       int                `json:"total_ml"`
	TotalMinutes  int                `json:"total_minutes"`
	DurationLabel string             `json:"duration_label,omitempty"`
	Events        []PumpInsightEvent `json:"events"`
}

type PumpInsightEvent struct {
	TimeLabel     string `json:"time_label"`
	VolumeLabel   string `json:"volume_label"`
	DurationLabel string `json:"duration_label,omitempty"`
}

type PumpInsightAggregate struct {
	HasAnyData                    bool   `json:"has_any_data"`
	RecordedDays                  int    `json:"recorded_days"`
	SessionCount                  int    `json:"session_count"`
	SessionsWithDurationCount     int    `json:"sessions_with_duration_count"`
	TotalMl                       int    `json:"total_ml"`
	TotalMlLabel                  string `json:"total_ml_label,omitempty"`
	TotalMinutes                  int    `json:"total_minutes"`
	TotalDurationLabel            string `json:"total_duration_label,omitempty"`
	DurationBasisLabel            string `json:"duration_basis_label,omitempty"`
	AveragePerDayLabel            string `json:"average_per_day_label,omitempty"`
	HasAverageGap                 bool   `json:"has_average_gap"`
	AverageGapLabel               string `json:"average_gap_label,omitempty"`
	AverageGapCaption             string `json:"average_gap_caption,omitempty"`
	AverageSessionMlLabel         string `json:"average_session_ml_label,omitempty"`
	AverageSessionDurationLabel   string `json:"average_session_duration_label,omitempty"`
	AverageSessionDurationCaption string `json:"average_session_duration_caption,omitempty"`
}

// OverviewInsights is backend-api's fully-computed payload for the Insights
// Overview tab's "recorded stats" card — one aggregate figure per category
// rather than the day-by-day detail SleepInsights/FeedInsights/etc. carry.
// Sleep/Feed/Nappy/Pump follow the requested range; Growth and Health report
// against the whole recorded history. Each category reports whether its
// source was available so a partial backend failure is distinct from a
// successful empty result. Every display string is pre-formatted here; the
// frontend only lays it out.
type OverviewInsights struct {
	RangeDays int `json:"range_days"`
	// AgeLabel is how old the baby is as of today in their own timezone
	// (e.g. "6 weeks, 3 days old"), computed by backend-api since it depends
	// on that timezone's "today" — omitted when the baby has no birth date or
	// its birth date is still in the future.
	// BirthDateLabel ("12 June 2026") travels alongside it for the same
	// reason: neither needs the baby's profile fetched separately, which is
	// what lets ShowInsights skip GetCurrentBaby on htmx partial re-renders.
	AgeLabel       string              `json:"age_label,omitempty"`
	BirthDateLabel string              `json:"birth_date_label,omitempty"`
	Sleep          OverviewSleepStats  `json:"sleep"`
	Feed           OverviewFeedStats   `json:"feed"`
	Nappy          OverviewNappyStats  `json:"nappy"`
	Pump           OverviewPumpStats   `json:"pump"`
	Growth         OverviewGrowthStats `json:"growth"`
	Health         OverviewHealthStats `json:"health"`
}

type OverviewSleepStats struct {
	Available              bool   `json:"available"`
	HasAnyData             bool   `json:"has_any_data"`
	AverageTotalLabel      string `json:"average_total_label,omitempty"`
	NightPercent           *int   `json:"night_percent,omitempty"`
	HasWakeWindow          bool   `json:"has_wake_window"`
	AverageWakeWindowLabel string `json:"average_wake_window_label,omitempty"`
}

type OverviewFeedStats struct {
	Available          bool   `json:"available"`
	HasAnyData         bool   `json:"has_any_data"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	BreastTotalLabel   string `json:"breast_total_label,omitempty"`
	// FormulaTotalLabel/ExpressedTotalLabel each switch from "ml" to "L" at
	// 1000 ml — see backend-api's feedVolumeLabel.
	FormulaTotalLabel   string `json:"formula_total_label,omitempty"`
	ExpressedTotalLabel string `json:"expressed_total_label,omitempty"`
}

type OverviewNappyStats struct {
	Available          bool   `json:"available"`
	HasAnyData         bool   `json:"has_any_data"`
	AveragePerDayLabel string `json:"average_per_day_label,omitempty"`
	HasAverageGap      bool   `json:"has_average_gap"`
	AverageGapLabel    string `json:"average_gap_label,omitempty"`
}

type OverviewPumpStats struct {
	Available    bool   `json:"available"`
	HasAnyData   bool   `json:"has_any_data"`
	SessionCount int    `json:"session_count"`
	TotalMlLabel string `json:"total_ml_label,omitempty"`
}

type OverviewGrowthStats struct {
	Available             bool   `json:"available"`
	HasAnyData            bool   `json:"has_any_data"`
	LatestValueLabel      string `json:"latest_value_label,omitempty"`
	HasBirthWeight        bool   `json:"has_birth_weight"`
	ChangeSinceBirthLabel string `json:"change_since_birth_label,omitempty"`

	// Length mirrors the weight fields above exactly — see
	// backend-api's overviewGrowthStats.
	HasLengthData               bool   `json:"has_length_data"`
	LatestLengthLabel           string `json:"latest_length_label,omitempty"`
	HasBirthLength              bool   `json:"has_birth_length"`
	LengthChangeSinceBirthLabel string `json:"length_change_since_birth_label,omitempty"`
}

// OverviewHealthStats is the Overview tab's "Health & medicine" card
// payload — vaccination and medicine history, always reported against the
// whole recorded history rather than the range pill (same reasoning as
// Growth: "3 recorded" wouldn't mean anything scoped to a week).
type OverviewHealthStats struct {
	Available bool `json:"available"`

	VaccinationCount int                   `json:"vaccination_count"`
	HasVaccinations  bool                  `json:"has_vaccinations"`
	RecentGroupLabel string                `json:"recent_group_label,omitempty"`
	RecentDateLabel  string                `json:"recent_date_label,omitempty"`
	RecentAgeLabel   string                `json:"recent_age_label,omitempty"`
	VaccineHistory   []OverviewHealthEvent `json:"vaccine_history,omitempty"`

	MedicineCount           int                   `json:"medicine_count"`
	HasMedicine             bool                  `json:"has_medicine"`
	RecentMedicineNameLabel string                `json:"recent_medicine_name_label,omitempty"`
	RecentMedicineDateLabel string                `json:"recent_medicine_date_label,omitempty"`
	RecentMedicineAgeLabel  string                `json:"recent_medicine_age_label,omitempty"`
	MedicineHistory         []OverviewHealthEvent `json:"medicine_history,omitempty"`

	// Other covers medication items logged with kind "other" — mirrors
	// Medicine's shape exactly.
	OtherCount           int                   `json:"other_count"`
	HasOther             bool                  `json:"has_other"`
	RecentOtherNameLabel string                `json:"recent_other_name_label,omitempty"`
	RecentOtherDateLabel string                `json:"recent_other_date_label,omitempty"`
	RecentOtherAgeLabel  string                `json:"recent_other_age_label,omitempty"`
	OtherHistory         []OverviewHealthEvent `json:"other_history,omitempty"`
}

// OverviewHealthEvent is one recorded vaccine dose, medicine dose, or other
// medication item.
type OverviewHealthEvent struct {
	NameLabel        string `json:"name_label"`
	DescriptionLabel string `json:"description_label,omitempty"`
	WhenLabel        string `json:"when_label"`
}

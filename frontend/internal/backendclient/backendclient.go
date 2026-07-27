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
	RangeDays    int                   `json:"range_days"`
	RangeLabel   string                `json:"range_label"`
	Days         []SleepInsightDay     `json:"days"`
	Aggregate    SleepInsightAggregate `json:"aggregate"`
	Observations []string              `json:"observations"`
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
	Periods        []SleepInsightPeriod `json:"periods"`
}

type SleepInsightPeriod struct {
	Type            string `json:"type"` // "nap" or "night"
	StartMinutes    int    `json:"start_minutes"`
	EndMinutes      int    `json:"end_minutes"`
	DurationMinutes int    `json:"duration_minutes"`
	Ongoing         bool   `json:"ongoing"`
	TimeRangeLabel  string `json:"time_range_label"`
	DurationLabel   string `json:"duration_label"`
}

type SleepInsightAggregate struct {
	HasAnyData               bool   `json:"has_any_data"`
	AverageTotalLabel        string `json:"average_total_label,omitempty"`
	AverageCompletedLabel    string `json:"average_completed_label,omitempty"`
	LongestOverallLabel      string `json:"longest_overall_label,omitempty"`
	HasWakeWindow            bool   `json:"has_wake_window"`
	AverageWakeWindowLabel   string `json:"average_wake_window_label,omitempty"`
	AverageWakeWindowCaption string `json:"average_wake_window_caption,omitempty"`
	NapPercent               *int   `json:"nap_percent,omitempty"`
	NightPercent             *int   `json:"night_percent,omitempty"`
}

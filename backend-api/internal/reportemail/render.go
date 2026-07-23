package reportemail

import (
	"fmt"
	"strings"
	"time"
)

const reportEncouragement = "You've got this."

// cardMetricColors are the label tints for the KPI card's columns, cycling
// if there are ever more metrics than colors. They mirror the event-type
// accent hues used elsewhere in Yauli's brand (web app card tints), adapted
// to hold up against the card's light blue background.
var cardMetricColors = []string{"#8F5A2B", "#6E4E96", "#B5652F", "#9C7A4E"}

func textBody(report Report) string {
	var b strings.Builder
	b.WriteString(report.Output.Title)
	b.WriteString("\n\n")
	b.WriteString(report.Output.Summary)
	b.WriteString("\n")

	if len(report.Card) > 0 {
		b.WriteString("\n")
		parts := make([]string, 0, len(report.Card))
		for _, metric := range report.Card {
			part := fmt.Sprintf("%s: %d", metric.Label, metric.Count)
			if metric.Detail != "" {
				part += fmt.Sprintf(" (%s)", metric.Detail)
			}
			parts = append(parts, part)
		}
		b.WriteString(strings.Join(parts, " · "))
		b.WriteString("\n")
	}

	writeTextList(&b, "Highlights", report.Output.Highlights)
	writeTextList(&b, "Patterns", report.Output.Patterns)
	writeTextList(&b, "Comparison", report.Output.Comparison)
	writeTextList(&b, "Caveats", report.Output.Caveats)
	writeTextTrend(&b, report.Trend)

	b.WriteString("\n")
	b.WriteString(reportEncouragement)
	b.WriteString("\n")

	b.WriteString("\nReport window: ")
	b.WriteString(report.StartDate)
	if report.EndDate != "" && report.EndDate != report.StartDate {
		b.WriteString(" to ")
		b.WriteString(report.EndDate)
	}
	b.WriteString("\n")

	return b.String()
}

func writeTextTrend(b *strings.Builder, days []TrendDay) {
	if len(days) == 0 {
		return
	}

	b.WriteString("\nLast 7 days:\n")
	for _, day := range days {
		fmt.Fprintf(
			b,
			"%s: Sleep %.1fh · Feeds %d (%s, %d mL bottle) · Pump %d mL (%s) · Nappies %d\n",
			day.Label,
			day.SleepHours,
			day.FeedCount,
			formatTrendDurationMinutes(day.FeedDurationMinutes),
			day.FeedBottleMl,
			day.PumpMl,
			formatTrendDurationMinutes(day.PumpDurationMinutes),
			day.NappyCount,
		)
	}
}

func writeTextList(b *strings.Builder, heading string, items []string) {
	if len(items) == 0 {
		return
	}
	b.WriteString("\n")
	b.WriteString(heading)
	b.WriteString(":\n")
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		b.WriteString("- ")
		b.WriteString(item)
		b.WriteString("\n")
	}
}

// formatTrendDurationMinutes renders a duration the way the trend charts'
// design mockup does: "1h 44m" once there's at least an hour, "16 min"
// otherwise. This is deliberately separate from the web app / KPI card's own
// duration formatting (report.go's formatCompactDurationMinutes) — the two
// are allowed to diverge since they serve different, independently designed
// surfaces.
func formatTrendDurationMinutes(minutes int) string {
	hours := minutes / 60
	remaining := minutes % 60
	if hours == 0 {
		return fmt.Sprintf("%d min", remaining)
	}
	if remaining == 0 {
		return fmt.Sprintf("%dh", hours)
	}
	return fmt.Sprintf("%dh %dm", hours, remaining)
}

// reportDateHeading formats the report's start date the way the KPI card
// mockup does ("Sunday, July 20"), falling back to the raw string if it
// cannot be parsed (StartDate is always a plain YYYY-MM-DD outside tests).
func reportDateHeading(report Report) string {
	parsed, err := time.Parse("2006-01-02", report.StartDate)
	if err != nil {
		return report.StartDate
	}
	return parsed.Format("Monday, January 2")
}

// reportSubtitle renders the small muted line under the date heading, the
// same "{baby}'s day, summarised" pattern the Reports page uses, adapted to
// whatever period the report covers (daily, weekly, ...).
func reportSubtitle(report Report) string {
	period := reportPeriodNoun(report.ReportType)
	name := strings.TrimSpace(report.BabyName)
	if name == "" {
		return fmt.Sprintf("%s, summarised", period)
	}
	return fmt.Sprintf("%s's %s, summarised", name, period)
}

func reportPeriodNoun(reportType string) string {
	switch strings.ToLower(strings.TrimSpace(reportType)) {
	case "daily":
		return "day"
	case "weekly":
		return "week"
	case "":
		return "report"
	default:
		return reportType
	}
}

package main

import (
	"bytes"
	"encoding/xml"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andreistefanciprian/yauli/frontend/internal/backendclient"
	"github.com/andreistefanciprian/yauli/frontend/internal/handlers"
)

func TestIconTemplatesRenderSVG(t *testing.T) {
	templates := parseFrontendTemplates(t)

	tests := []struct {
		name         string
		templateName string
		values       []string
	}{
		{
			name:         "event type",
			templateName: "event-type-icon",
			values:       []string{"nappy", "feed", "pump", "bath", "sleep", "observation", "temperature", "medication", "growth_measurement"},
		},
		{
			name:         "nappy kind",
			templateName: "nappy-kind-icon",
			values:       []string{"wet", "poo", "both"},
		},
		{
			name:         "poo size",
			templateName: "nappy-poo-size-icon",
			values:       []string{"smear", "small", "medium", "large", "blowout"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.values {
				t.Run(value, func(t *testing.T) {
					var rendered bytes.Buffer
					if err := templates.ExecuteTemplate(&rendered, test.templateName, value); err != nil {
						t.Fatalf("render icon: %v", err)
					}
					if !strings.Contains(rendered.String(), "<svg") {
						t.Fatalf("icon = %q, want inline SVG", rendered.String())
					}
				})
			}
		})
	}
}

func TestMedicationIconUsesTokenBasedPresentation(t *testing.T) {
	templates := parseFrontendTemplates(t)
	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "medication-icon", nil); err != nil {
		t.Fatalf("render medication icon: %v", err)
	}
	icon := rendered.String()
	if !strings.Contains(icon, "<svg") || !strings.Contains(icon, `stroke="currentColor"`) {
		t.Fatalf("medication icon = %q, want token-based capsule SVG", icon)
	}
	if strings.Contains(icon, "#") || strings.Contains(icon, "style=") {
		t.Fatalf("medication icon contains hardcoded presentation values: %q", icon)
	}
}

func TestMedicationItemRendersInteractiveFields(t *testing.T) {
	templates := parseFrontendTemplates(t)
	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "medication-item", map[string]string{"Index": "0", "Number": "1"}); err != nil {
		t.Fatalf("render medication item: %v", err)
	}
	item := rendered.String()
	for _, want := range []string{
		`data-medication-item`,
		`data-medication-item-toggle`,
		`name="medication_item" value="0"`,
		`name="item_kind_0"`,
		`name="item_name_0"`,
		`name="item_description_0"`,
		`data-medication-remove-item`,
		`data-medication-item-done`,
		`medication-item-edit-label">Edit`,
	} {
		if !strings.Contains(item, want) {
			t.Fatalf("medication item does not contain %q: %s", want, item)
		}
	}
}

func TestStaticAssetURLsChangeWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.js")
	if err := os.WriteFile(path, []byte("first"), 0o600); err != nil {
		t.Fatalf("write first asset: %v", err)
	}

	firstURLs, err := staticAssetURLs(dir)
	if err != nil {
		t.Fatalf("fingerprint first asset: %v", err)
	}
	firstURL, err := staticAssetURL(firstURLs, "app.js")
	if err != nil {
		t.Fatalf("get first asset URL: %v", err)
	}
	if !strings.HasPrefix(firstURL, "/static/app.js?v=") {
		t.Fatalf("first asset URL = %q", firstURL)
	}

	if err := os.WriteFile(path, []byte("second"), 0o600); err != nil {
		t.Fatalf("write second asset: %v", err)
	}
	secondURLs, err := staticAssetURLs(dir)
	if err != nil {
		t.Fatalf("fingerprint second asset: %v", err)
	}
	secondURL, err := staticAssetURL(secondURLs, "app.js")
	if err != nil {
		t.Fatalf("get second asset URL: %v", err)
	}
	if secondURL == firstURL {
		t.Fatalf("asset URL did not change after content changed: %q", secondURL)
	}
}

func TestStaticAssetURLRejectsUnknownAsset(t *testing.T) {
	if _, err := staticAssetURL(map[string]string{}, "missing.js"); err == nil {
		t.Fatal("staticAssetURL accepted an unknown asset")
	}
}

func TestDiscoveryFiles(t *testing.T) {
	handler := http.FileServer(http.Dir("../../static"))

	tests := []struct {
		path        string
		contentType string
		contains    []string
	}{
		{
			path:        "/robots.txt",
			contentType: "text/plain",
			contains: []string{
				"User-agent: *",
				"Allow: /",
				"Disallow: /app",
				"Disallow: /insights",
				"Sitemap: https://getyauli.com/sitemap.xml",
			},
		},
		{
			path:        "/sitemap.xml",
			contentType: "xml",
			contains: []string{
				`<loc>https://getyauli.com/</loc>`,
			},
		},
		{
			path:        "/llms.txt",
			contentType: "text/plain",
			contains: []string{
				"# Yauli",
				"> Yauli is a baby tracking and parenting companion",
				"7-, 30-, or 90-day insights for sleep, feeds, nappies, and growth",
				"medication, vaccines, other care items",
				"Scheduled weekly reports, dedicated milestone views",
				"[Yauli homepage](https://getyauli.com/)",
				"not yet generally available",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			response := httptest.NewRecorder()

			handler.ServeHTTP(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusOK, response.Body.String())
			}
			if got := response.Header().Get("Content-Type"); !strings.Contains(got, test.contentType) {
				t.Fatalf("Content-Type = %q, want it to contain %q", got, test.contentType)
			}
			for _, want := range test.contains {
				if !strings.Contains(response.Body.String(), want) {
					t.Fatalf("%s does not contain %q: %s", test.path, want, response.Body.String())
				}
			}
		})
	}
}

func TestSitemapContainsOnlyCanonicalHomepage(t *testing.T) {
	content, err := os.ReadFile("../../static/sitemap.xml")
	if err != nil {
		t.Fatalf("read sitemap: %v", err)
	}

	var sitemap struct {
		URLs []struct {
			Location     string `xml:"loc"`
			LastModified string `xml:"lastmod"`
		} `xml:"url"`
	}
	if err := xml.Unmarshal(content, &sitemap); err != nil {
		t.Fatalf("parse sitemap XML: %v", err)
	}
	if len(sitemap.URLs) != 1 {
		t.Fatalf("sitemap has %d URLs, want 1", len(sitemap.URLs))
	}
	if got := sitemap.URLs[0].Location; got != "https://getyauli.com/" {
		t.Fatalf("sitemap URL = %q, want canonical homepage", got)
	}
	if got := sitemap.URLs[0].LastModified; got != "2026-08-02" {
		t.Fatalf("sitemap lastmod = %q, want 2026-08-02", got)
	}
}

func TestTemplatesSetSearchIndexingPolicy(t *testing.T) {
	intro, err := os.ReadFile("../../templates/intro.html")
	if err != nil {
		t.Fatalf("read intro template: %v", err)
	}
	for _, want := range []string{
		`<title>Yauli Baby Tracker &mdash; Sleep, Feeding &amp; Nappy Log for Families</title>`,
		`<meta name="description" content="Track sleep, feeds, nappies, pumping, and growth in one shared family timeline, with daily summaries and 7&ndash;90 day insights. Free to start.">`,
		`<meta name="robots" content="index, follow">`,
		`<link rel="canonical" href="https://getyauli.com/">`,
		`<meta property="og:title"`,
		`"sameAs": ["https://instagram.com/yauli_parenting"]`,
		`@yauli_parenting`,
		`"Daily feed, sleep, pump, and nappy summaries"`,
		`"Sleep, feed, nappy, and growth insights"`,
	} {
		if !strings.Contains(string(intro), want) {
			t.Fatalf("intro template does not contain %q", want)
		}
	}
	for _, unwanted := range []string{`weekly milestone reports. Free to start.`, `href="#"`} {
		if strings.Contains(string(intro), unwanted) {
			t.Fatalf("intro template contains unavailable or placeholder content %q", unwanted)
		}
	}

	for _, name := range []string{"auth-verify.html", "index.html", "login.html", "onboarding.html", "settings.html"} {
		content, err := os.ReadFile(filepath.Join("../../templates", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		if !strings.Contains(string(content), `<meta name="robots" content="noindex, nofollow">`) {
			t.Fatalf("%s does not opt out of search indexing", name)
		}
	}
}

func TestDailyReportRendersFourKPIs(t *testing.T) {
	templates := parseFrontendTemplates(t)
	report := backendclient.DailyReport{
		Title: "Yau Yau today",
		Card: &backendclient.DailyReportCard{
			Metrics: []backendclient.DailyReportMetric{
				{Key: "feed", Count: 3, Label: "Feeds", Detail: "1h 27m (breast) · 530 ml (bottle)"},
				{Key: "sleep", Count: 3, Label: "Sleep", Detail: "5h 57m"},
				{Key: "pump", Count: 1, Label: "Pump", Detail: "150 ml · 1h"},
				{Key: "nappy", Count: 4, Label: "Nappies"},
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "daily-report", report); err != nil {
		t.Fatalf("render daily report: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`Yau Yau today`,
		`daily-report-metric-feed`,
		`daily-report-metric-count">3</strong>`,
		`daily-report-metric-label">Feeds</span>`,
		`daily-report-metric-detail">1h 27m (breast)</span>`,
		`daily-report-metric-detail">530 ml (bottle)</span>`,
		`daily-report-metric-sleep`,
		`daily-report-metric-detail">5h 57m</span>`,
		`daily-report-metric-pump`,
		`daily-report-metric-detail">150 ml</span>`,
		`daily-report-metric-detail">1h</span>`,
		`daily-report-metric-nappy`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("daily report HTML does not contain %q: %s", want, html)
		}
	}
	for _, unwanted := range []string{"hx-get=", "<p>", "<svg", "changed"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("daily report contains %q: %s", unwanted, html)
		}
	}
	if got := strings.Count(html, `class="daily-report-metric `); got != 4 {
		t.Fatalf("daily report contains %d metrics, want 4: %s", got, html)
	}
	if got := strings.Count(html, `class="daily-report-metric-detail"`); got != 5 {
		t.Fatalf("daily report contains %d metric detail rows, want 5: %s", got, html)
	}
}

func TestDailyReportMetricWidthsFollowContent(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	body := cssRuleBody(t, string(data), ".daily-report-metric {")
	if !strings.Contains(body, "flex: 1 1 auto") {
		t.Fatalf("daily report metrics should size from their content and available width, got rule body %q", body)
	}
	if strings.Contains(body, "flex: 1 1 0%") {
		t.Fatalf("daily report metrics still use fixed equal widths: %q", body)
	}
}

func TestIndexOmitsDailyReportToggle(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":        backendclient.Baby{Name: "YauYau", Timezone: "Australia/Perth"},
		"Account":     map[string]string{"Label": "Parent", "Email": "parent@example.com"},
		"Timeline":    handlers.TimelineViewData{SelectedDate: "2026-07-18"},
		"DailyReport": (*backendclient.DailyReport)(nil),
		"NowDate":     "2026-07-18",
		"NowTime":     "09:30",
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "index", data); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := rendered.String()
	if !strings.Contains(html, `id="type-filter"`) {
		t.Fatalf("event type filter missing: %s", html)
	}
	if !strings.Contains(html, `<body data-baby-timezone="Australia/Perth" data-current-date="2026-07-18">`) {
		t.Fatalf("index does not expose the baby timezone to event forms: %s", html)
	}
	for _, want := range []string{`data-filter-type="observation" title="Observations"`, `type-filter-chip-label">Observations</span>`} {
		if !strings.Contains(html, want) {
			t.Fatalf("observation filter does not contain %q: %s", want, html)
		}
	}
	for _, asset := range []string{"style.css", "htmx.min.js", "number-stepper.js", "app.js"} {
		if !strings.Contains(html, `/static/`+asset+`?v=test`) {
			t.Fatalf("index does not contain fingerprinted %s URL: %s", asset, html)
		}
	}
	for _, unwanted := range []string{"show-daily-report", "/timeline/preferences/daily-report", "timeline-display-filter"} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("index contains removed daily report toggle marker %q: %s", unwanted, html)
		}
	}
}

func TestIndexRendersMedicationCreateAndEditFields(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":     backendclient.Baby{Timezone: "Australia/Perth"},
		"Account":  map[string]string{"Label": "Parent"},
		"Timeline": handlers.TimelineViewData{SelectedDate: "2026-07-18"},
		"NowDate":  "2026-07-18",
		"NowTime":  "09:30",
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "index", data); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`data-filter-type="medication" title="Medication"`,
		`data-type="medication" hx-post="/medications"`,
		`data-edit-type="medication" data-medication-event`,
		`data-medication-item-list`,
		`data-medication-add-item`,
		`data-medication-kind-fields="vaccine"`,
		`name="item_kind_0" value="other"`,
		`id="medication-item-template"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("medication UI does not contain %q: %s", want, html)
		}
	}
}

func TestHTMXHistoryConfigIsPresentOnPushURLPages(t *testing.T) {
	templates := parseFrontendTemplates(t)
	want := `<meta name="htmx-config" content='{"historyCacheSize":0,"refreshOnHistoryMiss":true}'>`

	tests := []struct {
		name         string
		templateName string
		data         map[string]any
	}{
		{
			name:         "timeline",
			templateName: "index",
			data: map[string]any{
				"Baby":     backendclient.Baby{Timezone: "Australia/Perth"},
				"Account":  map[string]string{"Label": "Parent"},
				"Timeline": handlers.TimelineViewData{},
			},
		},
		{
			name:         "insights",
			templateName: "insights",
			data: map[string]any{
				"Baby":     backendclient.Baby{Timezone: "Australia/Perth"},
				"Account":  map[string]string{"Label": "Parent"},
				"Insights": handlers.InsightsViewData{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rendered bytes.Buffer
			if err := templates.ExecuteTemplate(&rendered, test.templateName, test.data); err != nil {
				t.Fatalf("render %s: %v", test.templateName, err)
			}
			if !strings.Contains(rendered.String(), want) {
				t.Fatalf("%s does not contain shared htmx history config: %s", test.templateName, rendered.String())
			}
		})
	}
}

func TestInsightsPeriodCountLabelsDescribeStartDayOwnership(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Perth"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			ShowSupportingRow:     true,
			ShowNapNight:          true,
			AverageBasisLabel:     "Based on 1 recorded day",
			AverageCompletedLabel: "1.0",
			RecordsBeginLabel:     "Records begin Jul 3",
			SelectedDay: &handlers.InsightsSelectedDay{
				HasPeriods: true,
				Periods: []handlers.InsightsPeriodRow{
					{
						TimeRangeLabel:     "Previous day – 1:27 AM",
						DurationLabel:      "1h 27m",
						Boundary:           true,
						FullTimeRangeLabel: "Sat 11:00 PM – Sun 1:27 AM",
						FullDurationLabel:  "2h 27m",
					},
					{
						TimeRangeLabel:     "10:00 PM – Next day",
						DurationLabel:      "2h",
						Boundary:           true,
						FullTimeRangeLabel: "Sun 10:00 PM – Mon 2:00 AM",
						FullDurationLabel:  "4h",
					},
					{
						TimeRangeLabel: "2:00 PM – 3:00 PM",
						DurationLabel:  "1h",
					},
				},
				BoundaryNote: "Selected-day view: “Sleep periods started” and “Longest recorded sleep” use sleeps that began this day; longest uses the whole completed period. Totals, the chart bar, and main row durations use only the portion that fell on this day. Whole sleep shows the complete period across midnight.",
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Average recorded sleep per day *",
		"Based on 1 recorded day",
		"Records begin Jul 3",
		"Sleep periods started",
		"Avg. completed sleep periods per day *",
		"* Averages per day include only days with recorded data. Gap days shown in the chart are excluded.",
		"Previous day – 1:27 AM*",
		"Whole sleep:",
		"Sat 11:00 PM – Sun 1:27 AM",
		"2h 27m",
		"10:00 PM – Next day*",
		"Sun 10:00 PM – Mon 2:00 AM",
		"4h",
		"2:00 PM – 3:00 PM",
		"* Selected-day view: “Sleep periods started” and “Longest recorded sleep” use sleeps that began this day; longest uses the whole completed period. Totals, the chart bar, and main row durations use only the portion that fell on this day. Whole sleep shows the complete period across midnight.",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("insights does not contain %q: %s", want, html)
		}
	}
	note := "* Averages per day include only days with recorded data. Gap days shown in the chart are excluded."
	barIndex := strings.Index(html, `class="insights-napnight-bar"`)
	if barIndex == -1 || strings.Index(html, note) < barIndex {
		t.Fatalf("sleep average note should appear below the horizontal breakdown bar: %s", html)
	}
	for _, unwanted := range []string{"insights-period-boundary", "insights-period-tooltip", `role="tooltip"`, `tabindex="0"`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("calendar-boundary context should use a plain footnote, found %q: %s", unwanted, html)
		}
	}
	if strings.Contains(html, "Completed sleep periods") {
		t.Fatalf("insights still uses misleading completed-period wording: %s", html)
	}
	if strings.Contains(html, "2:00 PM – 3:00 PM*") {
		t.Fatalf("within-day sleep period should not have a footnote marker: %s", html)
	}
	if count := strings.Count(html, "Whole sleep:"); count != 2 {
		t.Fatalf("whole sleep detail count = %d, want only the two boundary periods: %s", count, html)
	}
}

func TestInsightsAverageLabelsUseSharedRecordedDayNote(t *testing.T) {
	templates := parseFrontendTemplates(t)

	for _, test := range []struct {
		name string
		view handlers.InsightsViewData
		want string
	}{
		{
			name: "feeds",
			view: handlers.InsightsViewData{Category: "feeds", ShowFeedSupportingRow: true, ShowFeedBreakdown: true},
			want: "Avg. feeds per day *",
		},
		{
			name: "nappies",
			view: handlers.InsightsViewData{Category: "nappies", ShowNappySupportingRow: true, ShowNappyBreakdown: true},
			want: "Avg. nappies per day *",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			var rendered bytes.Buffer
			data := map[string]any{
				"Baby":     backendclient.Baby{Timezone: "Australia/Adelaide"},
				"Account":  map[string]string{"Label": "Parent"},
				"Insights": test.view,
			}
			if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
				t.Fatalf("render insights: %v", err)
			}
			html := rendered.String()
			note := "* Averages per day include only days with recorded data. Gap days shown in the chart are excluded."
			for _, want := range []string{
				test.want,
				note,
			} {
				if !strings.Contains(html, want) {
					t.Fatalf("insights does not contain %q: %s", want, html)
				}
			}
			barIndex := strings.Index(html, `class="insights-napnight-bar"`)
			if barIndex == -1 || strings.Index(html, note) < barIndex {
				t.Fatalf("average note should appear below the horizontal breakdown bar: %s", html)
			}
		})
	}
}

func TestInsightsBoundaryFootnoteUsesSubtleStyling(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	footnoteBody := cssRuleBody(t, css, ".insights-periods-footnote")
	for _, want := range []string{"var(--color-text-muted)", "font-size: 0.78rem", "line-height: 1.45"} {
		if !strings.Contains(footnoteBody, want) {
			t.Fatalf("calendar-boundary footnote should be subtle; missing %q from %q", want, footnoteBody)
		}
	}
	wholePeriodBody := cssRuleBody(t, css, ".insights-period-whole")
	for _, want := range []string{"var(--color-text-muted)", "font-size: 0.7rem"} {
		if !strings.Contains(wholePeriodBody, want) {
			t.Fatalf("whole-period detail should remain secondary; missing %q from %q", want, wholePeriodBody)
		}
	}
	for _, unwanted := range []string{"background:", "border-left:", "box-shadow:"} {
		if strings.Contains(wholePeriodBody, unwanted) {
			t.Fatalf("whole-period detail should not visually promote boundary rows; found %q in %q", unwanted, wholePeriodBody)
		}
	}
	for _, unwanted := range []string{".insights-period-boundary", ".insights-period-tooltip", ".insights-period-row.has-info"} {
		if strings.Contains(css, unwanted) {
			t.Fatalf("style.css should not retain boundary hover styling %q", unwanted)
		}
	}
}

func TestInsightsChartLabelsDoNotResizeBars(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	body := cssRuleBody(t, string(data), ".insights-bar-label")
	for _, want := range []string{"min-height: 1.2em", "line-height: 1.2", "white-space: nowrap"} {
		if !strings.Contains(body, want) {
			t.Fatalf("insights bar labels should reserve one non-wrapping line; missing %q from rule body %q", want, body)
		}
	}

	for _, test := range []struct {
		selector string
		width    string
		indent   string
	}{
		{selector: ".insights-chart-30 .insights-bar-col", width: "20px", indent: "    "},
		{selector: ".insights-chart-90 .insights-bar-col", width: "7px", indent: "  "},
		{selector: ".insights-chart-90 .insights-bar-col", width: "8px", indent: "    "},
	} {
		want := test.selector + " {\n" + test.indent + "width: " + test.width
		if !strings.Contains(string(data), want) {
			t.Fatalf("%s should keep a fixed %s column width when its date label is visible", test.selector, test.width)
		}
	}
}

func TestThirtyDayInsightsChartsFitMobileViewport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	for _, want := range []string{
		"@media (max-width: 899px)",
		".insights-chart-30 {\n    gap: 0.125rem;\n    overflow-x: hidden",
		".insights-chart-30 .insights-bar-col {\n    flex: 1 1 0;\n    width: auto;\n    min-width: 0",
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("30-day mobile chart should fit without horizontal scrolling; missing %q", want)
		}
	}
}

func TestNinetyDayInsightsChartsStartWithLatestData(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:   "sleep",
			RangeDays:  90,
			HasAnyData: true,
			ChartClass: "insights-chart-90",
			ChartDays: []handlers.InsightsChartDay{{
				FullLabel: "Sunday, August 2",
				HasData:   true,
			}},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render 90-day insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`/static/insights-charts.js?v=test`,
		`data-insights-scroll-hint`,
		`&larr; Scroll for older days`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("90-day insights should contain %q: %s", want, html)
		}
	}

	thirtyDayInsights := data["Insights"].(handlers.InsightsViewData)
	thirtyDayInsights.RangeDays = 30
	thirtyDayInsights.ChartClass = "insights-chart-30"
	data["Insights"] = thirtyDayInsights
	var thirtyDayRendered bytes.Buffer
	if err := templates.ExecuteTemplate(&thirtyDayRendered, "insights", data); err != nil {
		t.Fatalf("render 30-day insights: %v", err)
	}
	if strings.Contains(thirtyDayRendered.String(), "data-insights-scroll-hint") {
		t.Fatalf("30-day insights should not show a horizontal-scroll hint: %s", thirtyDayRendered.String())
	}

	content, err := os.ReadFile("../../static/insights-charts.js")
	if err != nil {
		t.Fatalf("read insights-charts.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`.querySelectorAll(".insights-chart-90")`,
		`chart.scrollLeft = maximumScroll`,
		`hint.hidden = true`,
		`"htmx:afterSwap"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("insights-charts.js does not contain %q", want)
		}
	}
}

func TestNappyInsightsMarkerLegendUsesPreformattedCountLabels(t *testing.T) {
	templates := parseFrontendTemplates(t)
	tests := []struct {
		name         string
		blowoutLabel string
		largeLabel   string
	}{
		{name: "zero", blowoutLabel: "0 blowouts", largeLabel: "0 large poos"},
		{name: "one", blowoutLabel: "1 blowout", largeLabel: "1 large poo"},
		{name: "multiple", blowoutLabel: "2 blowouts", largeLabel: "3 large poos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := map[string]any{
				"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
				"Account": map[string]string{"Label": "Parent"},
				"Insights": handlers.InsightsViewData{
					Category:               "nappies",
					HasNappyData:           true,
					ShowNappySupportingRow: true,
					ShowNappyBreakdown:     true,
					NappyBlowoutLabel:      tt.blowoutLabel,
					NappyLargeLabel:        tt.largeLabel,
				},
			}

			var rendered bytes.Buffer
			if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
				t.Fatalf("render nappy insights: %v", err)
			}
			html := rendered.String()
			for _, want := range []string{tt.blowoutLabel, tt.largeLabel} {
				if !strings.Contains(html, want) {
					t.Fatalf("nappy marker legend should contain %q: %s", want, html)
				}
			}
		})
	}
}

func TestShortInsightsChartsExpandWithinReadableLimit(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	body := cssRuleBody(t, string(data), ".insights-chart-adaptive .insights-bar-col")
	for _, want := range []string{"flex: 1 0 0", "width: auto", "max-width: 32px"} {
		if !strings.Contains(body, want) {
			t.Fatalf("adaptive insights bars should fill available space without becoming oversized; missing %q from rule body %q", want, body)
		}
	}
	if !strings.Contains(string(data), ".insights-chart-adaptive .insights-bar-col {\n    max-width: 40px") {
		t.Fatal("adaptive insights bars should have a readable desktop width cap")
	}
}

func TestGrowthInsightsPointsHaveAccessibleTouchTargets(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:         "growth",
			HasGrowthData:    true,
			GrowthLinePoints: "300.0,80.0",
			GrowthChartPoints: []handlers.InsightsChartPoint{{
				FullLabel:   "Monday, July 20",
				ValueLabel:  "4.200 kg",
				CX:          "300.0",
				CY:          "80.0",
				LeftPercent: "50.00",
				TopPercent:  "50.00",
				Href:        "/insights?category=growth&point=event-id",
			}},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`class="insights-growth-point-hit"`,
		`aria-label="Monday, July 20: 4.200 kg"`,
		`style="left:50.00%;top:50.00%"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("growth insights point is missing %q: %s", want, html)
		}
	}

	css, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	body := cssRuleBody(t, string(css), ".insights-growth-point-hit")
	for _, want := range []string{"width: 44px", "height: 44px"} {
		if !strings.Contains(body, want) {
			t.Fatalf("growth point touch target is missing %q from rule body %q", want, body)
		}
	}
}

func TestOverviewInsightsRendersRecordedStatsCard(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                   "overview",
			OverviewRangeContextLabel:  "Recorded over the last 30 days",
			OverviewAgeLabel:           "6 weeks, 3 days old",
			OverviewBirthDateLabel:     "12 June 2026",
			OverviewSleepValueLabel:    "7h 45m",
			OverviewSleepNightLabel:    "62% recorded overnight",
			OverviewSleepWakeLabel:     "2h 10m average awake window",
			OverviewSleepHref:          "/insights?category=sleep",
			OverviewFeedValueLabel:     "6.2",
			OverviewFeedBreastLabel:    "36h 47m breast",
			OverviewFeedFormulaLabel:   "8.2 L formula",
			OverviewFeedExpressedLabel: "4.8 L expressed",
			OverviewFeedHref:           "/insights?category=feeds",
			OverviewNappyValueLabel:    "8.1",
			OverviewNappyGapLabel:      "1h 50m average spacing",
			OverviewNappyHref:          "/insights?category=nappies",
			OverviewGrowthValueLabel:   "5.4 kg",
			OverviewGrowthChangeLabel:  "+1.2 kg since birth",
			OverviewGrowthLengthLabel:  "58.3 cm length (+7.8 cm since birth)",
			OverviewGrowthHref:         "/insights?category=growth",
			OverviewPumpSessionsLabel:  "4 pumping sessions",
			OverviewPumpMlLabel:        "320 ml expressed",
			OverviewPumpHref:           "/insights?category=pump",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Recorded over the last 30 days",
		"6 weeks, 3 days old", "Born 12 June 2026",
		"7h 45m", "62% recorded overnight", "2h 10m average awake window",
		"6.2", "36h 47m breast", "8.2 L formula", "4.8 L expressed",
		"8.1", "1h 50m average spacing",
		// html/template escapes "+" to "&#43;" in text nodes (renders as "+"
		// in the browser) — check the digits/copy, not the literal glyph.
		"5.4 kg", "1.2 kg since birth",
		"58.3 cm length", "7.8 cm since birth",
		"4 pumping sessions", "320 ml expressed",
		"Milk expressed, not milk the baby drank.",
		`href="/insights?category=sleep"`,
		`href="/insights?category=feeds"`,
		`href="/insights?category=nappies"`,
		`href="/insights?category=growth"`,
		`href="/insights?category=pump"`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("overview insights is missing %q", want)
		}
	}
	if strings.Contains(html, "YauYau") {
		t.Fatal("overview insights must not hardcode one family's baby name")
	}
}

func TestOverviewInsightsOmitsAgeCardWhenBirthDateIsMissing(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                  "overview",
			OverviewRangeContextLabel: "Recorded over the last 30 days",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()

	if strings.Contains(html, `insights-age-card`) {
		t.Fatal("age card should not render without a recorded birth date")
	}
}

func TestOverviewInsightsRendersFutureBirthDateAsExpected(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                  "overview",
			OverviewBirthDateLabel:    "12 September 2026",
			OverviewRangeContextLabel: "Recorded over the last 30 days",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()

	if !strings.Contains(html, "Expected 12 September 2026") {
		t.Fatalf("future birth date should render as expected: %s", html)
	}
	if strings.Contains(html, "Born 12 September 2026") || strings.Contains(html, "0 days old") {
		t.Fatalf("future birth date should not render as already born: %s", html)
	}
}

func TestOverviewInsightsRendersChatGptDiscussButton(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category: "overview",
			// Includes "&" and a quote so the assertions below prove the
			// data-chatgpt-summary attribute goes through html/template's
			// context-aware attribute escaping rather than being interpolated
			// raw (which would let summary text break out of the attribute).
			OverviewChatGptSummary: `Sleep & feeds: "steady"`,
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()

	if !strings.Contains(html, `data-chatgpt-summary="Sleep &amp; feeds: &#34;steady&#34;"`) {
		t.Fatalf("chatgpt summary attribute not escaped as expected, got: %s", html)
	}
	if !strings.Contains(html, `data-chatgpt-discuss-trigger`) {
		t.Fatal("overview insights is missing the Discuss with ChatGPT trigger")
	}
	if !strings.Contains(html, `id="insights-chatgpt-dialog"`) {
		t.Fatal("overview insights is missing the ChatGPT confirm dialog")
	}
	if strings.Count(html, `id="insights-chatgpt-dialog"`) != 1 {
		t.Fatal("ChatGPT confirm dialog should render exactly once, outside the htmx-swapped workspace")
	}
}

func TestOverviewInsightsOmitsChatGptButtonWhenSummaryEmpty(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category: "overview",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()

	if strings.Contains(html, `data-chatgpt-discuss-trigger`) {
		t.Fatal("Discuss with ChatGPT trigger should not render without a summary")
	}
	// The dialog itself is static page chrome (see insights.html), so it
	// still renders even when there's nothing to discuss yet.
	if !strings.Contains(html, `id="insights-chatgpt-dialog"`) {
		t.Fatal("ChatGPT confirm dialog should still be present as page chrome")
	}
}

func TestOverviewInsightsFallsBackToEmptyStateCopy(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                  "overview",
			OverviewSleepValueLabel:   "—",
			OverviewSleepEmptyLabel:   "Not enough recorded sleep yet",
			OverviewFeedEmptyLabel:    "Not enough recorded feeds yet",
			OverviewNappyEmptyLabel:   "Not enough recorded changes yet",
			OverviewGrowthChangeLabel: "No recorded weight yet",
			OverviewPumpEmptyLabel:    "No pumping sessions recorded",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Not enough recorded sleep yet",
		"Not enough recorded feeds yet",
		"Not enough recorded changes yet",
		"No recorded weight yet",
		"No pumping sessions recorded",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("overview insights empty state is missing %q", want)
		}
	}
	if strings.Contains(html, "recorded overnight") {
		t.Fatalf("sleep support lines should not render when there is no sleep data")
	}
}

func TestOverviewInsightsRendersHealthCard(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                       "overview",
			OverviewHealthAvailable:        true,
			OverviewHealthVaxCountLabel:    "3 recorded",
			OverviewHealthVaxRecentLabel:   "Most recent: Vaccinations at 6 weeks",
			OverviewHealthVaxMetaLabel:     "Jul 24 · at 6 weeks",
			OverviewHealthMedCountLabel:    "2 recorded",
			OverviewHealthMedRecentLabel:   "Most recent: Paracetamol · 1.5 ml",
			OverviewHealthMedMetaLabel:     "Jul 24 · at 6 weeks",
			OverviewHealthOtherCountLabel:  "1 recorded",
			OverviewHealthOtherRecentLabel: "Most recent: Sunscreen",
			OverviewHealthOtherMetaLabel:   "Aug 1 · at 7 weeks",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Health &amp; medicine",
		"3 recorded",
		"Most recent: Vaccinations at 6 weeks",
		"Jul 24 · at 6 weeks",
		"2 recorded",
		"Most recent: Paracetamol · 1.5 ml",
		"1 recorded",
		"Most recent: Sunscreen",
		"Aug 1 · at 7 weeks",
		"View health history",
		"data-health-history-toggle",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("overview health card is missing %q", want)
		}
	}
	// The history panel is a client-side disclosure now (insights-health-
	// history.js), not a server-rendered open/closed state — it's always in
	// the markup, just hidden by default.
	if !strings.Contains(html, `data-health-history hidden`) {
		t.Fatal("history panel should render hidden by default, not be omitted from the markup")
	}
	if strings.Contains(html, "hx-get") {
		t.Fatal("health history toggle should not make a server round trip — the data is already on the page")
	}
}

func TestOverviewInsightsRendersHealthHistoryRows(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                "overview",
			OverviewHealthAvailable: true,
			OverviewHealthVaxHistory: []handlers.InsightsHealthHistoryRow{
				{NameLabel: "6-in-1 · Dose 1", HasDescription: true, DescriptionLabel: "Vaxelis", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
				{NameLabel: "Rotavirus · Dose 1", WhenLabel: "Jul 24, 2026 · 10:35 AM · at 6 weeks"},
			},
			OverviewHealthMedHistory: []handlers.InsightsHealthHistoryRow{
				{NameLabel: "Paracetamol · 1.5 ml", HasDescription: true, DescriptionLabel: "For fever", WhenLabel: "Jul 24, 2026 · 7:20 PM · at 6 weeks"},
			},
			OverviewHealthOtherHistory: []handlers.InsightsHealthHistoryRow{
				{NameLabel: "Sunscreen", HasDescription: true, DescriptionLabel: "SPF 50", WhenLabel: "Aug 1, 2026 · 9:00 AM · at 7 weeks"},
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Vaccination history",
		"6-in-1 · Dose 1", "Vaxelis",
		"Rotavirus · Dose 1",
		"Jul 24, 2026 · 10:35 AM · at 6 weeks",
		"Medicine history",
		"Paracetamol · 1.5 ml", "For fever",
		"Jul 24, 2026 · 7:20 PM · at 6 weeks",
		"Other history",
		"Sunscreen", "SPF 50",
		"Aug 1, 2026 · 9:00 AM · at 7 weeks",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("health history is missing %q", want)
		}
	}
}

func TestOverviewInsightsHealthHistoryShowsEmptyStates(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                "overview",
			OverviewHealthAvailable: true,
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{"No vaccinations recorded yet.", "No medicine recorded yet.", "Nothing else recorded yet."} {
		if !strings.Contains(html, want) {
			t.Fatalf("empty health history is missing %q", want)
		}
	}
}

func TestOverviewInsightsOmitsHealthHistoryToggleWhenUnavailable(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                      "overview",
			OverviewHealthVaxEmptyLabel:   "Temporarily unavailable",
			OverviewHealthMedEmptyLabel:   "Temporarily unavailable",
			OverviewHealthOtherEmptyLabel: "Temporarily unavailable",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	if strings.Contains(html, "data-health-history-toggle") {
		t.Fatal("history toggle should not render when the health source is unavailable")
	}
}

func TestOverviewInsightsHealthEmptyState(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:                       "overview",
			OverviewHealthVaxEmptyLabel:    "None recorded",
			OverviewHealthVaxShowEmptyNote: true,
			OverviewHealthMedEmptyLabel:    "None recorded",
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		"Vaccinations you log will be kept here.",
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("overview health empty state is missing %q", want)
		}
	}
	if strings.Count(html, "None recorded") != 2 {
		t.Fatalf("expected both vaccination and medicine blocks to show \"None recorded\", got: %s", html)
	}
}

func TestGrowthInsightsChartShowsAxisAndEndpointLabels(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category:         "growth",
			HasGrowthData:    true,
			GrowthLinePoints: "20.0,120.0 580.0,40.0",
			GrowthAxisGuides: []handlers.InsightsGrowthAxisGuide{
				{Label: "6.1 kg", Y: "20.0", TopPercent: "12.50"},
				{Label: "4.8 kg", Y: "140.0", TopPercent: "87.50"},
			},
			GrowthChartPoints: []handlers.InsightsChartPoint{
				{
					ValueLabel:   "5.0 kg",
					CX:           "20.0",
					CY:           "120.0",
					LeftPercent:  "3.33",
					TopPercent:   "75.00",
					CalloutClass: "first",
				},
				{
					ValueLabel:   "5.9 kg",
					CX:           "580.0",
					CY:           "40.0",
					LeftPercent:  "96.67",
					TopPercent:   "25.00",
					CalloutClass: "last",
				},
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "insights", data); err != nil {
		t.Fatalf("render insights: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`class="insights-growth-axis-line" x1="0" y1="20.0" x2="600" y2="20.0"`,
		`class="insights-growth-axis-label" style="top:12.50%">6.1 kg</div>`,
		`class="insights-growth-axis-label" style="top:87.50%">4.8 kg</div>`,
		`insights-growth-point-callout-first`,
		`insights-growth-point-callout-last`,
		`style="left:3.33%;top:75.00%">5.0 kg</div>`,
		`style="left:96.67%;top:25.00%">5.9 kg</div>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("growth chart is missing %q: %s", want, html)
		}
	}

	cssData, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(cssData)
	for _, test := range []struct {
		selector string
		wants    []string
	}{
		{
			selector: ".insights-growth-axis-line",
			wants:    []string{"stroke: var(--color-border)", "stroke-width: 1", "stroke-dasharray: 5 5"},
		},
		{
			selector: ".insights-growth-axis-label",
			wants:    []string{"color: var(--color-text-muted)", "font-family: var(--font-body)", "font-size: 10px", "font-weight: 400"},
		},
		{
			selector: ".insights-growth-point-callout",
			wants:    []string{"color: var(--color-event-weight)", "font-family: var(--font-body)", "font-size: 10px", "font-weight: 400"},
		},
		{
			selector: ".insights-growth-point-callout-first",
			wants:    []string{"text-align: left", "translate(8px"},
		},
		{
			selector: ".insights-growth-point-callout-last",
			wants:    []string{"text-align: right", "translate(calc(-100% - 8px)"},
		},
	} {
		body := cssRuleBody(t, css, test.selector)
		for _, want := range test.wants {
			if !strings.Contains(body, want) {
				t.Fatalf("%s rule is missing %q from %q", test.selector, want, body)
			}
		}
	}
	if body := cssRuleBody(t, css, ".insights-growth-labels"); !strings.Contains(body, "height: 2.4em") {
		t.Fatalf("growth labels should reserve two rows at every viewport width; rule body %q", body)
	}
	if body := cssRuleBody(t, css, ".insights-growth-labels span:nth-child(even)"); !strings.Contains(body, "transform: translate(-50%, 1.15em)") {
		t.Fatalf("growth labels should stagger alternate dates at every viewport width; rule body %q", body)
	}

	var noDataRendered bytes.Buffer
	noData := map[string]any{
		"Baby":    backendclient.Baby{Timezone: "Australia/Adelaide"},
		"Account": map[string]string{"Label": "Parent"},
		"Insights": handlers.InsightsViewData{
			Category: "growth",
		},
	}
	if err := templates.ExecuteTemplate(&noDataRendered, "insights", noData); err != nil {
		t.Fatalf("render no-data insights: %v", err)
	}
	for _, unwanted := range []string{"insights-growth-axis-line", "insights-growth-axis-label", "insights-growth-point-callout"} {
		if strings.Contains(noDataRendered.String(), unwanted) {
			t.Fatalf("no-data growth chart should omit %q: %s", unwanted, noDataRendered.String())
		}
	}
}

func TestTimelineSectionDoesNotPoll(t *testing.T) {
	templates := parseFrontendTemplates(t)

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline-section", handlers.TimelineViewData{}); err != nil {
		t.Fatalf("render timeline section: %v", err)
	}
	html := rendered.String()
	if strings.Contains(html, "hx-trigger") || strings.Contains(html, "/timeline/events") {
		t.Fatalf("timeline section still contains polling attributes: %s", html)
	}
}

func TestTimelineWorkspaceCarriesSelectedDate(t *testing.T) {
	templates := parseFrontendTemplates(t)

	data := map[string]any{
		"Timeline": handlers.TimelineViewData{SelectedDate: "2026-08-05", ServerNowUnixMilli: 1785893400000},
	}
	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline-workspace", data); err != nil {
		t.Fatalf("render timeline workspace: %v", err)
	}
	if !strings.Contains(rendered.String(), `data-selected-date="2026-08-05"`) {
		t.Fatalf("timeline workspace does not identify its selected date: %s", rendered.String())
	}
	if !strings.Contains(rendered.String(), `data-server-now-ms="1785893400000"`) {
		t.Fatalf("timeline workspace does not carry the server clock anchor: %s", rendered.String())
	}
}

func TestAppJSUsesTimelineEventStreamWithoutPolling(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`new EventSource("/timeline/events/stream")`,
		`timelineEvents.addEventListener("timeline_changed"`,
		`timelineEvents.addEventListener("navigate"`,
		`document.visibilityState === "hidden"`,
		`event.detail.successful === true`,
		`refreshRetryDelay`,
		`target: "#timeline-workspace"`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js does not contain %q", want)
		}
	}
	for _, unwanted := range []string{"setInterval", "FALLBACK_INTERVAL", "every 30s"} {
		if strings.Contains(js, unwanted) {
			t.Fatalf("app.js still contains polling marker %q", unwanted)
		}
	}
}

func TestAppJSUpdatesLiveTimelineDurationsFromAbsoluteStartTime(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`function updateLiveTimelineDurations()`,
		`document.querySelectorAll("[data-live-duration-start-ms]")`,
		`const serverNowMS = Number(workspace?.dataset.serverNowMs)`,
		`serverNowMS - Date.now()`,
		`const now = Date.now() + liveDurationServerOffsetMS`,
		`const elapsedMS = Math.max(0, now - startedAtMS)`,
		`Math.floor(elapsedMS / 60000)`,
		`window.setTimeout(updateLiveTimelineDurations, nextUpdateDelay)`,
		`updateLiveTimelineDurations();`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js does not contain live-duration behavior %q", want)
		}
	}
}

func TestAppJSRejectsStaleTimelineWorkspaceResponses(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`let desiredTimelineDate = timelineWorkspace?.dataset.selectedDate`,
		`responseDate !== desiredTimelineDate`,
		`event.detail.shouldSwap = false`,
		"return `/app?date=${encodeURIComponent(desiredTimelineDate)}`",
		`desiredTimelineDate = renderedDate`,
		`setActiveTimelineDay(renderedDate)`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js does not contain stale timeline response guard %q", want)
		}
	}
}

func TestAppJSReconcilesTimelineAcrossBabyTimezoneMidnight(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`let timelineCalendarDate = document.body.dataset.currentDate`,
		`let followsCurrentTimelineDay = desiredTimelineDate === timelineCalendarDate`,
		`function reconcileTimelineDateRollover()`,
		`currentCalendarDate <= timelineCalendarDate`,
		`document.visibilityState === "hidden" || timelineEditorOpen()`,
		`window.location.replace(destination)`,
		`window.addEventListener("pageshow"`,
		`scheduleTimelineDateRolloverCheck()`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js does not contain timeline rollover behavior %q", want)
		}
	}
	if strings.Contains(js, `let timelineCalendarDate = dateTimeValuesInBabyTimezone(new Date()).date`) {
		t.Fatal("timeline rollover still treats the device date as authoritative")
	}
}

func TestNappyTimelineDetailIcons(t *testing.T) {
	templates := parseFrontendTemplates(t)

	tests := []struct {
		name     string
		kind     string
		pooSize  string
		wantSVGs int
	}{
		{name: "wet", kind: "wet", wantSVGs: 1},
		{name: "poo with size", kind: "poo", pooSize: "medium", wantSVGs: 2},
		{name: "poo without size", kind: "poo", wantSVGs: 1},
		{name: "both", kind: "both", pooSize: "large", wantSVGs: 4},
		{name: "both without size", kind: "both", wantSVGs: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var rendered bytes.Buffer
			data := map[string]string{"Kind": test.kind, "PooSize": test.pooSize}
			if err := templates.ExecuteTemplate(&rendered, "nappy-timeline-detail-icons", data); err != nil {
				t.Fatalf("render nappy timeline detail icons: %v", err)
			}
			if got := strings.Count(rendered.String(), "<svg"); got != test.wantSVGs {
				t.Fatalf("rendered %d SVGs, want %d: %s", got, test.wantSVGs, rendered.String())
			}
		})
	}
}

func TestNappyTimelineRendersSpecificLabelAndKindIconOnlyInDetailRow(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := handlers.TimelineViewData{
		SelectedDate: "2026-07-15",
		Events: []handlers.TimelineEvent{
			{
				ID:        "event-1",
				EventType: "nappy",
				CSSClass:  "nappy",
				TypeLabel: "Wee",
				KindValue: "wet",
				Time:      "10:15 AM",
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline", data); err != nil {
		t.Fatalf("render timeline: %v", err)
	}
	html := rendered.String()
	header := elementMarkup(t, html, `<div class="event-card-header">`)
	if strings.Contains(header, "nappy-detail-icons") {
		t.Fatalf("event header contains nappy kind icons: %s", header)
	}
	if !strings.Contains(header, `<span class="event-type">Wee</span>`) {
		t.Fatalf("event header does not contain the specific nappy label: %s", header)
	}
	if strings.Contains(header, `class="event-kind"`) {
		t.Fatalf("event header contains redundant nappy kind text: %s", header)
	}

	detail := elementMarkup(t, html, `<div class="event-detail">`)
	if !strings.Contains(detail, `class="nappy-detail-icons"`) {
		t.Fatalf("event detail does not contain nappy kind icons: %s", detail)
	}
	if got := strings.Count(detail, "<svg"); got != 1 {
		t.Fatalf("event detail contains %d SVGs, want one wet icon: %s", got, detail)
	}
}

func TestTimelineEventCardOpensEditorWithoutActionIcons(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := handlers.TimelineViewData{
		SelectedDate: "2026-07-15",
		Events: []handlers.TimelineEvent{
			{
				ID:        "event-1",
				EventType: "feed",
				CSSClass:  "feed",
				TypeLabel: "Bottle",
				Time:      "10:15 AM",
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline", data); err != nil {
		t.Fatalf("render timeline: %v", err)
	}
	html := rendered.String()
	for _, marker := range []string{
		`class="event-card-open" role="button" tabindex="0" aria-label="Edit Bottle event at 10:15 AM"`,
		`data-event-id="event-1"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("timeline event card missing %q: %s", marker, html)
		}
	}
	for _, unwanted := range []string{`class="event-edit"`, `class="event-delete"`, `hx-confirm`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("timeline event card contains removed action %q: %s", unwanted, html)
		}
	}
}

func TestTimelineRendersLiveAndCompletedDurationHierarchy(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := handlers.TimelineViewData{
		SelectedDate: "2026-07-15",
		Events: []handlers.TimelineEvent{
			{
				ID: "sleep-live", EventType: "sleep", CSSClass: "sleep", TypeLabel: "Nap", Time: "1:53 PM",
				DurationLabel: "28m", LiveStatusLabel: "sleeping now", LiveDurationStartMS: "1784092980000",
				Ongoing: true, CanFinishSleep: true,
			},
			{
				ID: "sleep-finished", EventType: "sleep", CSSClass: "sleep", TypeLabel: "Nap", Time: "1:28 PM",
				DurationLabel: "14m", FinishedTime: "1:42 PM", FinishedTimeValue: "2026-07-15T13:42:00+09:30",
			},
		},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline", data); err != nil {
		t.Fatalf("render timeline: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`class="event-duration-row ongoing"`,
		`data-live-duration-start-ms="1784092980000" aria-live="off">28m`,
		`class="event-duration-live"`,
		`sleeping now`,
		`class="event-duration-value">14m`,
		`class="event-finished-time"`,
		`<time datetime="2026-07-15T13:42:00&#43;09:30">1:42 PM</time>`,
		`Finish now`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("timeline duration markup missing %q: %s", want, html)
		}
	}
	if strings.Contains(html, `class="event-status-pill"`) {
		t.Fatalf("timeline still renders the noisy ongoing pill: %s", html)
	}
}

func TestCompletedTimelineTimingStaysVisuallySecondary(t *testing.T) {
	data, err := os.ReadFile("../../static/style.css")
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	if body := cssRuleBody(t, css, ".event-duration-value"); !strings.Contains(body, "font-weight: 500") {
		t.Fatalf("completed duration should use medium emphasis: %q", body)
	}
	if body := cssRuleBody(t, css, ".event-duration-row.ongoing .event-duration-value"); !strings.Contains(body, "font-weight: 800") {
		t.Fatalf("ongoing duration should remain prominent: %q", body)
	}
	if body := cssRuleBody(t, css, ".event-finished-time"); !strings.Contains(body, "font-weight: 400") {
		t.Fatalf("finished time should use regular emphasis: %q", body)
	}
}

func TestTimelineMedicationRendersOneStandardEventWithAllItems(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := handlers.TimelineViewData{
		SelectedDate: "2026-08-29",
		Events: []handlers.TimelineEvent{{
			ID: "medication-1", EventType: "medication", CSSClass: "medication", TypeLabel: "Medication",
			Time: "10:30 AM", DateValue: "2026-08-29", TimeValue: "10:30", InlineDetail: "3 items",
			MedicationItemsJSON: `[{"kind":"vaccine","name":"Rotavirus"},{"kind":"vaccine","name":"Pneumococcal"},{"kind":"medicine","name":"Infant paracetamol","dose_value":2.5,"dose_unit":"ml"}]`,
			MedicationItemRows:  []string{"Vaccine · Rotavirus", "Vaccine · Pneumococcal", "Medicine · Infant paracetamol · 2.5 mL"},
		}},
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "timeline", data); err != nil {
		t.Fatalf("render timeline: %v", err)
	}
	html := rendered.String()
	for _, want := range []string{
		`class="event-card event-medication"`,
		`class="event-card-open" role="button" tabindex="0" aria-label="Edit Medication event at 10:30 AM"`,
		`data-event-id="medication-1"`,
		`data-medication-items=`,
		`3 items`,
		`Rotavirus`,
		`Pneumococcal`,
		`Infant paracetamol`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("medication event missing %q: %s", want, html)
		}
	}
	if got := strings.Count(html, `<article class="event-card`); got != 1 {
		t.Fatalf("timeline rendered %d event cards, want one medication event: %s", got, html)
	}
	if got := strings.Count(html, `class="medication-timeline-item"`); got != 3 {
		t.Fatalf("timeline rendered %d medication item rows, want 3: %s", got, html)
	}
	for _, want := range []string{
		`<span class="medication-timeline-item">Vaccine · Rotavirus</span>`,
		`<span class="medication-timeline-item">Vaccine · Pneumococcal</span>`,
		`<span class="medication-timeline-item">Medicine · Infant paracetamol · 2.5 mL</span>`,
	} {
		if !strings.Contains(html, want) {
			t.Fatalf("timeline medication rows missing %q: %s", want, html)
		}
	}
}

func TestAppJSUsesBabyTimezoneForEventDefaults(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	js := string(content)
	for _, want := range []string{
		`const babyTimezone = document.body.dataset.babyTimezone`,
		`timeZone: babyTimezone`,
		`dateTimeValuesInBabyTimezone(new Date())`,
		`dateInput.value = now.date`,
		`timeInput.value = now.time`,
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("app.js does not contain %q", want)
		}
	}
	for _, unwanted := range []string{
		`dateInput.value = localDateValue(now)`,
		`timeInput.value = localTimeValue(now)`,
		`editDateInput.max = localDateValue(new Date())`,
	} {
		if strings.Contains(js, unwanted) {
			t.Fatalf("app.js still uses browser-local event defaults %q", unwanted)
		}
	}
}

func TestAppJSRestoresMedicationDescriptionForEditing(t *testing.T) {
	content, err := os.ReadFile("../../static/app.js")
	if err != nil {
		t.Fatalf("read app.js: %v", err)
	}
	if !strings.Contains(string(content), "setFieldValue(item, `item_description_${index}`, values.description)") {
		t.Fatal("app.js does not restore medication descriptions")
	}
}

func TestIndexEditDialogHasImmediateDeleteAndDisabledSave(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":        backendclient.Baby{Name: "YauYau"},
		"Account":     map[string]string{"Label": "Parent", "Email": "parent@example.com"},
		"Timeline":    handlers.TimelineViewData{SelectedDate: "2026-07-18"},
		"DailyReport": (*backendclient.DailyReport)(nil),
		"NowDate":     "2026-07-18",
		"NowTime":     "09:30",
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "index", data); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := rendered.String()
	for _, marker := range []string{
		`class="edit-event-actions"`,
		`id="edit-event-delete"`,
		`hx-delete="/events/__event_id__"`,
		`id="edit-event-save" disabled`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("edit dialog missing %q: %s", marker, html)
		}
	}
	if saveIndex, deleteIndex := strings.Index(html, `id="edit-event-save"`), strings.Index(html, `id="edit-event-delete"`); saveIndex == -1 || deleteIndex == -1 || saveIndex > deleteIndex {
		t.Fatalf("edit dialog actions are not ordered Save then Delete: %s", html)
	}
	for _, unwanted := range []string{`hx-confirm`, `id="confirm-dialog"`} {
		if strings.Contains(html, unwanted) {
			t.Fatalf("edit dialog contains removed confirmation UI %q: %s", unwanted, html)
		}
	}
}

func TestIndexGroupsEventDateAndTimeFields(t *testing.T) {
	templates := parseFrontendTemplates(t)
	data := map[string]any{
		"Baby":        backendclient.Baby{Name: "YauYau"},
		"Account":     map[string]string{"Label": "Parent", "Email": "parent@example.com"},
		"Timeline":    handlers.TimelineViewData{SelectedDate: "2026-07-18"},
		"DailyReport": (*backendclient.DailyReport)(nil),
		"NowDate":     "2026-07-18",
		"NowTime":     "09:30",
	}

	var rendered bytes.Buffer
	if err := templates.ExecuteTemplate(&rendered, "index", data); err != nil {
		t.Fatalf("render index: %v", err)
	}
	html := rendered.String()

	for _, eventType := range []string{"nappy", "bath", "observation", "temperature", "medication", "growth_measurement"} {
		form := createEventFormMarkup(t, html, eventType)
		if got := strings.Count(form, `class="event-occurred-at-fields"`); got != 1 {
			t.Errorf("%s create form has %d Time/Date groups, want 1", eventType, got)
		}
		if !strings.Contains(form, `type="time" name="time"`) || !strings.Contains(form, `type="date" name="date"`) {
			t.Errorf("%s create form does not contain Time and Date inputs in its shared group", eventType)
		}
	}

	for _, eventType := range []string{"feed", "pump"} {
		form := createEventFormMarkup(t, html, eventType)
		if !strings.Contains(form, `Started`) || strings.Count(form, `class="sleep-time-pair"`) != 2 {
			t.Errorf("%s create form does not group Started and Finished Date/Time fields", eventType)
		}
	}

	if got := strings.Count(html, `class="edit-occurred-at-fields"`); got != 1 {
		t.Errorf("edit dialog has %d shared Time/Date groups, want 1", got)
	}
}

// TestEventOccurredAtFieldsShowDateBeforeTime guards the date-before-time
// convention for the simple event types (nappy, bath, observation,
// temperature, medication, growth_measurement, and the edit dialog's non-grouped case).
// The markup itself still puts the Time field first (unchanged, so nothing
// that depends on that markup order breaks) — only the *visual* order is
// flipped via CSS `order`, the same technique .grouped-edit-time-fields
// already relied on for Started/Finished. Because that reorder lives
// entirely in CSS, no template-rendering test can see it; this test instead
// reads style.css directly so removing the `order` rules fails a test
// instead of silently reintroducing the Time/Date inconsistency.
func TestEventOccurredAtFieldsShowDateBeforeTime(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	firstOfType := cssRuleBody(t, css, ".event-occurred-at-fields .field-label:first-of-type")
	if !strings.Contains(firstOfType, "order: 2") {
		t.Errorf("the Time field (first in markup) should be ordered after Date (order: 2), got rule body %q", firstOfType)
	}

	lastOfType := cssRuleBody(t, css, ".event-occurred-at-fields .field-label:last-of-type")
	if !strings.Contains(lastOfType, "order: 1") {
		t.Errorf("the Date field (last in markup) should be ordered first (order: 1), got rule body %q", lastOfType)
	}

	// .edit-occurred-at-fields shares the same selector list as
	// .event-occurred-at-fields for both rules above (comma-separated), so
	// finding it at all confirms the edit dialog's non-grouped case is
	// covered by the same reorder.
	if !strings.Contains(css, ".edit-occurred-at-fields .field-label:first-of-type") {
		t.Error("edit dialog's shared Time/Date fields are not included in the date-before-time reorder")
	}
}

func TestEventDialogsFitDynamicMobileViewport(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}

	body := cssRuleBody(t, string(data), "dialog#edit-event-dialog")
	for _, want := range []string{"max-height: 85vh", "max-height: 85dvh", "overflow-y: auto"} {
		if !strings.Contains(body, want) {
			t.Errorf("event dialogs should fit and scroll within the visible mobile viewport; rule does not contain %q: %q", want, body)
		}
	}
}

func TestIntroLandingHandlesNarrowAndDarkScreens(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "static", "style.css"))
	if err != nil {
		t.Fatalf("read style.css: %v", err)
	}
	css := string(data)

	tests := []struct {
		selector string
		want     []string
	}{
		{selector: ".intro-main", want: []string{"max-width: none"}},
		{selector: ".intro-hero-copy", want: []string{"min-width: min(300px, 100%)"}},
		{selector: ".intro-hero-visual", want: []string{"min-width: min(280px, 100%)"}},
		{selector: ".intro-phone", want: []string{"box-sizing: border-box", "width: min(280px, 100%)"}},
		{selector: ".intro-phone-kpi .intro-phone-kpi-detail", want: []string{"color: #5C6B7A"}},
		{selector: ".intro-phone-events", want: []string{"text-align: left"}},
		{selector: ".intro-insight-intro", want: []string{"max-width: 640px", "margin: 0 auto"}},
		{selector: ".intro-insight-intro p", want: []string{"color: var(--color-text-secondary)"}},
		{selector: ".intro-insight-card", want: []string{"box-sizing: border-box", "max-width: 560px", "margin: 0 auto"}},
		{selector: ".intro-cta-banner", want: []string{"box-sizing: border-box", "width: calc(100% - 3rem)", "max-width: 1132px", "margin: 0 auto 6rem"}},
	}

	for _, test := range tests {
		t.Run(test.selector, func(t *testing.T) {
			body := cssRuleBody(t, css, test.selector)
			for _, want := range test.want {
				if !strings.Contains(body, want) {
					t.Fatalf("%s rule does not contain %q:\n%s", test.selector, want, body)
				}
			}
		})
	}
	for _, selector := range []string{".intro-hero-section", ".intro-insight-section-tint"} {
		if body := cssRuleBody(t, css, selector); strings.Contains(body, "50vw") {
			t.Fatalf("%s still uses a viewport-width full-bleed margin that overflows the stable scrollbar gutter", selector)
		}
	}
	if !strings.Contains(css, "@media (prefers-reduced-motion: reduce)") ||
		!strings.Contains(css, ".intro-secondary-arrow {\n    animation: none;") {
		t.Fatal("intro landing does not disable its decorative arrow animation for reduced-motion users")
	}
}

// cssRuleBody returns the `{ ... }` body of the first CSS rule whose
// selector list contains selector, ignoring how the rest of the selector
// list is formatted (e.g. a comma-separated sibling selector on its own
// line). Fails the test if selector or its rule body cannot be found.
func cssRuleBody(t *testing.T, css, selector string) string {
	t.Helper()
	start := strings.Index(css, selector)
	if start == -1 {
		t.Fatalf("style.css does not contain selector %q", selector)
	}
	openBrace := strings.Index(css[start:], "{")
	if openBrace == -1 {
		t.Fatalf("selector %q has no rule body", selector)
	}
	bodyStart := start + openBrace + 1
	closeBrace := strings.Index(css[bodyStart:], "}")
	if closeBrace == -1 {
		t.Fatalf("rule body for selector %q is not closed", selector)
	}
	return css[bodyStart : bodyStart+closeBrace]
}

func parseFrontendTemplates(t *testing.T) *template.Template {
	t.Helper()
	templates, err := template.New("").Funcs(template.FuncMap{
		"assetURL":          func(name string) string { return "/static/" + name + "?v=test" },
		"dict":              dict,
		"splitTime":         splitEventTime,
		"splitMetricDetail": splitMetricDetail,
		"initial":           initial,
	}).ParseGlob("../../templates/*.html")
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	return templates
}

func elementMarkup(t *testing.T, html, openingTag string) string {
	t.Helper()
	start := strings.Index(html, openingTag)
	if start == -1 {
		t.Fatalf("rendered HTML does not contain %q", openingTag)
	}
	relativeEnd := strings.Index(html[start:], "</div>")
	if relativeEnd == -1 {
		t.Fatalf("rendered HTML element %q is not closed", openingTag)
	}
	return html[start : start+relativeEnd+len("</div>")]
}

func createEventFormMarkup(t *testing.T, html, eventType string) string {
	t.Helper()
	openingTag := `<form class="event-form" data-type="` + eventType + `"`
	start := strings.Index(html, openingTag)
	if start == -1 {
		t.Fatalf("rendered HTML does not contain %s create form", eventType)
	}
	relativeEnd := strings.Index(html[start:], "</form>")
	if relativeEnd == -1 {
		t.Fatalf("rendered %s create form is not closed", eventType)
	}
	return html[start : start+relativeEnd+len("</form>")]
}

package reportemail

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"math"
	"strings"
)

//go:embed templates/report_email.html
var reportEmailTemplateFS embed.FS

// reportEmailTmpl is parsed once at package init — compiled into the binary
// via go:embed rather than loaded from disk, since backend-api's Dockerfile
// (unlike frontend's) doesn't ship a templates/ directory alongside the
// binary.
var reportEmailTmpl = template.Must(template.New("").ParseFS(reportEmailTemplateFS, "templates/*.html"))

// reportEmailView is everything templates/report_email.html needs, fully
// resolved: no arithmetic, scaling, or conditionals left for the template to
// perform. Every field here is either a plain value or an already-decided
// list; html/template auto-escapes every {{.Field}} for its HTML context, so
// callers no longer need the old manual htmlEscape.
type reportEmailView struct {
	Subject        string
	DateHeading    string
	Subtitle       string
	Card           []cardColumnView
	Summary        string
	Highlights     []listItemView
	Patterns       []listItemView
	Comparison     []listItemView
	Caveats        []listItemView
	Trend          []trendChartView
	Encouragement  string
	ReportWindow   string
	RecipientEmail string
}

// cardColumnView is one KPI card column, fully resolved (column width,
// padding, color, and detail lines already computed).
//
// PaddingStyle is template.CSS, not string: it's a raw "property:value;"
// fragment (e.g. "padding-right:10px;") spliced into a style="..."
// attribute alongside static text, which html/template's contextual
// auto-escaper can't safely classify as plain string data and blanks to
// "ZgotmplZ" otherwise. It's always one of a small set of hardcoded
// literals from buildCardView below, never user input, so trusting it is
// safe.
type cardColumnView struct {
	WidthPercent int
	PaddingStyle template.CSS
	Color        string
	Count        int
	Label        string
	Details      []string
	ShowDivider  bool
}

// listItemView is one bullet in a Highlights/Patterns/Comparison/Caveats
// list, with its trailing padding already decided (last item gets none).
type listItemView struct {
	Text          string
	PaddingBottom string
}

// trendChartView is one "Last 7 days" chart (Sleep, Feeds, Nappies, or
// Pump), fully resolved: every row's bar segment widths, colors, and corner
// rounding are already computed.
type trendChartView struct {
	Heading       string
	HeadingColor  string
	Legend        []trendLegendEntry
	ValueFontSize string
	Rows          []trendRowView
}

// trendLegendEntry is one color-swatch legend chip shown beside a chart's
// heading, e.g. the amber square labeled "Count" on the Feeds chart.
type trendLegendEntry struct {
	Color string
	Label string
}

// trendRowView is one day's bar (or one series' bar within a stacked day
// group, for Feeds/Pump).
//
// LabelPadding and ValuePadding are template.CSS, not string, for the same
// reason as cardColumnView.PaddingStyle above: they're raw "property:value;"
// fragments (or "") spliced into a style="..." attribute, which
// html/template's auto-escaper otherwise blanks to "ZgotmplZ". Both are
// always one of a small set of hardcoded literals from buildTrendChartView
// below, never user input.
type trendRowView struct {
	Label        string
	LabelPadding template.CSS
	Segments     []barSegmentView
	Value        string
	ValuePadding template.CSS
}

// barSegmentView is one colored piece of a bar: the filled portion, the
// muted track behind it, or both. Bar and track render as one seamless
// pill — whichever segment is present alone gets fully rounded corners, and
// when both are present the fill only rounds its left edge while the track
// only rounds its right edge, so there's no visible seam between colors.
type barSegmentView struct {
	WidthPx      int
	HeightPx     int
	Color        string
	BorderRadius string
}

// buildReportEmailView turns a Report into the fully-resolved view the
// template renders. All formatting decisions (padding, scaling, color
// selection, escaping) happen here, in Go, not in the template.
func buildReportEmailView(report Report) reportEmailView {
	reportWindow := report.StartDate
	if report.EndDate != "" && report.EndDate != report.StartDate {
		reportWindow += " to " + report.EndDate
	}

	return reportEmailView{
		Subject:        subject(report),
		DateHeading:    reportDateHeading(report),
		Subtitle:       reportSubtitle(report),
		Card:           buildCardView(report.Card),
		Summary:        report.Output.Summary,
		Highlights:     buildListView(report.Output.Highlights),
		Patterns:       buildListView(report.Output.Patterns),
		Comparison:     buildListView(report.Output.Comparison),
		Caveats:        buildListView(report.Output.Caveats),
		Trend:          buildTrendChartViews(report.Trend),
		Encouragement:  reportEncouragement,
		ReportWindow:   reportWindow,
		RecipientEmail: report.RecipientEmail,
	}
}

// buildCardView mirrors the KPI card's former writeHTMLCard logic: an even
// column width per metric, tighter padding on the first/last columns, cycling
// label colors, and a divider between (not after) columns.
func buildCardView(cards []CardMetric) []cardColumnView {
	if len(cards) == 0 {
		return nil
	}

	columnWidth := 100 / len(cards)
	views := make([]cardColumnView, 0, len(cards))
	for i, metric := range cards {
		color := cardMetricColors[i%len(cardMetricColors)]
		padding := "padding-left:10px; padding-right:10px;"
		if i == 0 {
			padding = "padding-right:10px;"
		} else if i == len(cards)-1 {
			padding = "padding-left:10px;"
		}

		var details []string
		for _, detail := range strings.Split(metric.Detail, " · ") {
			if detail != "" {
				details = append(details, detail)
			}
		}

		views = append(views, cardColumnView{
			WidthPercent: columnWidth,
			PaddingStyle: template.CSS(padding),
			Color:        color,
			Count:        metric.Count,
			Label:        metric.Label,
			Details:      details,
			ShowDivider:  i < len(cards)-1,
		})
	}
	return views
}

// buildListView mirrors the former writeHTMLList logic: trim and drop empty
// items, then mark the last surviving item's padding as none instead of the
// usual 6px gap. Returns nil (falsy in the template) when nothing survives,
// matching the old early-return-and-omit-the-section behavior.
func buildListView(items []string) []listItemView {
	nonEmpty := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			nonEmpty = append(nonEmpty, item)
		}
	}
	if len(nonEmpty) == 0 {
		return nil
	}

	views := make([]listItemView, 0, len(nonEmpty))
	for i, item := range nonEmpty {
		paddingBottom := "6px"
		if i == len(nonEmpty)-1 {
			paddingBottom = "0"
		}
		views = append(views, listItemView{Text: item, PaddingBottom: paddingBottom})
	}
	return views
}

// trendBarTrackWidth, trendBarTrackColor are the shared proportions every
// "Last 7 days" bar scales against, matching the Reports page design (the
// 30px day-label and 44px value columns are constant across every chart, so
// they're hardcoded directly in the template rather than threaded through
// here).
const (
	trendBarTrackWidth = 434
	trendBarTrackColor = "#EDE2D6"
)

// trendSeriesBuilder is one bar series within a "Last 7 days" chart —
// Go-side only, never touches the template. Single-bar charts (Sleep,
// Nappies) have exactly one series; stacked charts (Feeds, Pump) have
// several, each on its own row per day.
type trendSeriesBuilder struct {
	Color  string
	Value  func(TrendDay) float64
	Format func(float64) string
}

// trendChartBuilder describes one of the "Last 7 days" charts before its
// data is resolved into a trendChartView.
type trendChartBuilder struct {
	Heading       string
	HeadingColor  string
	Legend        []trendLegendEntry
	BarHeight     int
	ValueFontSize string
	Series        []trendSeriesBuilder
}

// buildTrendChartViews builds the four "Last 7 days" charts — Sleep, Feeds
// (Count/Duration/Bottle mL stacked), Nappies, then Pump (mL/Duration
// stacked) — matching the Reports page design. Each series scales
// independently against its own weekly max. Returns nil when trend is empty,
// so the template omits the section entirely.
func buildTrendChartViews(trend []TrendDay) []trendChartView {
	if len(trend) == 0 {
		return nil
	}

	builders := []trendChartBuilder{
		{
			Heading:       "Sleep",
			HeadingColor:  "#6E4E96",
			Legend:        []trendLegendEntry{{Color: "#B99BD1", Label: "Hours"}},
			BarHeight:     12,
			ValueFontSize: "11.5px",
			Series: []trendSeriesBuilder{
				{
					Color:  "#B99BD1",
					Value:  func(d TrendDay) float64 { return d.SleepHours },
					Format: func(v float64) string { return fmt.Sprintf("%.1fh", v) },
				},
			},
		},
		{
			Heading:      "Feeds",
			HeadingColor: "#8F5A2B",
			Legend: []trendLegendEntry{
				{Color: "#E8A87C", Label: "Count"},
				{Color: "#F0C7A3", Label: "Duration"},
				{Color: "#B5652F", Label: "Bottle mL"},
			},
			BarHeight:     9,
			ValueFontSize: "11px",
			Series: []trendSeriesBuilder{
				{
					Color:  "#E8A87C",
					Value:  func(d TrendDay) float64 { return float64(d.FeedCount) },
					Format: func(v float64) string { return fmt.Sprintf("%d", int(v)) },
				},
				{
					Color:  "#F0C7A3",
					Value:  func(d TrendDay) float64 { return float64(d.FeedDurationMinutes) },
					Format: func(v float64) string { return formatTrendDurationMinutes(int(v)) },
				},
				{
					Color:  "#B5652F",
					Value:  func(d TrendDay) float64 { return float64(d.FeedBottleMl) },
					Format: func(v float64) string { return fmt.Sprintf("%d mL", int(v)) },
				},
			},
		},
		{
			Heading:       "Nappies",
			HeadingColor:  "#9C7A4E",
			Legend:        []trendLegendEntry{{Color: "#9C7A4E", Label: "Changed"}},
			BarHeight:     12,
			ValueFontSize: "11.5px",
			Series: []trendSeriesBuilder{
				{
					Color:  "#9C7A4E",
					Value:  func(d TrendDay) float64 { return float64(d.NappyCount) },
					Format: func(v float64) string { return fmt.Sprintf("%d", int(v)) },
				},
			},
		},
		{
			Heading:      "Pump",
			HeadingColor: "#B5652F",
			Legend: []trendLegendEntry{
				{Color: "#D6A339", Label: "mL"},
				{Color: "#E8C978", Label: "Duration"},
			},
			BarHeight:     9,
			ValueFontSize: "11px",
			Series: []trendSeriesBuilder{
				{
					Color:  "#D6A339",
					Value:  func(d TrendDay) float64 { return float64(d.PumpMl) },
					Format: func(v float64) string { return fmt.Sprintf("%d mL", int(v)) },
				},
				{
					Color:  "#E8C978",
					Value:  func(d TrendDay) float64 { return float64(d.PumpDurationMinutes) },
					Format: func(v float64) string { return formatTrendDurationMinutes(int(v)) },
				},
			},
		},
	}

	views := make([]trendChartView, 0, len(builders))
	for _, spec := range builders {
		views = append(views, buildTrendChartView(trend, spec))
	}
	return views
}

// buildTrendChartView resolves one chart's rows: each series is scaled
// against the max value across the given days for that series alone, so a
// quiet pump week doesn't read as flat against a busy feed week.
func buildTrendChartView(days []TrendDay, spec trendChartBuilder) trendChartView {
	maxValues := make([]float64, len(spec.Series))
	for si, series := range spec.Series {
		for _, d := range days {
			if v := series.Value(d); v > maxValues[si] {
				maxValues[si] = v
			}
		}
	}

	// days arrives oldest-first (see TrendDay's doc comment); render
	// newest-first instead so the report day sits at the top of each chart,
	// closest to the rest of the report, rather than scrolled to the bottom.
	days = newestFirst(days)
	multiSeries := len(spec.Series) > 1

	rows := make([]trendRowView, 0, len(days)*len(spec.Series))
	for di, d := range days {
		for si, series := range spec.Series {
			rowPadding := ""
			switch {
			case di == 0 && si == 0:
				rowPadding = ""
			case si > 0:
				rowPadding = "padding-top:2px;"
			case multiSeries:
				rowPadding = "padding-top:8px;"
			default:
				rowPadding = "padding-top:5px;"
			}

			label := ""
			if si == 0 {
				label = d.Label
			}

			value := series.Value(d)
			barWidth := 0
			if maxValues[si] > 0 {
				barWidth = int(math.Round(trendBarTrackWidth * value / maxValues[si]))
				if barWidth > trendBarTrackWidth {
					barWidth = trendBarTrackWidth
				}
			}
			spacerWidth := trendBarTrackWidth - barWidth

			// Bar and track render as one seamless pill: whichever segment
			// is present alone gets fully rounded corners, and when both
			// are present the fill only rounds its left edge while the
			// track only rounds its right edge, so there's no visible seam
			// between the two colors.
			var segments []barSegmentView
			switch {
			case barWidth == 0:
				segments = []barSegmentView{
					{WidthPx: trendBarTrackWidth, HeightPx: spec.BarHeight, Color: trendBarTrackColor, BorderRadius: "4px"},
				}
			case spacerWidth == 0:
				segments = []barSegmentView{
					{WidthPx: barWidth, HeightPx: spec.BarHeight, Color: series.Color, BorderRadius: "4px"},
				}
			default:
				segments = []barSegmentView{
					{WidthPx: barWidth, HeightPx: spec.BarHeight, Color: series.Color, BorderRadius: "4px 0 0 4px"},
					{WidthPx: spacerWidth, HeightPx: spec.BarHeight, Color: trendBarTrackColor, BorderRadius: "0 4px 4px 0"},
				}
			}

			rows = append(rows, trendRowView{
				Label:        label,
				LabelPadding: template.CSS(rowPadding),
				Segments:     segments,
				Value:        series.Format(value),
				ValuePadding: template.CSS(rowPadding),
			})
		}
	}

	return trendChartView{
		Heading:       spec.Heading,
		HeadingColor:  spec.HeadingColor,
		Legend:        spec.Legend,
		ValueFontSize: spec.ValueFontSize,
		Rows:          rows,
	}
}

// newestFirst returns a copy of days in reverse order, for chart rendering
// that wants the most recent day first without mutating the caller's slice
// or affecting the plain-text trend log, which reads more naturally
// oldest-first as a chronological list.
func newestFirst(days []TrendDay) []TrendDay {
	reversed := make([]TrendDay, len(days))
	for i, d := range days {
		reversed[len(days)-1-i] = d
	}
	return reversed
}

// htmlBody renders the report email's HTML from templates/report_email.html.
// ExecuteTemplate can only fail here if the template and view model
// disagree about a field — a static, test-covered mismatch, not a runtime
// condition — so a failure indicates a genuine bug rather than bad input.
func htmlBody(report Report) string {
	var buf bytes.Buffer
	if err := reportEmailTmpl.ExecuteTemplate(&buf, "report-email", buildReportEmailView(report)); err != nil {
		panic(fmt.Errorf("execute report email template: %w", err))
	}
	return buf.String()
}

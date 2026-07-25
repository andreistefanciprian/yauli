package reportemail

import (
	"strings"
	"testing"

	"github.com/andreistefanciprian/yauli/backend-api/internal/aireport"
)

func TestHTMLAndTextBodiesRenderTheSameAIOutput(t *testing.T) {
	report := testReport()
	report.Card = []CardMetric{
		{Label: "Feeds", Count: 5, Detail: "350 ml"},
		{Label: "Sleep", Count: 2, Detail: "3 hr"},
		{Label: "Pump", Count: 1, Detail: "90 ml"},
		{Label: "Nappies", Count: 6},
	}
	report.Output = aireport.Output{
		Insights: []string{
			"The notes mention an easy settle after bath time.",
			"Nappies often followed feeds.",
			"Sleep was close to the recent daily average.",
		},
		Caveat: "One sleep was ongoing, so the total may change.",
	}

	text := textBody(report)
	html := htmlBody(report)
	insights := []string{
		"The notes mention an easy settle after bath time.",
		"Nappies often followed feeds.",
		"Sleep was close to the recent daily average.",
	}
	previousTextIndex := -1
	previousHTMLIndex := -1
	for _, insight := range insights {
		if !strings.Contains(text, insight) {
			t.Errorf("text body missing selected insight %q", insight)
		}
		if !strings.Contains(html, insight) {
			t.Errorf("HTML body missing selected insight %q", insight)
		}
		textIndex := strings.Index(text, insight)
		htmlIndex := strings.Index(html, insight)
		if textIndex <= previousTextIndex {
			t.Errorf("text body does not preserve insight order: %q", text)
		}
		if htmlIndex <= previousHTMLIndex {
			t.Errorf("HTML body does not preserve insight order")
		}
		previousTextIndex = textIndex
		previousHTMLIndex = htmlIndex
	}
	if !strings.Contains(text, report.Output.Caveat) || !strings.Contains(html, report.Output.Caveat) {
		t.Error("HTML and text bodies must both render the caveat")
	}
	for _, unwanted := range []string{"Highlights", "Patterns", "Comparison"} {
		if strings.Contains(text, unwanted) {
			t.Errorf("text body contains unwanted prose or heading %q", unwanted)
		}
		if strings.Contains(html, unwanted) {
			t.Errorf("HTML body contains unwanted prose or heading %q", unwanted)
		}
	}
}

func TestHTMLAndTextBodiesOmitEmptyCaveat(t *testing.T) {
	report := testReport()
	report.Output.Caveat = "  "

	if text := textBody(report); strings.Contains(text, "Caveat:") {
		t.Fatalf("text body contains empty caveat section: %q", text)
	}
	if html := htmlBody(report); strings.Contains(html, ">Caveat<") {
		t.Fatalf("HTML body contains empty caveat section: %q", html)
	}
}

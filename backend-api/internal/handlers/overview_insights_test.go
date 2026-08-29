package handlers

import "testing"

func TestParseOverviewInsightsRangeDays(t *testing.T) {
	tests := []struct {
		name   string
		raw    string
		want   int
		wantOK bool
	}{
		{name: "empty defaults to 30", raw: "", want: 30, wantOK: true},
		{name: "7 is allowed", raw: "7", want: 7, wantOK: true},
		{name: "90 is allowed", raw: "90", want: 90, wantOK: true},
		{name: "14 is rejected", raw: "14", wantOK: false},
		{name: "non-numeric is rejected", raw: "abc", wantOK: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseOverviewInsightsRangeDays(tt.raw)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if ok && got != tt.want {
				t.Fatalf("days = %d, want %d", got, tt.want)
			}
		})
	}
}

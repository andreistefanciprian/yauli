package handlers

import "testing"

func TestNormalizeFeedLabelsDedupesSupportedLabels(t *testing.T) {
	labels, ok := normalizeFeedLabels([]string{"burped_after", "fussy", "burped_after"})
	if !ok {
		t.Fatal("normalizeFeedLabels rejected supported labels")
	}
	want := []string{"burped_after", "fussy"}
	if len(labels) != len(want) {
		t.Fatalf("len(labels) = %d, want %d: %#v", len(labels), len(want), labels)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Fatalf("labels[%d] = %q, want %q", i, labels[i], want[i])
		}
	}
}

func TestNormalizeFeedLabelsRejectsUnsupportedLabel(t *testing.T) {
	if _, ok := normalizeFeedLabels([]string{"angry"}); ok {
		t.Fatal("normalizeFeedLabels accepted unsupported label")
	}
}

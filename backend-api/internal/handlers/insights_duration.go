package handlers

import "fmt"

// insightsDurationBasisLabel gives duration-based Insights categories one
// consistent way to disclose how many recorded events contributed minutes.
func insightsDurationBasisLabel(recorded, total int, singular, plural string) string {
	noun := plural
	if total == 1 {
		noun = singular
	}
	return fmt.Sprintf("Duration recorded for %d of %d %s", recorded, total, noun)
}

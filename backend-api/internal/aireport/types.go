package aireport

import "encoding/json"

const (
	InputSchemaVersion  = "ai_report_input.v1"
	OutputSchemaVersion = "ai_report_output.v2"
	PromptVersion       = "ai_report_prompt.v8"
)

// GenerationInput is the model-facing envelope. It deliberately contains
// deterministic report data and version identifiers, not raw auth/session
// context or frontend state.
type GenerationInput struct {
	InputSchemaVersion  string
	OutputSchemaVersion string
	PromptVersion       string
	ReportType          string
	Locale              string
	ReportData          any
}

// GenerationResult is the raw structured JSON returned by the model plus the
// model identifier used for cache metadata.
type GenerationResult struct {
	Model       string
	ContentJSON json.RawMessage
}

// Output is the channel-neutral AI report response. Insights arrive in display
// order so renderers do not need to interpret or reorganize model prose.
type Output struct {
	SchemaVersion string   `json:"schema_version"`
	Insights      []string `json:"insights"`
	Caveat        string   `json:"caveat"`
}

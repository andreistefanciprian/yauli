# Simplified AI Report Output

## Context

The first generic AI report contract split parent-facing prose across title,
summary, highlights, patterns, comparison, caveats, and follow-up questions.
Scheduled email then had to merge those categories back into a short Insights
section. That duplicated curation decisions in the renderer and made the code
harder to follow without adding useful product behaviour.

## Decision

Replace `ai_report_output.v1` with `ai_report_output.v2`:

```json
{
  "schema_version": "ai_report_output.v2",
  "insights": [],
  "caveat": ""
}
```

The model returns at most three concise insights in display order. It returns
one material caveat or an empty string. Deterministic renderers own headings,
KPI cards, charts, and encouragement; they display generated insights directly
without interpreting or reorganising prose.

The prompt version is bumped with the output schema version. Both versions are
part of the semantic input hash, so existing v1 cache entries remain stored but
cannot be reused for v2 requests.

## Alternatives Considered

Keep v1 and maintain renderer-side selection. This preserves compatibility but
keeps two layers responsible for deciding which prose matters.

Keep v1 and render every field. This exposes repetitive sections and makes the
daily email harder to scan.

Add an email-only output contract. This would make the shared report workflow
channel-specific and create another generation and cache path.

## Consequences

The `/reports/ai` response is a breaking public-contract change. API and future
MCP consumers receive ordered insights rather than named prose categories or
follow-up questions. Tests, prompt schema, documentation, and golden fixtures
move to v2 together.

Generation, validation, semantic caching, deterministic report data, scheduled
delivery, and fallback boundaries remain unchanged. Renderers become smaller
because they no longer infer meaning from generated text.

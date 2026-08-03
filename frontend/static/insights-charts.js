// Ninety-day charts keep one bar per day and scroll horizontally. Start at
// the latest data so the most relevant days are visible, then hide the hint
// as soon as the parent explores older days. Re-run after htmx replaces the
// Insights workspace for a category, metric, range, or selected-day change.
function initializeInsightsChartScrolling(root = document) {
  root.querySelectorAll(".insights-chart-90").forEach((chart) => {
    if (chart.dataset.latestScrollInitialized === "true") return;
    chart.dataset.latestScrollInitialized = "true";

    window.requestAnimationFrame(() => {
      const hint = chart.closest(".insights-chart-frame")?.querySelector("[data-insights-scroll-hint]");
      const maximumScroll = chart.scrollWidth - chart.clientWidth;
      if (maximumScroll <= 1) {
        if (hint) hint.hidden = true;
        return;
      }

      chart.scrollLeft = maximumScroll;

      function hideHintAfterExploringOlderDays() {
        if (chart.scrollLeft >= maximumScroll - 1) return;
        if (hint) hint.hidden = true;
        chart.removeEventListener("scroll", hideHintAfterExploringOlderDays);
      }
      chart.addEventListener("scroll", hideHintAfterExploringOlderDays, {passive: true});
    });
  });
}

document.addEventListener("DOMContentLoaded", () => initializeInsightsChartScrolling());
document.body.addEventListener("htmx:afterSwap", (event) => initializeInsightsChartScrolling(event.target));

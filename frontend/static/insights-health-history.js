// The Overview tab's "View health history" panel used to be an htmx round
// trip that re-fetched the exact same data already sitting in the page —
// backend-api's /insights/overview response always includes the full
// vaccine/medicine history regardless of whether the panel is open. So this
// is a plain client-side disclosure instead: the rows are already rendered
// (hidden), and this just toggles them.
//
// State is tracked here, not read off the DOM, so it survives an htmx
// swap of #insights-workspace (a range-pill click) — the panel stays open
// across a range change, matching how it behaved when this was server-side.
(() => {
  let open = false;

  function apply(root) {
    const toggle = root.querySelector("[data-health-history-toggle]");
    const panel = root.querySelector("[data-health-history]");
    if (!toggle || !panel) return;
    panel.hidden = !open;
    toggle.textContent = open ? "Hide health history" : "View health history →";
    toggle.setAttribute("aria-expanded", String(open));
  }

  document.body.addEventListener("click", (event) => {
    const toggle = event.target.closest("[data-health-history-toggle]");
    if (!toggle) return;
    open = !open;
    apply(document);
  });

  document.addEventListener("DOMContentLoaded", () => apply(document));
  document.body.addEventListener("htmx:afterSwap", (event) => apply(event.target));
})();

// Shared by every page with a segmented .range-nav/.range-pill control
// (timeline day pills, Sleep Insights' range selector, ...). Each pill swaps
// some workspace element via htmx instead of a full page reload, so the
// pills themselves are never re-rendered by the server on click — move the
// active/current state here instead, the same way the timeline's type
// filter chips already update themselves client-side. Browser back/forward
// is a real page reload, so the server-rendered active state already covers
// that path and needs no client-side sync here.
document.body.addEventListener("click", (event) => {
  const pill = event.target.closest(".range-pill");
  if (!pill) return;
  const rangeNav = pill.closest(".range-nav");
  if (!rangeNav) return;

  rangeNav.querySelectorAll(".range-pill").forEach((candidate) => {
    const isSelected = candidate === pill;
    candidate.classList.toggle("active", isSelected);
    if (isSelected) {
      candidate.setAttribute("aria-current", "page");
    } else {
      candidate.removeAttribute("aria-current");
    }
  });
});

// "Discuss with ChatGPT" on the Insights Overview tab. The trigger button
// (rendered inside #insights-workspace, so it survives htmx swaps on range
// change) carries the ready-made prompt in data-chatgpt-summary — built
// server-side in buildOverviewChatGptSummary. Clicking it opens a confirm
// dialog rather than sending straight away, since the prompt includes
// recorded health/medicine data. Confirming opens chatgpt.com with the
// prompt prefilled via its `?q=` URL param (the same mechanism ChatGPT's own
// share links use) — that only fills the composer, it does not auto-send.
(() => {
  const dialog = document.getElementById("insights-chatgpt-dialog");
  const confirmButton = document.getElementById("insights-chatgpt-confirm");
  const cancelButton = document.getElementById("insights-chatgpt-cancel");
  if (!dialog || !confirmButton || !cancelButton) return;

  let pendingSummary = "";

  document.body.addEventListener("click", (event) => {
    const trigger = event.target.closest("[data-chatgpt-discuss-trigger]");
    if (!trigger) return;
    pendingSummary = trigger.dataset.chatgptSummary || "";
    dialog.showModal();
  });

  cancelButton.addEventListener("click", () => dialog.close());

  // Backdrop click closes, matching the other dialogs in app.js.
  dialog.addEventListener("click", (event) => {
    if (event.target === dialog) dialog.close();
  });

  confirmButton.addEventListener("click", () => {
    const url = "https://chatgpt.com/?q=" + encodeURIComponent(pendingSummary);
    window.open(url, "_blank", "noopener");
    dialog.close();
  });
})();

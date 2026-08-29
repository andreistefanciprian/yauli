// Drives the single "Add Event" dialog: step 1 picks an event type, step 2
// shows only that type's form. Each form posts straight to its existing
// endpoint (/nappies, /feeds, /pumps, /baths, /sleeps, /observations, /temperatures, /medications, /growth-measurements) via htmx and swaps in
// the refreshed #timeline on success.

const dialog = document.getElementById("add-event-dialog");
const openButton = document.getElementById("add-event-button");
const closeButton = document.getElementById("add-event-close");
const backButton = document.getElementById("add-event-back");
const picker = document.getElementById("event-type-picker");
const title = document.getElementById("add-event-title");
const addEventForms = Array.from(dialog.querySelectorAll(".event-form"));
const babyTimezone = document.body.dataset.babyTimezone;
const babyDateTimeFormatter = new Intl.DateTimeFormat("en-AU", {
  timeZone: babyTimezone,
  year: "numeric",
  month: "2-digit",
  day: "2-digit",
  hour: "2-digit",
  minute: "2-digit",
  hourCycle: "h23",
});

const typeLabels = {
  nappy: "Log a nappy",
  feed: "Log a feed",
  pump: "Log pumping",
  bath: "Log a bath",
  sleep: "Log sleep",
  observation: "Log an observation",
  temperature: "Log temperature",
  medication: "Log medication",
  growth_measurement: "Log growth",
};

const medicationKindPresentation = {
  medicine: { label: "Medicine", placeholder: "e.g. infant paracetamol", unnamed: "Unnamed medicine" },
  vaccine: { label: "Vaccine", placeholder: "e.g. Rotavirus", unnamed: "Unnamed vaccine" },
  other: { label: "Other", placeholder: "e.g. supplement", unnamed: "Unnamed item" },
};

function hideDialogError(dialogEl) {
  const errorEl = dialogEl.querySelector(".dialog-error");
  if (errorEl) errorEl.hidden = true;
}

// A save/edit that fails (e.g. backend-api rejecting a future-dated event)
// gets its message shown here instead of failing silently — htmx doesn't
// swap non-2xx responses into the page by default, so without this the
// dialog would just sit there with no indication anything went wrong.
document.body.addEventListener("htmx:responseError", (event) => {
  const dialogEl = event.target.closest("dialog");
  if (!dialogEl) return;
  const errorEl = dialogEl.querySelector(".dialog-error");
  if (!errorEl) return;
  errorEl.textContent = event.detail.xhr.responseText || "Something went wrong. Please try again.";
  errorEl.hidden = false;
});

function showPickerStep() {
  picker.hidden = false;
  backButton.hidden = true;
  title.textContent = "Add event";
  addEventForms.forEach((form) => {
    form.hidden = true;
    // Clear whatever was entered so going Back and picking a type again
    // (the same one or a different one) never resubmits stale field
    // values or a manually-backdated time the user meant to keep editing.
    form.reset();
    if (form.matches("[data-medication-event]")) resetMedicationEvent(form);
  });
}

function showFormStep(type) {
  picker.hidden = true;
  backButton.hidden = false;
  title.textContent = typeLabels[type] || "Add event";

  const form = addEventForms.find((f) => f.dataset.type === type);
  addEventForms.forEach((f) => {
    f.hidden = f !== form;
  });
  if (form) {
    setFormToNow(form);
    const firstField = form.querySelector("input, textarea");
    if (firstField) firstField.focus();
  }
}

function localDateValue(date) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}`;
}

function localTimeValue(date) {
  const pad = (n) => String(n).padStart(2, "0");
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`;
}

function dateTimeValuesInBabyTimezone(date) {
  const values = {};
  babyDateTimeFormatter.formatToParts(date).forEach(({ type, value }) => {
    values[type] = value;
  });
  return {
    date: `${values.year}-${values.month}-${values.day}`,
    time: `${values.hour}:${values.minute}`,
  };
}

function parseLocalDateTime(dateValue, timeValue) {
  if (!dateValue || !timeValue) return null;
  const [year, month, day] = dateValue.split("-").map(Number);
  const [hour, minute] = timeValue.split(":").map(Number);
  if (![year, month, day, hour, minute].every(Number.isFinite)) return null;
  return new Date(year, month - 1, day, hour, minute);
}

function formatDuration(minutes) {
  if (!Number.isFinite(minutes) || minutes <= 0) return "";
  const hours = Math.floor(minutes / 60);
  const remainingMinutes = minutes % 60;
  if (hours === 0) return `${remainingMinutes}m`;
  if (remainingMinutes === 0) return `${hours}h`;
  return `${hours}h ${remainingMinutes}m`;
}

function selectedRadioValue(scope, name) {
  return scope.querySelector(`input[type="radio"][name="${name}"]:checked`)?.value || "";
}

function updatePooSizeFields(scope) {
  const containers = scope.querySelectorAll("[data-poo-size-field]");
  containers.forEach((container) => {
    const form = container.closest("form");
    if (!form) return;

    const kind = selectedRadioValue(form, "kind");
    const show = kind === "poo" || kind === "both";
    container.disabled = !show;
  });
}

function updateFeedAmountFields(scope) {
  scope.querySelectorAll("[data-feed-amount-field]").forEach((container) => {
    const feedFields = container.closest('form[data-type="feed"], [data-edit-type="feed"]');
    if (!feedFields || feedFields.hidden) return;

    const feedType = selectedRadioValue(feedFields, "type");
    const disableAmount = feedType === "breast";
    container.classList.toggle("field-disabled", disableAmount);
    container.querySelectorAll("input").forEach((input) => {
      input.disabled = disableAmount;
      if (disableAmount) input.value = "";
    });
  });
}

function updateMedicationFields(scope) {
  const itemContainers = scope.matches?.("[data-medication-item]") ? [scope] : [];
  itemContainers.push(...scope.querySelectorAll("[data-medication-item]"));
  itemContainers.forEach((container) => {
    const index = container.dataset.medicationIndex;
    updateMedicationKindFields(container, selectedRadioValue(container, `item_kind_${index}`) || "medicine");
    updateMedicationItemSummary(container);
  });
}

function updateMedicationKindFields(container, kind) {
  container.querySelectorAll("[data-medication-kind-fields]").forEach((fields) => {
    const enabled = fields.dataset.medicationKindFields === kind;
    fields.hidden = !enabled;
    if ("disabled" in fields) fields.disabled = !enabled;
    fields.querySelectorAll("input, select").forEach((field) => {
      field.disabled = !enabled;
    });
  });

  const presentation = medicationKindPresentation[kind] || medicationKindPresentation.other;
  const nameInput = container.querySelector('input[name="name"], input[name^="item_name_"]');
  if (nameInput) nameInput.placeholder = presentation.placeholder;
}

function updateMedicationItemSummary(item) {
  const index = item.dataset.medicationIndex;
  const kind = selectedRadioValue(item, `item_kind_${index}`) || "medicine";
  const name = item.querySelector(`[name="item_name_${index}"]`)?.value.trim() || "";
  const dose = item.querySelector(`[name="item_dose_value_${index}"]`)?.value.trim() || "";
  const doseUnit = item.querySelector(`[name="item_dose_unit_${index}"]`)?.value || "";
  const seriesDose = selectedRadioValue(item, `item_series_dose_${index}`);
  const presentation = medicationKindPresentation[kind] || medicationKindPresentation.other;
  const details = [];
  if (kind === "medicine" && dose) details.push(`${dose} ${doseUnit === "ml" ? "mL" : doseUnit}`);
  if (kind === "vaccine" && seriesDose) details.push(`${seriesDose.charAt(0).toUpperCase()}${seriesDose.slice(1)} dose`);

  const summary = item.querySelector("[data-medication-item-summary]");
  if (summary) summary.textContent = [name || presentation.unnamed, ...details].join(" · ");
  const kindLabel = item.querySelector("[data-medication-item-kind-label]");
  if (kindLabel) kindLabel.textContent = presentation.label;
}

function setMedicationItemExpanded(item, expanded) {
  item.classList.toggle("is-expanded", expanded);
  const editor = item.querySelector("[data-medication-item-editor]");
  if (editor) editor.hidden = !expanded;
  const toggle = item.querySelector("[data-medication-item-toggle]");
  if (toggle) toggle.setAttribute("aria-expanded", String(expanded));
}

function updateMedicationEventCount(form) {
  const items = form.querySelectorAll("[data-medication-item]");
  const count = items.length;
  const countBadge = form.querySelector("[data-medication-item-count]");
  if (countBadge) countBadge.textContent = String(count);
  form.querySelectorAll("[data-medication-remove-item]").forEach((button) => {
    button.disabled = count === 1;
  });
  items.forEach((item, index) => {
    const number = item.querySelector(".medication-item-number");
    if (number) number.textContent = String(index + 1);
  });
}

function resetMedicationEvent(form) {
  const items = Array.from(form.querySelectorAll("[data-medication-item]"));
  items.slice(1).forEach((item) => item.remove());
  if (items[0]) {
    setMedicationItemExpanded(items[0], true);
    updateMedicationFields(items[0]);
  }
  form.dataset.nextMedicationIndex = "1";
  updateMedicationEventCount(form);
}

function addMedicationItem(form) {
  const template = document.getElementById("medication-item-template");
  const list = form.querySelector("[data-medication-item-list]");
  if (!template || !list) return null;

  const index = Number.parseInt(form.dataset.nextMedicationIndex || "1", 10);
  const number = list.querySelectorAll("[data-medication-item]").length + 1;
  const wrapper = document.createElement("div");
  wrapper.innerHTML = template.innerHTML.replaceAll("__INDEX__", String(index)).replaceAll("__NUMBER__", String(number)).trim();
  const item = wrapper.firstElementChild;
  if (!item) return null;

  list.querySelectorAll("[data-medication-item]").forEach((existing) => setMedicationItemExpanded(existing, false));
  list.appendChild(item);
  form.dataset.nextMedicationIndex = String(index + 1);
  updateMedicationFields(item);
  updateMedicationEventCount(form);
  item.querySelector('input[name^="item_name_"]')?.focus();
  return item;
}

function setMedicationItemValues(item, values) {
  const index = item.dataset.medicationIndex;
  setRadioValue(item, `item_kind_${index}`, values.kind, "medicine");
  setFieldValue(item, `item_name_${index}`, values.name);
  setFieldValue(item, `item_description_${index}`, values.description);
  setFieldValue(item, `item_dose_value_${index}`, values.dose_value);
  setFieldValue(item, `item_dose_unit_${index}`, values.dose_unit || "ml");
  setRadioValue(item, `item_series_dose_${index}`, values.series_dose, "");
  updateMedicationFields(item);
}

function populateMedicationEventEdit(section, rawItems) {
  const list = section.querySelector("[data-medication-item-list]");
  if (!list) return;
  list.replaceChildren();
  section.dataset.nextMedicationIndex = "0";

  let items = [];
  try {
    items = JSON.parse(rawItems || "[]");
  } catch (_) {
    items = [];
  }
  if (!Array.isArray(items) || items.length === 0) items = [{ kind: "medicine", name: "" }];

  items.forEach((values) => {
    const item = addMedicationItem(section);
    if (item) setMedicationItemValues(item, values);
  });
  section.querySelectorAll("[data-medication-item]").forEach((item, index) => setMedicationItemExpanded(item, index === 0));
  updateMedicationEventCount(section);
}

// Set a form's date/time fields to now in the baby's configured timezone.
// Called each time a form is shown, since a value baked in at page load would
// go stale if the dialog is opened later in the same session.
function setFormToNow(form) {
  const now = dateTimeValuesInBabyTimezone(new Date());

  const dateInput = form.querySelector('input[type="date"]');
  if (dateInput) {
    dateInput.value = now.date;
    dateInput.max = now.date;
  }

  const timeInput = form.querySelector('input[type="time"]');
  if (timeInput) {
    timeInput.value = now.time;
  }

  form.querySelectorAll("[data-sleep-end-date]").forEach((input) => {
    input.value = "";
    input.max = now.date;
  });
  form.querySelectorAll("[data-sleep-end-time]").forEach((input) => {
    input.value = "";
  });
  form.querySelectorAll("[data-feed-end-date]").forEach((input) => {
    input.value = "";
    input.max = now.date;
  });
  form.querySelectorAll("[data-feed-end-time]").forEach((input) => {
    input.value = "";
  });
  form.querySelectorAll("[data-pump-end-date]").forEach((input) => {
    input.value = "";
    input.max = now.date;
  });
  form.querySelectorAll("[data-pump-end-time]").forEach((input) => {
    input.value = "";
  });
  updateSleepDuration(form);
  updateFeedDuration(form);
  updatePumpDuration(form);
  updatePooSizeFields(form);
  updateFeedAmountFields(form);
  updateMedicationFields(form);
}

function openDialog() {
  showPickerStep();
  dialog.showModal();
}

openButton.addEventListener("click", openDialog);
closeButton.addEventListener("click", () => dialog.close());
backButton.addEventListener("click", showPickerStep);

picker.addEventListener("click", (event) => {
  const choice = event.target.closest(".type-choice");
  if (choice) showFormStep(choice.dataset.type);
});

function handleMedicationEventClick(event) {
  const addButton = event.target.closest("[data-medication-add-item]");
  if (addButton) {
    const form = addButton.closest("[data-medication-event]");
    addMedicationItem(form);
    form.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  const removeButton = event.target.closest("[data-medication-remove-item]");
  if (removeButton && !removeButton.disabled) {
    const form = removeButton.closest("[data-medication-event]");
    removeButton.closest("[data-medication-item]")?.remove();
    updateMedicationEventCount(form);
    form.dispatchEvent(new Event("input", { bubbles: true }));
    return;
  }

  const doneButton = event.target.closest("[data-medication-item-done]");
  if (doneButton) {
    setMedicationItemExpanded(doneButton.closest("[data-medication-item]"), false);
    return;
  }

  const toggle = event.target.closest("[data-medication-item-toggle]");
  if (toggle) {
    const item = toggle.closest("[data-medication-item]");
    setMedicationItemExpanded(item, !item.classList.contains("is-expanded"));
  }
}

dialog.addEventListener("click", handleMedicationEventClick);

dialog.addEventListener("invalid", (event) => {
  const item = event.target.closest?.("[data-medication-item]");
  if (item) setMedicationItemExpanded(item, true);
}, true);

// Clicking the backdrop (outside the dialog's content box) closes it.
dialog.addEventListener("click", (event) => {
  if (event.target === dialog) dialog.close();
});

// Reset every form once the dialog is dismissed, however that happened
// (close button, backdrop click, Esc, or a successful save).
dialog.addEventListener("close", () => {
  addEventForms.forEach((form) => {
    form.reset();
    if (form.matches("[data-medication-event]")) resetMedicationEvent(form);
  });
  hideDialogError(dialog);
  reconcileTimelineDateRollover();
});

function onEventSaved() {
  dialog.close();
}

window.onEventSaved = onEventSaved;

// Event editing uses one dialog with type-specific sections. Timeline cards
// carry their current values in data-* attributes so the dialog can open
// immediately without another backend request.

const editDialog = document.getElementById("edit-event-dialog");
const editForm = document.getElementById("edit-event-form");
const editCloseButton = document.getElementById("edit-event-close");
const editDeleteButton = document.getElementById("edit-event-delete");
const editSaveButton = document.getElementById("edit-event-save");
const editTypeInput = document.getElementById("edit-event-type");
const editSections = Array.from(document.querySelectorAll(".edit-event-fields"));
const editTitle = document.getElementById("edit-event-title");
const editTimeLabel = editForm.querySelector("[data-edit-time-label]");
const editDateLabel = editForm.querySelector("[data-edit-date-label]");
const editOccurredAtFields = editForm.querySelector("[data-edit-occurred-at-fields]");
const editOccurredAtLabel = editForm.querySelector("[data-edit-occurred-at-label]");
let editFormBaseline = "";

editDialog.addEventListener("click", handleMedicationEventClick);

editDialog.addEventListener("invalid", (event) => {
  const item = event.target.closest?.("[data-medication-item]");
  if (item) setMedicationItemExpanded(item, true);
}, true);

function setSectionEnabled(section, enabled) {
  section.hidden = !enabled;
  section.querySelectorAll("input, textarea, select").forEach((field) => {
    field.disabled = !enabled;
  });
}

editSections.forEach((section) => setSectionEnabled(section, false));

function setRadioValue(form, name, value, fallback) {
  const targetValue = value || fallback;
  form.querySelectorAll(`input[type="radio"][name="${name}"]`).forEach((radio) => {
    radio.checked = radio.value === targetValue;
  });
}

function setFieldValue(form, name, value) {
  const field = form.querySelector(`[name="${name}"]`);
  if (field) {
    field.value = value || "";
    field.dispatchEvent(new Event("input", { bubbles: true }));
  }
}

function setCheckboxValues(form, name, rawValues) {
  const values = new Set((rawValues || "").split(",").filter(Boolean));
  form.querySelectorAll(`input[type="checkbox"][name="${name}"]`).forEach((checkbox) => {
    checkbox.checked = values.has(checkbox.value);
  });
}

function setSleepEndFromStart(form, durationMinutes) {
  const minutes = Number.parseInt(durationMinutes, 10);
  const startDate = form.querySelector('input[name="date"]');
  const startTime = form.querySelector('input[name="time"]');
  const endDate = form.querySelector("[data-sleep-end-date]");
  const endTime = form.querySelector("[data-sleep-end-time]");
  const start = parseLocalDateTime(startDate?.value, startTime?.value);
  if (!start || !endDate || !endTime) return;

  if (Number.isFinite(minutes) && minutes > 0) {
    const end = new Date(start);
    end.setMinutes(end.getMinutes() + minutes);
    endDate.value = localDateValue(end);
    endTime.value = localTimeValue(end);
    return;
  }

  endDate.value = "";
  endTime.value = "";
}

function setFeedEndFromStart(form, durationMinutes) {
  const minutes = Number.parseInt(durationMinutes, 10);
  const startDate = form.querySelector('input[name="date"]');
  const startTime = form.querySelector('input[name="time"]');
  const endDate = form.querySelector("[data-feed-end-date]");
  const endTime = form.querySelector("[data-feed-end-time]");
  const start = parseLocalDateTime(startDate?.value, startTime?.value);
  if (!start || !endDate || !endTime) return;

  if (Number.isFinite(minutes) && minutes > 0) {
    const end = new Date(start);
    end.setMinutes(end.getMinutes() + minutes);
    endDate.value = localDateValue(end);
    endTime.value = localTimeValue(end);
    return;
  }

  endDate.value = "";
  endTime.value = "";
}

function setPumpEndFromStart(form, durationMinutes) {
  const minutes = Number.parseInt(durationMinutes, 10);
  const startDate = form.querySelector('input[name="date"]');
  const startTime = form.querySelector('input[name="time"]');
  const endDate = form.querySelector("[data-pump-end-date]");
  const endTime = form.querySelector("[data-pump-end-time]");
  const start = parseLocalDateTime(startDate?.value, startTime?.value);
  if (!start || !endDate || !endTime) return;

  if (Number.isFinite(minutes) && minutes > 0) {
    const end = new Date(start);
    end.setMinutes(end.getMinutes() + minutes);
    endDate.value = localDateValue(end);
    endTime.value = localTimeValue(end);
    return;
  }

  endDate.value = "";
  endTime.value = "";
}

function syncNumberSliderThumb(input) {
  const slider = input.closest(".number-slider");
  const range = slider?.querySelector(".number-slider-range");
  if (!range || input.value === "") return;

  const value = Number(input.value);
  if (Number.isNaN(value)) return;
  const min = Number(range.min);
  const max = Number(range.max);
  range.value = String(Math.min(max, Math.max(min, value)));
}

function setSleepDurationValue(form, value) {
  const durationInput = form.querySelector("[data-sleep-duration-minutes]");
  if (!durationInput) return;
  durationInput.value = value;
  syncNumberSliderThumb(durationInput);
}

function setFeedDurationValue(form, value) {
  const durationInput = form.querySelector("[data-feed-duration-minutes]");
  if (!durationInput) return;
  durationInput.value = value;
  syncNumberSliderThumb(durationInput);
}

function setPumpDurationValue(form, value) {
  const durationInput = form.querySelector("[data-pump-duration-minutes]");
  if (!durationInput) return;
  durationInput.value = value;
  syncNumberSliderThumb(durationInput);
}

function updateSleepSubmitLabel(form, hasCompletedSleep) {
  const submitButton = form.querySelector("[data-sleep-submit-label]");
  if (!submitButton) return;
  submitButton.textContent = hasCompletedSleep ? "Save" : "Start";
}

function updateFeedSubmitLabel(form, hasCompletedFeed) {
  const submitButton = form.querySelector("[data-feed-submit-label]");
  if (!submitButton) return;
  submitButton.textContent = hasCompletedFeed ? "Save" : "Start";
}

function updatePumpSubmitLabel(form, hasCompletedPump) {
  const submitButton = form.querySelector("[data-pump-submit-label]");
  if (!submitButton) return;
  submitButton.textContent = hasCompletedPump ? "Save" : "Start";
}

function updateSleepDuration(scope) {
  const fields = scope.querySelectorAll("[data-sleep-time-fields]");
  fields.forEach((container) => {
    const form = container.closest("form");
    if (!form) return;

    const startDate = form.querySelector('input[name="date"]');
    const startTime = form.querySelector('input[name="time"]');
    const endDate = container.querySelector("[data-sleep-end-date]");
    const endTime = container.querySelector("[data-sleep-end-time]");
    const preview = container.querySelector("[data-sleep-duration-preview]");
    const start = parseLocalDateTime(startDate?.value, startTime?.value);
    const end = parseLocalDateTime(endDate?.value, endTime?.value);

    endDate.setCustomValidity("");
    endTime.setCustomValidity("");

    if (!start || !end) {
      setSleepDurationValue(form, "");
      updateSleepSubmitLabel(form, false);
      if (preview) preview.textContent = "Add wake-up time to calculate duration.";
      return;
    }

    if (end <= start) {
      const message = "Wake-up time must be after sleep start.";
      endTime.setCustomValidity(message);
      setSleepDurationValue(form, "");
      updateSleepSubmitLabel(form, false);
      if (preview) preview.textContent = message;
      return;
    }

    const minutes = Math.round((end - start) / 60000);
    setSleepDurationValue(form, String(minutes));
    updateSleepSubmitLabel(form, true);
    if (preview) preview.textContent = `Duration: ${formatDuration(minutes)}`;
  });
}

function updateFeedDuration(scope) {
  const fields = scope.querySelectorAll("[data-feed-time-fields]");
  fields.forEach((container) => {
    const form = container.closest("form");
    if (!form) return;

    const startDate = form.querySelector('input[name="date"]');
    const startTime = form.querySelector('input[name="time"]');
    const endDate = container.querySelector("[data-feed-end-date]");
    const endTime = container.querySelector("[data-feed-end-time]");
    const preview = container.querySelector("[data-feed-duration-preview]");
    const start = parseLocalDateTime(startDate?.value, startTime?.value);
    const end = parseLocalDateTime(endDate?.value, endTime?.value);

    endDate.setCustomValidity("");
    endTime.setCustomValidity("");

    if (!start || !end) {
      setFeedDurationValue(form, "");
      updateFeedSubmitLabel(form, false);
      if (preview) preview.textContent = "Add finish time to calculate duration.";
      return;
    }

    if (end <= start) {
      const message = "Finish time must be after feed start.";
      endTime.setCustomValidity(message);
      setFeedDurationValue(form, "");
      updateFeedSubmitLabel(form, false);
      if (preview) preview.textContent = message;
      return;
    }

    const minutes = Math.round((end - start) / 60000);
    setFeedDurationValue(form, String(minutes));
    updateFeedSubmitLabel(form, true);
    if (preview) preview.textContent = `Duration: ${formatDuration(minutes)}`;
  });
}

function updatePumpDuration(scope) {
  const fields = scope.querySelectorAll("[data-pump-time-fields]");
  fields.forEach((container) => {
    const form = container.closest("form");
    if (!form) return;

    const startDate = form.querySelector('input[name="date"]');
    const startTime = form.querySelector('input[name="time"]');
    const endDate = container.querySelector("[data-pump-end-date]");
    const endTime = container.querySelector("[data-pump-end-time]");
    const preview = container.querySelector("[data-pump-duration-preview]");
    const start = parseLocalDateTime(startDate?.value, startTime?.value);
    const end = parseLocalDateTime(endDate?.value, endTime?.value);

    endDate.setCustomValidity("");
    endTime.setCustomValidity("");

    if (!start || !end) {
      setPumpDurationValue(form, "");
      updatePumpSubmitLabel(form, false);
      if (preview) preview.textContent = "Add finish time to calculate duration.";
      return;
    }

    if (end <= start) {
      const message = "Finish time must be after pump start.";
      endTime.setCustomValidity(message);
      setPumpDurationValue(form, "");
      updatePumpSubmitLabel(form, false);
      if (preview) preview.textContent = message;
      return;
    }

    const minutes = Math.round((end - start) / 60000);
    setPumpDurationValue(form, String(minutes));
    updatePumpSubmitLabel(form, true);
    if (preview) preview.textContent = `Duration: ${formatDuration(minutes)}`;
  });
}

function updateSleepEndFromDuration(form) {
  const durationInput = form.querySelector("[data-sleep-duration-minutes]");
  if (!durationInput) return;

  setSleepEndFromStart(form, durationInput.value);
  updateSleepDuration(form);
}

function updateFeedEndFromDuration(form) {
  const durationInput = form.querySelector("[data-feed-duration-minutes]");
  if (!durationInput) return;

  setFeedEndFromStart(form, durationInput.value);
  updateFeedDuration(form);
}

function updatePumpEndFromDuration(form) {
  const durationInput = form.querySelector("[data-pump-duration-minutes]");
  if (!durationInput) return;

  setPumpEndFromStart(form, durationInput.value);
  updatePumpDuration(form);
}

function editSectionForType(type) {
  return editSections.find((section) => section.dataset.editType === type);
}

function editFormState() {
  return JSON.stringify(Array.from(new FormData(editForm).entries()));
}

function updateEditSaveState() {
  editSaveButton.disabled = editFormState() === editFormBaseline;
}

function queueEditSaveStateUpdate() {
  queueMicrotask(updateEditSaveState);
}

function openEditDialog(card) {
  const type = card.dataset.eventType;
  const eventID = card.dataset.eventId;
  if (!type || !eventID) return;

  editForm.reset();
  const patchURL = `/events/${eventID}?selected_date=${encodeURIComponent(selectedTimelineDate())}`;
  editForm.setAttribute("hx-patch", patchURL);
  editForm.dataset.patchUrl = patchURL;
  const deleteURL = `/events/${eventID}?selected_date=${encodeURIComponent(selectedTimelineDate())}`;
  editDeleteButton.setAttribute("hx-delete", deleteURL);
  editDeleteButton.dataset.deleteUrl = deleteURL;
  editTypeInput.value = type;
  editTitle.textContent = typeLabels[type] ? typeLabels[type].replace("Log", "Edit") : "Edit event";
  const groupedStartFields = type === "sleep" || type === "feed" || type === "pump";
  if (editOccurredAtFields) editOccurredAtFields.classList.toggle("grouped-edit-time-fields", groupedStartFields);
  if (editOccurredAtLabel) {
    editOccurredAtLabel.hidden = !groupedStartFields;
    editOccurredAtLabel.textContent = type === "sleep" ? "Fell asleep" : "Started";
  }
  if (editTimeLabel) editTimeLabel.textContent = "Time";
  if (editDateLabel) editDateLabel.textContent = "Date";

  editSections.forEach((section) => {
    setSectionEnabled(section, section.dataset.editType === type);
  });
  const activeSection = editSectionForType(type);
  if (!activeSection) return;

  const editDateInput = editForm.querySelector('input[type="date"]');
  if (editDateInput) editDateInput.max = dateTimeValuesInBabyTimezone(new Date()).date;

  setFieldValue(editForm, "date", card.dataset.date);
  setFieldValue(editForm, "time", card.dataset.time);

  switch (type) {
    case "nappy":
      setRadioValue(activeSection, "kind", card.dataset.kind, "wet");
      setRadioValue(activeSection, "poo_size", card.dataset.pooSize, "medium");
      setCheckboxValues(activeSection, "labels", card.dataset.labels);
      updatePooSizeFields(editForm);
      setFieldValue(activeSection, "notes", card.dataset.notes);
      break;
    case "feed":
      setRadioValue(activeSection, "type", card.dataset.type, "expressed");
      setFieldValue(activeSection, "amount_ml", card.dataset.amountMl);
      setFieldValue(activeSection, "duration_minutes", card.dataset.durationMinutes);
      setFeedEndFromStart(editForm, card.dataset.durationMinutes);
      updateFeedDuration(editForm);
      updateFeedAmountFields(editForm);
      setCheckboxValues(activeSection, "labels", card.dataset.labels);
      setFieldValue(activeSection, "notes", card.dataset.notes);
      break;
    case "pump":
      setFieldValue(activeSection, "amount_ml", card.dataset.amountMl);
      setFieldValue(activeSection, "notes", card.dataset.notes);
      setFieldValue(activeSection, "duration_minutes", card.dataset.durationMinutes);
      setPumpEndFromStart(editForm, card.dataset.durationMinutes);
      updatePumpDuration(editForm);
      break;
    case "bath":
      setRadioValue(activeSection, "type", card.dataset.type, "bottom_part");
      setRadioValue(activeSection, "bath_time_basis", "start", "start");
      setFieldValue(activeSection, "notes", card.dataset.notes);
      setFieldValue(activeSection, "duration_minutes", card.dataset.durationMinutes);
      break;
    case "sleep":
      setRadioValue(activeSection, "type", card.dataset.type, "nap");
      setFieldValue(activeSection, "notes", card.dataset.notes);
      setFieldValue(activeSection, "duration_minutes", card.dataset.durationMinutes);
      setSleepEndFromStart(editForm, card.dataset.durationMinutes);
      updateSleepDuration(editForm);
      break;
    case "observation":
      setFieldValue(activeSection, "text", card.dataset.text);
      setFieldValue(activeSection, "category", card.dataset.category);
      break;
    case "temperature":
      setFieldValue(activeSection, "temperature_c", card.dataset.temperatureC);
      setFieldValue(activeSection, "method", card.dataset.method || "ear");
      setFieldValue(activeSection, "notes", card.dataset.notes);
      break;
    case "medication":
      populateMedicationEventEdit(activeSection, card.dataset.medicationItems);
      setFieldValue(activeSection, "notes", card.dataset.notes);
      break;
    case "growth_measurement":
      setFieldValue(activeSection, "weight_kg", card.dataset.weightKg);
      setFieldValue(activeSection, "length_cm", card.dataset.lengthCm);
      setFieldValue(activeSection, "head_circumference_cm", card.dataset.headCircumferenceCm);
      setFieldValue(activeSection, "notes", card.dataset.notes);
      break;
  }

  editFormBaseline = editFormState();
  updateEditSaveState();
  editDialog.showModal();
}

function selectedTimelineDate() {
  const dateInput = editForm.querySelector('input[name="selected_date"]');
  return dateInput ? dateInput.value : "";
}

// A quick-view popup shown on row click, ahead of the full edit form: icon,
// title, time, detail, and Edit/Delete actions. Its content is cloned
// straight from the clicked row's own already-rendered markup rather than
// rebuilt from data-* attributes, so it can never drift out of sync with
// how the row itself formats a title/detail/time.
const quickViewDialog = document.getElementById("event-quickview-dialog");
const quickViewMarkerWrap = document.getElementById("quickview-marker-wrap");
const quickViewTitle = document.getElementById("quickview-title");
const quickViewTime = document.getElementById("quickview-time");
const quickViewDetail = document.getElementById("quickview-detail");
const quickViewEditButton = document.getElementById("quickview-edit-button");
const quickViewDeleteButton = document.getElementById("quickview-delete-button");
let quickViewCard = null;

function eventTimeText(card) {
  const clock = card.querySelector(".event-time-clock");
  const date = card.querySelector(".event-time-date");
  const clockText = clock ? clock.textContent.trim() : card.querySelector(".event-time")?.textContent.trim() ?? "";
  return date ? `${date.textContent.trim()}, ${clockText}` : clockText;
}

function openQuickView(card) {
  const eventID = card.dataset.eventId;
  if (!eventID) return;

  quickViewCard = card;

  const cssClass = card.dataset.cssClass;
  const icon = card.querySelector(".event-marker .event-type-icon");
  quickViewMarkerWrap.className = "quickview-marker-wrap" + (cssClass ? ` event-${cssClass}` : "");
  quickViewMarkerWrap.innerHTML = `<span class="event-marker">${icon ? icon.outerHTML : ""}</span>`;

  quickViewTitle.innerHTML = card.querySelector(".event-type")?.innerHTML ?? "";
  quickViewTime.textContent = eventTimeText(card);

  const detail = card.querySelector(".event-detail");
  if (detail) {
    quickViewDetail.innerHTML = detail.innerHTML;
    quickViewDetail.hidden = false;
  } else {
    quickViewDetail.innerHTML = "";
    quickViewDetail.hidden = true;
  }

  const deleteURL = `/events/${eventID}?selected_date=${encodeURIComponent(selectedTimelineDate())}`;
  quickViewDeleteButton.setAttribute("hx-delete", deleteURL);
  quickViewDeleteButton.dataset.deleteUrl = deleteURL;

  quickViewDialog.showModal();
}

quickViewEditButton.addEventListener("click", () => {
  const card = quickViewCard;
  quickViewDialog.close();
  if (card) openEditDialog(card);
});

quickViewDialog.addEventListener("click", (event) => {
  if (event.target === quickViewDialog) quickViewDialog.close();
});

quickViewDialog.addEventListener("close", () => {
  quickViewCard = null;
  delete quickViewDeleteButton.dataset.deleteUrl;
});

quickViewDeleteButton.addEventListener("htmx:configRequest", (event) => {
  if (quickViewDeleteButton.dataset.deleteUrl) event.detail.path = quickViewDeleteButton.dataset.deleteUrl;
});

document.body.addEventListener("click", (event) => {
  if (event.target.closest("button, a, input, select, textarea, .event-quick-action")) return;
  const card = event.target.closest(".event-card");
  if (card) openQuickView(card);
});

document.body.addEventListener("keydown", (event) => {
  if (event.key !== "Enter" && event.key !== " ") return;
  const trigger = event.target.closest(".event-card-open");
  if (!trigger || event.target !== trigger) return;
  event.preventDefault();
  openQuickView(trigger.closest(".event-card"));
});

editCloseButton.addEventListener("click", () => editDialog.close());

editDialog.addEventListener("click", (event) => {
  if (event.target === editDialog) editDialog.close();
});

editDialog.addEventListener("close", () => {
  editForm.reset();
  delete editForm.dataset.patchUrl;
  delete editDeleteButton.dataset.deleteUrl;
  editFormBaseline = "";
  editSaveButton.disabled = true;
  editSections.forEach((section) => setSectionEnabled(section, false));
  hideDialogError(editDialog);
  reconcileTimelineDateRollover();
});

editForm.addEventListener("htmx:configRequest", (event) => {
  if (editForm.dataset.patchUrl) event.detail.path = editForm.dataset.patchUrl;
});

editDeleteButton.addEventListener("htmx:configRequest", (event) => {
  if (editDeleteButton.dataset.deleteUrl) event.detail.path = editDeleteButton.dataset.deleteUrl;
});

editForm.addEventListener("input", queueEditSaveStateUpdate);
editForm.addEventListener("change", queueEditSaveStateUpdate);

function onEventEdited() {
  editDialog.close();
}

window.onEventEdited = onEventEdited;

function onEventDeleted() {
  editDialog.close();
  quickViewDialog.close();
}

window.onEventDeleted = onEventDeleted;

document.body.addEventListener("input", (event) => {
  const form = event.target.closest("form");
  if (!form) return;

  const medicationItem = event.target.closest("[data-medication-item]");
  if (medicationItem) updateMedicationItemSummary(medicationItem);

  if (event.target.matches("[data-sleep-duration-minutes]")) {
    updateSleepEndFromDuration(form);
    return;
  }

  if (event.target.matches("[data-feed-duration-minutes]")) {
    updateFeedEndFromDuration(form);
    return;
  }

  if (event.target.matches("[data-pump-duration-minutes]")) {
    updatePumpEndFromDuration(form);
    return;
  }

  if (event.target.closest("[data-sleep-time-fields]") || event.target.matches('input[name="date"], input[name="time"]')) {
    updateSleepDuration(form);
  }

  if (event.target.closest("[data-feed-time-fields]") || event.target.matches('input[name="date"], input[name="time"]')) {
    updateFeedDuration(form);
  }

  if (event.target.closest("[data-pump-time-fields]") || event.target.matches('input[name="date"], input[name="time"]')) {
    updatePumpDuration(form);
  }
});

document.body.addEventListener("change", (event) => {
  const medicationItem = event.target.closest("[data-medication-item]");
  if (medicationItem) updateMedicationFields(medicationItem);
  if (event.target.matches('input[type="radio"][name="kind"]')) {
    updatePooSizeFields(event.target.closest("form"));
  }
  if (event.target.matches('input[type="radio"][name="type"]')) {
    updateFeedAmountFields(event.target.closest("form"));
  }
});

// The day-range pills' active/current-state toggling lives in range-pills.js,
// shared with any other page that has its own .range-nav (e.g. Sleep
// Insights) — see that file for why it's client-side at all.

const timelineWorkspace = document.getElementById("timeline-workspace");
const timelineDayNavigation = document.querySelector(".timeline-filters .range-nav");
let desiredTimelineDate = timelineWorkspace?.dataset.selectedDate || "";
// The server owns calendar-day selection. Keep its rendered current date as
// the baseline so device clock skew cannot turn Today into a historical date.
let timelineCalendarDate = document.body.dataset.currentDate ||
  dateTimeValuesInBabyTimezone(new Date()).date;
let followsCurrentTimelineDay = desiredTimelineDate === timelineCalendarDate;
let timelineDateCheckTimer;

function timelineEditorOpen() {
  return dialog.open || editDialog.open;
}

// The server renders the day labels, so a page that remains open across the
// baby's midnight needs one full-page reconciliation. Preserve an explicitly
// selected historical date, but let a page following Today advance to the new
// current day. Defer while an editor is open so unsaved input is never lost.
function reconcileTimelineDateRollover() {
  if (!timelineWorkspace) return false;

  const currentCalendarDate = dateTimeValuesInBabyTimezone(new Date()).date;
  // A device slightly ahead of the server may request the new day too early.
  // The resulting page still carries the old server date, so this remains true
  // and the next page retries instead of becoming pinned to yesterday. A
  // device behind the server must not navigate a page the server already
  // rendered for the new day.
  if (currentCalendarDate <= timelineCalendarDate) return false;
  if (document.visibilityState === "hidden" || timelineEditorOpen()) return true;

  const destination = followsCurrentTimelineDay
    ? "/app"
    : `/app?date=${encodeURIComponent(desiredTimelineDate)}`;
  window.location.replace(destination);
  return true;
}

function scheduleTimelineDateRolloverCheck() {
  window.clearTimeout(timelineDateCheckTimer);
  timelineDateCheckTimer = window.setTimeout(() => {
    if (!reconcileTimelineDateRollover()) scheduleTimelineDateRolloverCheck();
  }, 60000);
}

function timelineDateFromURL(rawURL) {
  if (!rawURL) return "";
  return new URL(rawURL, window.location.href).searchParams.get("date") || "";
}

function setActiveTimelineDay(date) {
  if (!timelineDayNavigation) return;

  timelineDayNavigation.querySelectorAll(".range-pill").forEach((pill) => {
    const isSelected = timelineDateFromURL(pill.href) === date;
    pill.classList.toggle("active", isSelected);
    if (isSelected) {
      pill.setAttribute("aria-current", "page");
    } else {
      pill.removeAttribute("aria-current");
    }
  });
}

// Day navigation, event mutations, and live SSE refreshes can all return a
// complete #timeline-workspace. Remember the latest day choice before htmx
// starts its request, then reject any response rendered for an older choice.
// Without this guard, whichever request finishes last wins even when it was
// started for a day the user has since left.
document.body.addEventListener("click", (event) => {
  if (
    event.defaultPrevented || event.button !== 0 || event.metaKey ||
    event.ctrlKey || event.shiftKey || event.altKey
  ) return;
  const pill = event.target.closest?.(".timeline-filters .range-pill");
  if (!pill) return;
  if (reconcileTimelineDateRollover()) {
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  const selectedDate = timelineDateFromURL(pill.href);
  if (selectedDate) {
    desiredTimelineDate = selectedDate;
    followsCurrentTimelineDay = selectedDate === timelineCalendarDate;
  }
}, true);

document.body.addEventListener("htmx:beforeSwap", (event) => {
  if (event.target.id !== "timeline-workspace" || !desiredTimelineDate) return;

  const response = document.createElement("template");
  response.innerHTML = event.detail.serverResponse;
  const responseDate = response.content.querySelector("#timeline-workspace")?.dataset.selectedDate;
  if (responseDate && responseDate !== desiredTimelineDate) {
    event.detail.shouldSwap = false;
  }
});

// The shared pill script updates selection immediately on click. If the
// latest day request itself fails, restore the pill to the date that is still
// rendered instead of leaving the controls and timeline out of agreement.
document.body.addEventListener("htmx:afterRequest", (event) => {
  const pill = event.target.closest?.(".timeline-filters .range-pill");
  if (!pill || event.detail.successful === true) return;

  const requestedDate = timelineDateFromURL(event.detail.requestConfig?.path);
  if (!requestedDate || requestedDate !== desiredTimelineDate) return;

  const renderedDate = document.getElementById("timeline-workspace")?.dataset.selectedDate;
  if (!renderedDate) return;
  desiredTimelineDate = renderedDate;
  followsCurrentTimelineDay = renderedDate === timelineCalendarDate;
  setActiveTimelineDay(renderedDate);
});

// The add-event and edit-event dialogs sit outside #timeline-workspace (they
// need to survive being open across a day switch), so each of their hidden
// selected_date fields was only ever set once, from whatever day was active
// on the original full page load. Switching days now just swaps
// #timeline-workspace via htmx rather than reloading the page, so without
// this those fields go stale: create/edit/delete an event after switching
// days and it silently re-targets the day you *started* the session on,
// while the day pill correctly shows the day you actually switched to. The
// rendered workspace carries its canonical selected date, so the forms are
// re-synced from that value after every successful swap. This also covers
// plain event mutations without depending on history-update timing.
document.body.addEventListener("htmx:afterSwap", (event) => {
  if (event.target.id !== "timeline-workspace") return;
  const selectedDate = event.target.dataset.selectedDate;
  if (!selectedDate) return;

  document.querySelectorAll('input[name="selected_date"]').forEach((input) => {
    if (!input.closest("#timeline-workspace")) input.value = selectedDate;
  });
});

// Timeline updates arrive as small invalidation signals over Server-Sent
// Events. The signal carries no baby data; it tells this page to re-fetch
// the selected day's canonical report + timeline HTML through the same
// /app HTMX path used by day navigation.
if (timelineWorkspace) {
  const TIMELINE_REFRESH_DEBOUNCE_MS = 150;
  const TIMELINE_REFRESH_RETRY_MIN_MS = 1000;
  const TIMELINE_REFRESH_RETRY_MAX_MS = 30000;

  let refreshTimer;
  let refreshInFlight = false;
  let refreshPending = false;
  let refreshRetryDelay = TIMELINE_REFRESH_RETRY_MIN_MS;
  let refreshRequestSequence = 0;

  function selectedTimelineURL() {
    return `/app?date=${encodeURIComponent(desiredTimelineDate)}`;
  }

  function runTimelineRefresh() {
    refreshTimer = undefined;
    if (reconcileTimelineDateRollover()) return;
    if (document.visibilityState === "hidden" || refreshInFlight) return;

    refreshPending = false;
    refreshInFlight = true;
    const requestID = String(++refreshRequestSequence);
    let completed = false;

    function completeTimelineRefresh(successful) {
      if (completed) return;
      completed = true;
      document.body.removeEventListener("htmx:afterRequest", onTimelineRefreshComplete);
      refreshInFlight = false;

      if (successful) {
        refreshRetryDelay = TIMELINE_REFRESH_RETRY_MIN_MS;
        if (refreshPending) scheduleTimelineRefresh();
        return;
      }

      refreshPending = true;
      const retryDelay = refreshRetryDelay;
      refreshRetryDelay = Math.min(refreshRetryDelay * 2, TIMELINE_REFRESH_RETRY_MAX_MS);
      scheduleTimelineRefresh(retryDelay);
    }

    function onTimelineRefreshComplete(event) {
      if (event.detail.requestConfig?.headers?.["X-Yauli-Timeline-Refresh"] !== requestID) return;
      completeTimelineRefresh(event.detail.successful === true);
    }

    document.body.addEventListener("htmx:afterRequest", onTimelineRefreshComplete);
    try {
      Promise.resolve(htmx.ajax("GET", selectedTimelineURL(), {
        source: document.body,
        target: "#timeline-workspace",
        swap: "outerHTML",
        headers: {"X-Yauli-Timeline-Refresh": requestID},
      })).catch(() => completeTimelineRefresh(false));
    } catch {
      completeTimelineRefresh(false);
    }
  }

  function scheduleTimelineRefresh(delay = TIMELINE_REFRESH_DEBOUNCE_MS) {
    refreshPending = true;
    if (document.visibilityState === "hidden" || refreshInFlight) return;

    window.clearTimeout(refreshTimer);
    refreshTimer = window.setTimeout(runTimelineRefresh, delay);
  }

  if ("EventSource" in window) {
    const timelineEvents = new EventSource("/timeline/events/stream");

    timelineEvents.addEventListener("ready", () => {
      // Reconcile on every connection, including reconnects after sleep,
      // network loss, a deploy, or access-token renewal.
      scheduleTimelineRefresh();
    });
    timelineEvents.addEventListener("timeline_changed", () => {
      scheduleTimelineRefresh();
    });
    timelineEvents.addEventListener("navigate", (event) => {
      if (event.data !== "/login" && event.data !== "/onboarding") return;
      timelineEvents.close();
      window.location.assign(event.data);
    });
  }

  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState !== "visible" || reconcileTimelineDateRollover()) return;
    if (refreshPending) {
      scheduleTimelineRefresh(0);
    }
  });
  window.addEventListener("pageshow", () => {
    if (!reconcileTimelineDateRollover() && refreshPending) scheduleTimelineRefresh(0);
  });
  window.addEventListener("online", () => {
    if (!reconcileTimelineDateRollover()) scheduleTimelineRefresh(0);
  });

  scheduleTimelineDateRolloverCheck();
}

// Timeline navigation and display filters live inside a collapsible section
// so they don't take up screen space when the timeline itself is what's
// wanted. Collapsed is the default; the expand/collapse state is remembered
// per device, same as the type filter below.

const filtersToggle = document.getElementById("timeline-filters-toggle");
const filtersBody = document.getElementById("timeline-filters-body");
const FILTERS_EXPANDED_STORAGE_KEY = "yauli-filters-expanded";

function setFiltersExpanded(expanded) {
  filtersBody.hidden = !expanded;
  filtersToggle.setAttribute("aria-expanded", String(expanded));
  filtersToggle.setAttribute("aria-label", expanded ? "Hide timeline filters" : "Show timeline filters");
  try {
    localStorage.setItem(FILTERS_EXPANDED_STORAGE_KEY, expanded ? "1" : "0");
  } catch {
    // Storage can be unavailable (e.g. private browsing) - the toggle still
    // works for the current page view, it just won't be remembered.
  }
}

function loadFiltersExpanded() {
  try {
    return localStorage.getItem(FILTERS_EXPANDED_STORAGE_KEY) === "1";
  } catch {
    return false;
  }
}

if (filtersToggle && filtersBody) {
  setFiltersExpanded(loadFiltersExpanded());

  filtersToggle.addEventListener("click", () => {
    setFiltersExpanded(filtersBody.hidden);
  });
}

// Event type filter: purely client-side, hiding/showing already-rendered
// cards, so switching types is instant and needs no round trip to the
// backend. The selection is remembered per device so it doesn't reset every
// time the page loads or the selected date changes.

const typeFilter = document.getElementById("type-filter");
const TYPE_FILTER_STORAGE_KEY = "yauli-event-filter";

function loadStoredEventFilter() {
  try {
    const types = JSON.parse(localStorage.getItem(TYPE_FILTER_STORAGE_KEY) || "[]");
    return Array.isArray(types) ? types : [];
  } catch {
    return [];
  }
}

function storeEventFilter(types) {
  try {
    localStorage.setItem(TYPE_FILTER_STORAGE_KEY, JSON.stringify(types));
  } catch {
    // Storage can be unavailable (e.g. private browsing) - the filter still
    // works for the current page view, it just won't be remembered.
  }
}

function activeEventFilters() {
  return Array.from(typeFilter.querySelectorAll('.type-filter-chip.active[data-filter-type]:not([data-filter-type="all"])'))
    .map((chip) => ({
      type: chip.dataset.filterType,
      name: chip.querySelector(".type-filter-chip-label")?.textContent.trim() || chip.dataset.filterType,
    }));
}

function activeEventFilterTypes() {
  return activeEventFilters().map((filter) => filter.type);
}

function setActiveEventFilterChips(types) {
  const hasSelection = types.length > 0;
  typeFilter.querySelectorAll(".type-filter-chip").forEach((chip) => {
    const isAll = chip.dataset.filterType === "all";
    chip.classList.toggle("active", isAll ? !hasSelection : types.includes(chip.dataset.filterType));
  });
}

function applyEventFilter() {
  const activeFilters = activeEventFilters();
  const activeTypes = activeFilters.map((filter) => filter.type);
  const cards = Array.from(document.querySelectorAll("#timeline .event-card"));
  let anyVisible = false;
  cards.forEach((card) => {
    const visible = activeTypes.length === 0 || activeTypes.includes(card.dataset.eventType);
    card.hidden = !visible;
    if (visible) anyVisible = true;
  });

  const filterEmptyMessage = document.getElementById("timeline-filter-empty");
  if (filterEmptyMessage) {
    if (activeFilters.length === 1) {
      filterEmptyMessage.textContent = `No events match the ${activeFilters[0].name} filter.`;
    } else if (activeFilters.length > 1) {
      filterEmptyMessage.textContent = `No events match the selected filters: ${activeFilters.map((filter) => filter.name).join(", ")}.`;
    }
    filterEmptyMessage.hidden = cards.length === 0 || anyVisible;
  }
}

if (typeFilter) {
  setActiveEventFilterChips(loadStoredEventFilter());
  applyEventFilter();

  typeFilter.addEventListener("click", (event) => {
    const chip = event.target.closest(".type-filter-chip");
    if (!chip) return;

    let types;
    if (chip.dataset.filterType === "all") {
      types = [];
    } else {
      const selected = new Set(activeEventFilterTypes());
      if (selected.has(chip.dataset.filterType)) {
        selected.delete(chip.dataset.filterType);
      } else {
        selected.add(chip.dataset.filterType);
      }
      types = Array.from(selected);
    }

    setActiveEventFilterChips(types);
    storeEventFilter(types);
    applyEventFilter();
  });

  // Re-apply the filter every time htmx swaps in fresh timeline markup
  // (after creating, editing, deleting, or finishing an event), since the
  // new cards start with every type visible.
  document.body.addEventListener("htmx:afterSwap", (event) => {
    if (event.target.id === "timeline" || event.target.id === "timeline-workspace") applyEventFilter();
  });
}

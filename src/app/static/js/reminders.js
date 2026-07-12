import { get, post, put, del } from '/static/js/lib/api.js';

// ---- Clock (nav signature element) ----
function tickClock() {
  const text = new Date().toLocaleTimeString([], { hour12: false });
  const clock = document.getElementById('clock');
  const clockMobile = document.getElementById('clock-mobile');
  if (clock) clock.textContent = text;
  if (clockMobile) clockMobile.textContent = text;
}
tickClock();
setInterval(tickClock, 1000);

// ---- Mobile nav drawer ----
const navToggle = document.getElementById('nav-toggle');
const navClose = document.getElementById('nav-close');
const drawer = document.getElementById('mobile-drawer');
const backdrop = document.getElementById('mobile-drawer-backdrop');

function openDrawer() {
  drawer.classList.remove('translate-x-full');
  backdrop.classList.remove('hidden');
  navToggle.setAttribute('aria-expanded', 'true');
}

function closeDrawer() {
  drawer.classList.add('translate-x-full');
  backdrop.classList.add('hidden');
  navToggle.setAttribute('aria-expanded', 'false');
}

navToggle.addEventListener('click', openDrawer);
navClose.addEventListener('click', closeDrawer);
backdrop.addEventListener('click', closeDrawer);
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeDrawer();
});
drawer.querySelectorAll('a').forEach((a) => a.addEventListener('click', closeDrawer));

// ---- Collapsible-mobile section helper ----
// Builds the <details>/<summary>/<div class="collapsible-body"> shell that
// makes a section collapse on mobile (native <details> disclosure; CSS in
// input.css hides the toggle and forces the body open at 768px+). Returns
// the pieces so callers can mount content into `body` and keep updating
// `labelEl.textContent` (e.g. New/Edit toggles) exactly as before.
function makeCollapsibleSection(labelText, detailsClassName) {
  const details = document.createElement('details');
  details.className = 'collapsible-mobile ' + detailsClassName;
  details.open = true;

  const summary = document.createElement('summary');
  summary.className =
    'collapsible-toggle flex items-center justify-between cursor-pointer select-none py-2 md:pointer-events-none';

  const labelEl = document.createElement('span');
  labelEl.className = 'text-sm font-medium text-ink';
  labelEl.textContent = labelText;
  summary.appendChild(labelEl);

  const svgNS = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('class', 'w-4 h-4 text-ink-dim md:hidden');
  svg.setAttribute('viewBox', '0 0 20 20');
  svg.setAttribute('fill', 'currentColor');
  svg.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(svgNS, 'path');
  path.setAttribute('fill-rule', 'evenodd');
  path.setAttribute('clip-rule', 'evenodd');
  path.setAttribute(
    'd',
    'M5.23 7.21a.75.75 0 011.06.02L10 11.168l3.71-3.938a.75.75 0 111.08 1.04l-4.25 4.5a.75.75 0 01-1.08 0l-4.25-4.5a.75.75 0 01.02-1.06z'
  );
  svg.appendChild(path);
  summary.appendChild(svg);

  details.appendChild(summary);

  const body = document.createElement('div');
  body.className = 'collapsible-body pt-2';
  details.appendChild(body);

  return { details, body, labelEl };
}

const app = document.getElementById('app');

// Delete-error element: created once, inserted as a sibling of #app so it
// survives renderList()'s replaceChildren() re-renders.
const deleteErrEl = document.createElement('p');
deleteErrEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', deleteErrEl);

const RECURRENCE_LABELS = {
  none: 'Does not repeat',
  daily: 'Daily',
  weekly: 'Weekly',
  monthly: 'Monthly',
  specific_days: 'Specific days',
};

const DAY_LABELS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];

// Form state (shared between create and edit modes)
let editingId = null;
let editingIsActive = true;
let formTitleEl, titleInput, remindAtInput, recurrenceSelect, daysFieldset, dayCheckboxes, submitBtn, cancelBtn, errEl;

async function loadList() {
  const res = await get('/api/reminders');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-danger';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  renderList(res.data ?? []);
}

function formatRemindAt(value) {
  const d = new Date(value);
  if (isNaN(d.getTime())) return value;
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function recurrenceLabel(item) {
  if (item.recurrence_type === 'specific_days' && item.recurrence_days) {
    const days = item.recurrence_days
      .split(',')
      .map((s) => DAY_LABELS[parseInt(s, 10)])
      .filter(Boolean);
    return days.length ? `Weekly on ${days.join(', ')}` : RECURRENCE_LABELS.specific_days;
  }
  return RECURRENCE_LABELS[item.recurrence_type] ?? item.recurrence_type;
}

function renderList(items) {
  app.replaceChildren();
  if (items.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No reminders yet.';
    app.appendChild(p);
    return;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-2';

  items.forEach((item) => {
    const li = document.createElement('li');
    li.className =
      'border border-hairline bg-surface-raised p-4 flex items-center justify-between gap-4' +
      (item.is_active ? '' : ' opacity-50');

    const info = document.createElement('div');
    info.className = 'flex-1 cursor-pointer';
    info.addEventListener('click', () => populateFormForEdit(item));

    const titleEl = document.createElement('p');
    titleEl.className = 'text-sm font-medium text-ink';
    titleEl.textContent = item.title;
    info.appendChild(titleEl);

    const metaEl = document.createElement('p');
    metaEl.className = 'text-xs text-ink-dim';
    metaEl.textContent = `${formatRemindAt(item.remind_at)} · ${recurrenceLabel(item)}`;
    info.appendChild(metaEl);

    li.appendChild(info);

    const actions = document.createElement('div');
    actions.className = 'flex items-center gap-2 shrink-0';

    const toggleBtn = document.createElement('button');
    toggleBtn.type = 'button';
    toggleBtn.className =
      'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
    toggleBtn.textContent = item.is_active ? 'Active' : 'Inactive';
    toggleBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      toggleBtn.disabled = true;
      await post('/api/reminders/' + item.id + '/toggle');
      await loadList();
    });
    actions.appendChild(toggleBtn);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'px-3 py-1.5 text-xs border border-danger text-danger hover:bg-danger/10 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      deleteBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/reminders/' + item.id);
      if (res.ok) {
        if (editingId === item.id) resetFormToCreateMode();
        await loadList();
      } else {
        deleteBtn.disabled = false;
        deleteErrEl.textContent = res.error ?? 'Failed to delete.';
        deleteErrEl.classList.remove('hidden');
      }
    });
    actions.appendChild(deleteBtn);

    li.appendChild(actions);
    ul.appendChild(li);
  });

  app.appendChild(ul);
}

function updateDaysVisibility() {
  daysFieldset.classList.toggle('hidden', recurrenceSelect.value !== 'specific_days');
}

function populateFormForEdit(item) {
  editingId = item.id;
  editingIsActive = item.is_active;
  formTitleEl.textContent = 'Edit Reminder';
  submitBtn.textContent = 'Save Changes';
  cancelBtn.classList.remove('hidden');

  titleInput.value = item.title;
  remindAtInput.value = item.remind_at;
  recurrenceSelect.value = item.recurrence_type || 'none';
  const selectedDays = new Set((item.recurrence_days || '').split(',').filter(Boolean));
  dayCheckboxes.forEach((cb) => {
    cb.checked = selectedDays.has(cb.value);
  });
  updateDaysVisibility();
  window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
}

function resetFormToCreateMode() {
  editingId = null;
  editingIsActive = true;
  formTitleEl.textContent = 'New Reminder';
  submitBtn.textContent = 'Add Reminder';
  cancelBtn.classList.add('hidden');
  titleInput.value = '';
  remindAtInput.value = '';
  recurrenceSelect.value = 'none';
  dayCheckboxes.forEach((cb) => {
    cb.checked = false;
  });
  updateDaysVisibility();
}

setupRemindersForm(document.getElementById('forms-container'));
// @inject-forms

async function init() {
  await loadList();
}

init();

function setupRemindersForm(container) {
  const { details, body, labelEl } = makeCollapsibleSection(
    'New Reminder',
    'border border-hairline bg-surface p-5 space-y-3 mt-4'
  );
  formTitleEl = labelEl;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const remindAtLabel = document.createElement('label');
  remindAtLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  remindAtLabel.textContent = 'Remind At';
  remindAtInput = document.createElement('input');
  remindAtInput.type = 'datetime-local';
  remindAtInput.name = 'remind_at';
  remindAtInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  remindAtInput.required = true;
  form.appendChild(remindAtLabel);
  form.appendChild(remindAtInput);

  const recurrenceFieldLabel = document.createElement('label');
  recurrenceFieldLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  recurrenceFieldLabel.textContent = 'Recurrence';
  recurrenceSelect = document.createElement('select');
  recurrenceSelect.name = 'recurrence_type';
  recurrenceSelect.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  [
    ['none', 'Does not repeat'],
    ['daily', 'Daily'],
    ['weekly', 'Weekly'],
    ['monthly', 'Monthly'],
    ['specific_days', 'Specific days'],
  ].forEach(([value, label]) => {
    const opt = document.createElement('option');
    opt.value = value;
    opt.textContent = label;
    recurrenceSelect.appendChild(opt);
  });
  recurrenceSelect.addEventListener('change', updateDaysVisibility);
  form.appendChild(recurrenceFieldLabel);
  form.appendChild(recurrenceSelect);

  daysFieldset = document.createElement('fieldset');
  daysFieldset.className = 'hidden flex flex-wrap gap-3 pt-1';
  dayCheckboxes = [];
  DAY_LABELS.forEach((label, index) => {
    const wrapperLabel = document.createElement('label');
    wrapperLabel.className = 'flex items-center gap-1 text-xs text-ink-dim';
    const cb = document.createElement('input');
    cb.type = 'checkbox';
    cb.value = String(index);
    cb.name = 'recurrence_day';
    wrapperLabel.appendChild(cb);
    const span = document.createElement('span');
    span.textContent = label;
    wrapperLabel.appendChild(span);
    daysFieldset.appendChild(wrapperLabel);
    dayCheckboxes.push(cb);
  });
  form.appendChild(daysFieldset);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  submitBtn.textContent = 'Add Reminder';
  btnRow.appendChild(submitBtn);

  cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors hidden';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', resetFormToCreateMode);
  btnRow.appendChild(cancelBtn);

  form.appendChild(btnRow);

  errEl = document.createElement('p');
  errEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');

    const recurrenceDays =
      recurrenceSelect.value === 'specific_days'
        ? dayCheckboxes.filter((cb) => cb.checked).map((cb) => cb.value).join(',')
        : '';

    const data = {
      title: titleInput.value,
      remind_at: remindAtInput.value,
      recurrence_type: recurrenceSelect.value,
      recurrence_days: recurrenceDays,
    };

    const res = editingId
      ? await put('/api/reminders/' + editingId, { ...data, is_active: editingIsActive })
      : await post('/api/reminders_create', data);

    submitBtn.disabled = false;
    if (res.ok) {
      resetFormToCreateMode();
      await loadList();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  container.appendChild(details);
}

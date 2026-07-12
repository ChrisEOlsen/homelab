import { get, post, put, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

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
    p.className = 'text-sm text-red-600';
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
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No reminders yet.';
    app.appendChild(p);
    return;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-2';

  items.forEach((item) => {
    const li = document.createElement('li');
    li.className =
      'border border-gray-200 rounded-lg p-4 bg-white flex items-center justify-between gap-4' +
      (item.is_active ? '' : ' opacity-50');

    const info = document.createElement('div');
    info.className = 'flex-1 cursor-pointer';
    info.addEventListener('click', () => populateFormForEdit(item));

    const titleEl = document.createElement('p');
    titleEl.className = 'text-sm font-medium text-gray-900';
    titleEl.textContent = item.title;
    info.appendChild(titleEl);

    const metaEl = document.createElement('p');
    metaEl.className = 'text-xs text-gray-500';
    metaEl.textContent = `${formatRemindAt(item.remind_at)} · ${recurrenceLabel(item)}`;
    info.appendChild(metaEl);

    li.appendChild(info);

    const actions = document.createElement('div');
    actions.className = 'flex items-center gap-2 shrink-0';

    const toggleBtn = document.createElement('button');
    toggleBtn.type = 'button';
    toggleBtn.className =
      'px-3 py-1.5 text-xs rounded border border-gray-300 hover:bg-gray-50 transition-colors';
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
      'px-3 py-1.5 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      deleteBtn.disabled = true;
      await del('/api/reminders/' + item.id);
      if (editingId === item.id) resetFormToCreateMode();
      await loadList();
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
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  formTitleEl = document.createElement('h3');
  formTitleEl.className = 'text-sm font-semibold text-gray-900';
  formTitleEl.textContent = 'New Reminder';
  wrapper.appendChild(formTitleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const remindAtLabel = document.createElement('label');
  remindAtLabel.className = 'block text-sm font-medium text-gray-700';
  remindAtLabel.textContent = 'Remind At';
  remindAtInput = document.createElement('input');
  remindAtInput.type = 'datetime-local';
  remindAtInput.name = 'remind_at';
  remindAtInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  remindAtInput.required = true;
  form.appendChild(remindAtLabel);
  form.appendChild(remindAtInput);

  const recurrenceFieldLabel = document.createElement('label');
  recurrenceFieldLabel.className = 'block text-sm font-medium text-gray-700';
  recurrenceFieldLabel.textContent = 'Recurrence';
  recurrenceSelect = document.createElement('select');
  recurrenceSelect.name = 'recurrence_type';
  recurrenceSelect.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
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
    wrapperLabel.className = 'flex items-center gap-1 text-xs text-gray-700';
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
  submitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Reminder';
  btnRow.appendChild(submitBtn);

  cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 text-sm rounded border border-gray-300 hover:bg-gray-50 transition-colors hidden';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', resetFormToCreateMode);
  btnRow.appendChild(cancelBtn);

  form.appendChild(btnRow);

  errEl = document.createElement('p');
  errEl.className = 'text-sm text-red-600 hidden';
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

  wrapper.appendChild(form);
  container.appendChild(wrapper);
}

import { get, post, put, del } from '/static/js/lib/api.js';
import { createModal } from '/static/js/lib/modal.js';

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

// ---- Row actions menu (kebab dropdown) ----
function makeActionsMenu(actions, toggleClassName) {
  const wrap = document.createElement('div');
  wrap.className = 'relative shrink-0';

  const toggleBtn = document.createElement('button');
  toggleBtn.type = 'button';
  toggleBtn.className =
    toggleClassName ||
    'px-2 py-1.5 text-ink-dim hover:text-ink hover:bg-surface-raised border border-hairline transition-colors leading-none';
  toggleBtn.textContent = '⋯';
  toggleBtn.setAttribute('aria-label', 'Actions');
  toggleBtn.setAttribute('aria-haspopup', 'true');
  toggleBtn.setAttribute('aria-expanded', 'false');
  wrap.appendChild(toggleBtn);

  const menu = document.createElement('div');
  menu.className = 'absolute right-0 top-full mt-1 min-w-32 bg-surface border border-hairline z-10 hidden';
  wrap.appendChild(menu);

  function closeMenu() {
    menu.classList.add('hidden');
    toggleBtn.setAttribute('aria-expanded', 'false');
    document.removeEventListener('click', onOutsideClick);
    document.removeEventListener('keydown', onKeydown);
  }
  function onOutsideClick(e) {
    if (!wrap.contains(e.target)) closeMenu();
  }
  function onKeydown(e) {
    if (e.key === 'Escape') closeMenu();
  }
  function openMenu() {
    menu.classList.remove('hidden');
    toggleBtn.setAttribute('aria-expanded', 'true');
    document.addEventListener('click', onOutsideClick);
    document.addEventListener('keydown', onKeydown);
  }

  toggleBtn.addEventListener('click', (e) => {
    e.stopPropagation();
    if (menu.classList.contains('hidden')) openMenu();
    else closeMenu();
  });

  actions.forEach(({ label, danger, onClick }) => {
    const item = document.createElement('button');
    item.type = 'button';
    item.className =
      'block w-full text-left px-3 py-2 text-xs transition-colors ' +
      (danger ? 'text-danger hover:bg-danger/10' : 'text-ink-dim hover:text-ink hover:bg-surface-raised');
    item.textContent = label;
    item.addEventListener('click', async (e) => {
      e.stopPropagation();
      closeMenu();
      await onClick();
    });
    menu.appendChild(item);
  });

  return wrap;
}

const app = document.getElementById('app');

const deleteErrEl = document.createElement('p');
deleteErrEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', deleteErrEl);

const FIELD_TYPES = ['text', 'date', 'time'];

let categories = [];
let selectedCategoryId = null;
let entries = [];

function parseSchema(category) {
  if (!category) return [];
  try {
    const parsed = JSON.parse(category.schema_def);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

// ---- Category modal (title + repeatable field rows) ----
let editingCategoryId = null;
const categoryModal = createModal('category-modal-title');
let catFormTitleEl, catTitleInput, catSubmitBtn, catErrEl, fieldRowsEl;
let fieldRows = []; // { row, nameInput, typeSelect }

function addFieldRow(initial) {
  const row = document.createElement('div');
  row.className = 'flex items-center gap-2';

  const nameInput = document.createElement('input');
  nameInput.type = 'text';
  nameInput.placeholder = 'Field name';
  nameInput.value = initial?.name ?? '';
  nameInput.className =
    'flex-1 bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  nameInput.required = true;

  const typeSelect = document.createElement('select');
  typeSelect.className =
    'bg-canvas border border-hairline px-3 py-2 text-sm text-ink focus:outline-none focus:border-accent';
  FIELD_TYPES.forEach((type) => {
    const opt = document.createElement('option');
    opt.value = type;
    opt.textContent = type;
    if (initial?.type === type) opt.selected = true;
    typeSelect.appendChild(opt);
  });

  const removeBtn = document.createElement('button');
  removeBtn.type = 'button';
  removeBtn.className = 'text-xs text-danger/70 hover:text-danger px-2';
  removeBtn.textContent = 'Remove';
  removeBtn.addEventListener('click', () => {
    fieldRows = fieldRows.filter((entry) => entry.row !== row);
    row.remove();
  });

  row.appendChild(nameInput);
  row.appendChild(typeSelect);
  row.appendChild(removeBtn);

  fieldRowsEl.appendChild(row);
  fieldRows.push({ row, nameInput, typeSelect });
}

function resetCategoryForm() {
  fieldRows.forEach((entry) => entry.row.remove());
  fieldRows = [];
}

function buildCategoryModal() {
  const heading = document.createElement('h3');
  heading.id = 'category-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'New Log Category';
  categoryModal.panel.appendChild(heading);
  catFormTitleEl = heading;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  catTitleInput = document.createElement('input');
  catTitleInput.type = 'text';
  catTitleInput.name = 'title';
  catTitleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  catTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(catTitleInput);

  const fieldsLabel = document.createElement('label');
  fieldsLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  fieldsLabel.textContent = 'Fields';
  form.appendChild(fieldsLabel);

  fieldRowsEl = document.createElement('div');
  fieldRowsEl.className = 'space-y-2 max-h-48 overflow-y-auto';
  form.appendChild(fieldRowsEl);

  const addFieldBtn = document.createElement('button');
  addFieldBtn.type = 'button';
  addFieldBtn.className =
    'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  addFieldBtn.textContent = '+ Add field';
  addFieldBtn.addEventListener('click', () => addFieldRow());
  form.appendChild(addFieldBtn);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => categoryModal.close());
  btnRow.appendChild(cancelBtn);

  catSubmitBtn = document.createElement('button');
  catSubmitBtn.type = 'submit';
  catSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  catSubmitBtn.textContent = 'Add Category';
  btnRow.appendChild(catSubmitBtn);

  form.appendChild(btnRow);

  catErrEl = document.createElement('p');
  catErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(catErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    catSubmitBtn.disabled = true;
    catErrEl.classList.add('hidden');

    const fields = fieldRows
      .filter((entry) => entry.nameInput.value.trim() !== '')
      .map((entry) => ({ name: entry.nameInput.value.trim(), type: entry.typeSelect.value }));

    const res = editingCategoryId
      ? await put('/api/log_categories/' + editingCategoryId, { title: catTitleInput.value, fields })
      : await post('/api/log_categories_create', { title: catTitleInput.value, fields });

    catSubmitBtn.disabled = false;
    if (res.ok) {
      categoryModal.close();
      await loadCategories();
    } else {
      catErrEl.textContent = res.error ?? 'Something went wrong.';
      catErrEl.classList.remove('hidden');
    }
  });

  categoryModal.panel.appendChild(form);
}

function openCategoryModalForCreate(trigger) {
  editingCategoryId = null;
  catFormTitleEl.textContent = 'New Log Category';
  catSubmitBtn.textContent = 'Add Category';
  catTitleInput.value = '';
  resetCategoryForm();
  addFieldRow();
  catErrEl.classList.add('hidden');
  categoryModal.open(trigger);
}

function openCategoryModalForEdit(cat) {
  editingCategoryId = cat.id;
  catFormTitleEl.textContent = 'Edit Log Category';
  catSubmitBtn.textContent = 'Save Changes';
  catTitleInput.value = cat.title;
  resetCategoryForm();
  const schema = parseSchema(cat);
  if (schema.length === 0) {
    addFieldRow();
  } else {
    schema.forEach((field) => addFieldRow(field));
  }
  catErrEl.classList.add('hidden');
  categoryModal.open();
}

async function loadCategories() {
  const res = await get('/api/logger');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-danger';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  categories = res.data ?? [];
  if (selectedCategoryId === null || !categories.some((c) => c.id === selectedCategoryId)) {
    selectedCategoryId = categories.length > 0 ? categories[0].id : null;
  }
  await loadEntries();
}

async function loadEntries() {
  if (selectedCategoryId === null) {
    entries = [];
    render();
    return;
  }
  const res = await get('/api/log_categories/' + selectedCategoryId + '/entries');
  entries = res.ok ? (res.data ?? []) : [];
  render();
}

function getSelectedCategory() {
  return categories.find((c) => c.id === selectedCategoryId) ?? null;
}

function render() {
  app.replaceChildren();

  const container = document.createElement('div');
  container.className = 'flex flex-col sm:flex-row gap-6';

  container.appendChild(renderSidebar());
  container.appendChild(renderMain());

  app.appendChild(container);
}

function renderSidebar() {
  const { details, body } = makeCollapsibleSection(
    'Categories',
    'w-full sm:w-56 shrink-0 border border-hairline bg-surface p-4 space-y-2'
  );

  if (categories.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No log categories yet.';
    body.appendChild(p);
  } else {
    const ul = document.createElement('ul');
    ul.className = 'space-y-1';

    categories.forEach((cat) => {
      const li = document.createElement('li');
      li.className =
        'group flex items-center justify-between gap-2 px-3 py-2 text-sm cursor-pointer border ' +
        (cat.id === selectedCategoryId
          ? 'bg-accent text-canvas border-accent'
          : 'bg-surface text-ink-dim border-hairline hover:bg-surface-raised hover:text-ink');

      const titleSpan = document.createElement('span');
      titleSpan.className = 'flex-1 truncate';
      titleSpan.textContent = cat.title;
      titleSpan.addEventListener('click', async () => {
        selectedCategoryId = cat.id;
        await loadEntries();
      });
      li.appendChild(titleSpan);

      const toggleClass =
        'text-xs px-1.5 py-0.5 leading-none ' +
        (cat.id === selectedCategoryId ? 'text-canvas/70 hover:text-canvas' : 'text-ink-dim hover:text-ink');

      li.appendChild(
        makeActionsMenu(
          [
            { label: 'Edit', onClick: () => openCategoryModalForEdit(cat) },
            {
              label: 'Delete',
              danger: true,
              onClick: async () => {
                if (!window.confirm('Delete this category and all its entries?')) return;
                deleteErrEl.classList.add('hidden');
                const res = await del('/api/log_categories/' + cat.id);
                if (res.ok) {
                  if (selectedCategoryId === cat.id) selectedCategoryId = null;
                  await loadCategories();
                } else {
                  deleteErrEl.textContent = res.error ?? 'Failed to delete.';
                  deleteErrEl.classList.remove('hidden');
                }
              },
            },
          ],
          toggleClass
        )
      );
      ul.appendChild(li);
    });

    body.appendChild(ul);
  }

  const addBtn = document.createElement('button');
  addBtn.type = 'button';
  addBtn.className =
    'w-full mt-2 px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  addBtn.textContent = '+ Category';
  addBtn.addEventListener('click', (e) => openCategoryModalForCreate(e.currentTarget));
  body.appendChild(addBtn);

  return details;
}

function renderMain() {
  const section = document.createElement('section');
  section.className = 'flex-1 space-y-4 min-w-0';

  const selected = getSelectedCategory();

  if (!selected) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'Create a log category to get started.';
    section.appendChild(p);
    return section;
  }

  const schema = parseSchema(selected);

  const heading = document.createElement('h2');
  heading.className = 'text-lg font-semibold text-ink';
  heading.textContent = selected.title;
  section.appendChild(heading);

  section.appendChild(renderEntryForm(selected, schema));
  section.appendChild(renderEntryTable(schema));

  return section;
}

function renderEntryTable(schema) {
  if (schema.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'This category has no fields defined.';
    return p;
  }

  if (entries.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No entries yet.';
    return p;
  }

  const wrap = document.createElement('div');
  wrap.className = 'overflow-x-auto border border-hairline bg-surface';

  const table = document.createElement('table');
  table.className = 'min-w-full text-sm';

  const thead = document.createElement('thead');
  const headRow = document.createElement('tr');
  headRow.className = 'border-b border-hairline';

  schema.forEach((field) => {
    const th = document.createElement('th');
    th.className = 'text-left px-4 py-2 text-xs uppercase tracking-wide text-ink-dim';
    th.textContent = field.name;
    headRow.appendChild(th);
  });

  const actionsTh = document.createElement('th');
  actionsTh.className = 'text-left px-4 py-2 text-xs uppercase tracking-wide text-ink-dim';
  actionsTh.textContent = 'Actions';
  headRow.appendChild(actionsTh);

  thead.appendChild(headRow);
  table.appendChild(thead);

  const tbody = document.createElement('tbody');

  entries.forEach((entry) => {
    let entryData = {};
    try {
      entryData = JSON.parse(entry.entry_data);
    } catch {
      entryData = {};
    }

    const row = document.createElement('tr');
    row.className = 'border-b border-hairline last:border-0';

    schema.forEach((field) => {
      const td = document.createElement('td');
      td.className = 'px-4 py-2 text-ink';
      const value = entryData[field.name];
      td.textContent = value === undefined || value === null || value === '' ? '-' : String(value);
      row.appendChild(td);
    });

    const actionsTd = document.createElement('td');
    actionsTd.className = 'px-4 py-2';
    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'px-3 py-1.5 text-xs border border-danger text-danger hover:bg-danger/10 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async () => {
      deleteBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/log_entries/' + entry.id);
      if (res.ok) {
        await loadEntries();
      } else {
        deleteBtn.disabled = false;
        deleteErrEl.textContent = res.error ?? 'Failed to delete.';
        deleteErrEl.classList.remove('hidden');
      }
    });
    actionsTd.appendChild(deleteBtn);
    row.appendChild(actionsTd);

    tbody.appendChild(row);
  });

  table.appendChild(tbody);
  wrap.appendChild(table);
  return wrap;
}

function renderEntryForm(category, schema) {
  const { details, body } = makeCollapsibleSection('New Entry', 'border border-hairline bg-surface p-5 space-y-3');

  if (schema.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'Add fields to this category to start logging entries.';
    body.appendChild(p);
    return details;
  }

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const fieldInputs = {};

  schema.forEach((field) => {
    const label = document.createElement('label');
    label.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
    label.textContent = field.name;

    const input = document.createElement('input');
    input.type = field.type === 'date' || field.type === 'time' ? field.type : 'text';
    input.name = field.name;
    input.className =
      'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';

    form.appendChild(label);
    form.appendChild(input);

    fieldInputs[field.name] = input;
  });

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  submitBtn.textContent = 'Add Entry';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');

    const data = {};
    schema.forEach((field) => {
      data[field.name] = fieldInputs[field.name].value;
    });

    const res = await post('/api/log_entries_create', {
      category_id: category.id,
      data,
    });

    submitBtn.disabled = false;
    if (res.ok) {
      form.reset();
      await loadEntries();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  return details;
}

buildCategoryModal();

async function init() {
  await loadCategories();
}

init();

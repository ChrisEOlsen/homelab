import { get, post, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');
const formsContainer = document.getElementById('forms-container');

const FIELD_TYPES = ['text', 'date', 'time'];

let categories = [];
let selectedCategoryId = null;
let entries = [];

// Category-create form state: repeatable field rows.
let catTitleInput;
let fieldRowsEl;
let catErrEl;
let fieldRows = []; // { row, nameInput, typeSelect }

async function loadCategories() {
  const res = await get('/api/logger');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-red-600';
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

function parseSchema(category) {
  if (!category) return [];
  try {
    const parsed = JSON.parse(category.schema_def);
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

function tabClass(active) {
  return (
    'px-3 py-1.5 text-xs rounded border transition-colors ' +
    (active
      ? 'bg-gray-900 text-white border-gray-900'
      : 'border-gray-300 text-gray-700 hover:bg-gray-50')
  );
}

function render() {
  app.replaceChildren();

  const tabsWrap = document.createElement('div');
  tabsWrap.className = 'flex flex-wrap gap-2';

  if (categories.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No log categories yet. Create one below.';
    app.appendChild(p);
  } else {
    categories.forEach((cat) => {
      const tab = document.createElement('div');
      tab.className = 'flex items-center gap-1';

      const btn = document.createElement('button');
      btn.type = 'button';
      btn.className = tabClass(cat.id === selectedCategoryId);
      btn.textContent = cat.title;
      btn.addEventListener('click', async () => {
        selectedCategoryId = cat.id;
        await loadEntries();
      });
      tab.appendChild(btn);

      const delBtn = document.createElement('button');
      delBtn.type = 'button';
      delBtn.className = 'text-xs text-red-400 hover:text-red-600 px-1';
      delBtn.title = 'Delete category';
      delBtn.textContent = '×';
      delBtn.addEventListener('click', async (e) => {
        e.stopPropagation();
        delBtn.disabled = true;
        await del('/api/log_categories/' + cat.id);
        if (selectedCategoryId === cat.id) selectedCategoryId = null;
        await loadCategories();
      });
      tab.appendChild(delBtn);

      tabsWrap.appendChild(tab);
    });
    app.appendChild(tabsWrap);
  }

  const selected = getSelectedCategory();
  const schema = parseSchema(selected);

  if (selected) {
    const entriesSection = document.createElement('div');
    entriesSection.className = 'mt-6 space-y-4';

    const heading = document.createElement('h2');
    heading.className = 'text-sm font-semibold text-gray-900';
    heading.textContent = selected.title + ' entries';
    entriesSection.appendChild(heading);

    entriesSection.appendChild(renderEntryForm(selected, schema));
    entriesSection.appendChild(renderEntryTable(schema));

    app.appendChild(entriesSection);
  }
}

function renderEntryTable(schema) {
  if (schema.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'This category has no fields defined.';
    return p;
  }

  if (entries.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No entries yet.';
    return p;
  }

  const wrap = document.createElement('div');
  wrap.className = 'overflow-x-auto border border-gray-200 rounded-lg bg-white';

  const table = document.createElement('table');
  table.className = 'min-w-full text-sm';

  const thead = document.createElement('thead');
  const headRow = document.createElement('tr');
  headRow.className = 'border-b border-gray-200';

  schema.forEach((field) => {
    const th = document.createElement('th');
    th.className = 'text-left px-4 py-2 font-medium text-gray-700';
    th.textContent = field.name;
    headRow.appendChild(th);
  });

  const actionsTh = document.createElement('th');
  actionsTh.className = 'text-left px-4 py-2 font-medium text-gray-700';
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
    row.className = 'border-b border-gray-100 last:border-0';

    schema.forEach((field) => {
      const td = document.createElement('td');
      td.className = 'px-4 py-2 text-gray-800';
      const value = entryData[field.name];
      td.textContent = value === undefined || value === null || value === '' ? '-' : String(value);
      row.appendChild(td);
    });

    const actionsTd = document.createElement('td');
    actionsTd.className = 'px-4 py-2';
    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'px-3 py-1.5 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async () => {
      deleteBtn.disabled = true;
      await del('/api/log_entries/' + entry.id);
      await loadEntries();
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
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3';

  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = 'New Entry';
  wrapper.appendChild(titleEl);

  if (schema.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'Add fields to this category to start logging entries.';
    wrapper.appendChild(p);
    return wrapper;
  }

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const fieldInputs = {};

  schema.forEach((field) => {
    const label = document.createElement('label');
    label.className = 'block text-sm font-medium text-gray-700';
    label.textContent = field.name;

    const input = document.createElement('input');
    input.type = field.type === 'date' || field.type === 'time' ? field.type : 'text';
    input.name = field.name;
    input.className =
      'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';

    form.appendChild(label);
    form.appendChild(input);

    fieldInputs[field.name] = input;
  });

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Entry';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-red-600 hidden';
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

  wrapper.appendChild(form);
  return wrapper;
}

function addFieldRow() {
  const row = document.createElement('div');
  row.className = 'flex items-center gap-2';

  const nameInput = document.createElement('input');
  nameInput.type = 'text';
  nameInput.placeholder = 'Field name';
  nameInput.className =
    'flex-1 border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  nameInput.required = true;

  const typeSelect = document.createElement('select');
  typeSelect.className =
    'border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  FIELD_TYPES.forEach((type) => {
    const opt = document.createElement('option');
    opt.value = type;
    opt.textContent = type;
    typeSelect.appendChild(opt);
  });

  const removeBtn = document.createElement('button');
  removeBtn.type = 'button';
  removeBtn.className = 'text-xs text-red-400 hover:text-red-600 px-2';
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
  catTitleInput.value = '';
  fieldRows.forEach((entry) => entry.row.remove());
  fieldRows = [];
  addFieldRow();
}

function setupLogCategoriesCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = 'New Log Category';
  wrapper.appendChild(titleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  catTitleInput = document.createElement('input');
  catTitleInput.type = 'text';
  catTitleInput.name = 'title';
  catTitleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  catTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(catTitleInput);

  const fieldsLabel = document.createElement('label');
  fieldsLabel.className = 'block text-sm font-medium text-gray-700';
  fieldsLabel.textContent = 'Fields';
  form.appendChild(fieldsLabel);

  fieldRowsEl = document.createElement('div');
  fieldRowsEl.className = 'space-y-2';
  form.appendChild(fieldRowsEl);

  const addFieldBtn = document.createElement('button');
  addFieldBtn.type = 'button';
  addFieldBtn.className =
    'px-3 py-1.5 text-xs rounded border border-gray-300 hover:bg-gray-50 transition-colors';
  addFieldBtn.textContent = '+ Add field';
  addFieldBtn.addEventListener('click', () => addFieldRow());
  form.appendChild(addFieldBtn);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className =
    'block px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Category';
  form.appendChild(submitBtn);

  catErrEl = document.createElement('p');
  catErrEl.className = 'text-sm text-red-600 hidden';
  form.appendChild(catErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    catErrEl.classList.add('hidden');

    const fields = fieldRows
      .filter((entry) => entry.nameInput.value.trim() !== '')
      .map((entry) => ({ name: entry.nameInput.value.trim(), type: entry.typeSelect.value }));

    const res = await post('/api/log_categories_create', {
      title: catTitleInput.value,
      fields,
    });

    submitBtn.disabled = false;
    if (res.ok) {
      resetCategoryForm();
      await loadCategories();
    } else {
      catErrEl.textContent = res.error ?? 'Something went wrong.';
      catErrEl.classList.remove('hidden');
    }
  });

  wrapper.appendChild(form);
  container.appendChild(wrapper);

  addFieldRow();
}

setupLogCategoriesCreateForm(formsContainer);
// @inject-forms

async function init() {
  await loadCategories();
}

init();

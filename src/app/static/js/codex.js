import { get, post, put, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

let entries = [];

// Snippet form state (shared between create and edit modes)
let editingId = null;
let cxFormTitleEl, cxSubmitBtn, cxCancelBtn;
let cxTitleInput, cxLanguageInput, cxCodeInput, cxTagsInput, cxDescriptionInput, cxBundleIdInput, cxErrEl;

async function loadList() {
  const res = await get('/api/codex');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-red-600';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  entries = res.data ?? [];
  render();
}

function groupByBundle(items) {
  const groups = new Map();
  items.forEach((item) => {
    const key = item.bundle_id ? item.bundle_id : 'single-' + item.id;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(item);
  });
  return groups;
}

function render() {
  app.replaceChildren();

  if (entries.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No snippets yet.';
    app.appendChild(p);
    return;
  }

  const groups = groupByBundle(entries);
  const wrap = document.createElement('div');
  wrap.className = 'space-y-4';

  groups.forEach((items, key) => {
    wrap.appendChild(renderGroupCard(items, key));
  });

  app.appendChild(wrap);
}

function renderGroupCard(items, key) {
  const card = document.createElement('div');
  card.className = 'border border-gray-200 rounded-lg bg-white p-4 space-y-4';

  if (items.length > 1) {
    const header = document.createElement('div');
    header.className = 'flex items-center gap-2 text-xs text-gray-400';
    const label = document.createElement('span');
    label.className = 'font-medium uppercase tracking-wide';
    label.textContent = 'Bundle';
    const value = document.createElement('span');
    value.textContent = key;
    header.appendChild(label);
    header.appendChild(value);
    card.appendChild(header);
  }

  const list = document.createElement('div');
  list.className = 'space-y-4 divide-y divide-gray-100';
  items.forEach((item) => {
    list.appendChild(renderSnippet(item));
  });
  card.appendChild(list);

  return card;
}

function renderSnippet(item) {
  const wrap = document.createElement('div');
  wrap.className = 'pt-4 first:pt-0 space-y-2';

  const headerRow = document.createElement('div');
  headerRow.className = 'flex items-start justify-between gap-4';

  const info = document.createElement('div');
  info.className = 'min-w-0';

  const titleEl = document.createElement('p');
  titleEl.className = 'text-sm font-medium text-gray-900';
  titleEl.textContent = item.title;
  info.appendChild(titleEl);

  const metaEl = document.createElement('p');
  metaEl.className = 'text-xs text-gray-400';
  metaEl.textContent = item.language + (item.tags ? ' · ' + item.tags : '');
  info.appendChild(metaEl);

  if (item.description) {
    const descEl = document.createElement('p');
    descEl.className = 'text-xs text-gray-500 mt-1';
    descEl.textContent = item.description;
    info.appendChild(descEl);
  }

  headerRow.appendChild(info);

  const actions = document.createElement('div');
  actions.className = 'flex items-center gap-2 shrink-0';

  const editBtn = document.createElement('button');
  editBtn.type = 'button';
  editBtn.className =
    'px-3 py-1.5 text-xs rounded border border-gray-300 hover:bg-gray-50 transition-colors';
  editBtn.textContent = 'Edit';
  editBtn.addEventListener('click', () => populateFormForEdit(item));
  actions.appendChild(editBtn);

  const deleteBtn = document.createElement('button');
  deleteBtn.type = 'button';
  deleteBtn.className =
    'px-3 py-1.5 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors';
  deleteBtn.textContent = 'Delete';
  deleteBtn.addEventListener('click', async () => {
    deleteBtn.disabled = true;
    await del('/api/codex_entries/' + item.id);
    if (editingId === item.id) resetSnippetFormToCreateMode();
    await loadList();
  });
  actions.appendChild(deleteBtn);

  headerRow.appendChild(actions);
  wrap.appendChild(headerRow);

  const pre = document.createElement('pre');
  pre.className = 'bg-gray-900 text-gray-100 text-xs rounded p-3 overflow-x-auto';
  const code = document.createElement('code');
  code.textContent = item.code; // safe: textContent, never innerHTML
  pre.appendChild(code);
  wrap.appendChild(pre);

  return wrap;
}

function populateFormForEdit(item) {
  editingId = item.id;
  cxFormTitleEl.textContent = 'Edit Snippet';
  cxSubmitBtn.textContent = 'Save Changes';
  cxCancelBtn.classList.remove('hidden');
  cxTitleInput.value = item.title;
  cxLanguageInput.value = item.language ?? '';
  cxCodeInput.value = item.code;
  cxTagsInput.value = item.tags ?? '';
  cxDescriptionInput.value = item.description ?? '';
  cxBundleIdInput.value = item.bundle_id ?? '';
  window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
}

function resetSnippetFormToCreateMode() {
  editingId = null;
  cxFormTitleEl.textContent = 'New Snippet';
  cxSubmitBtn.textContent = 'Save Snippet';
  cxCancelBtn.classList.add('hidden');
  cxTitleInput.value = '';
  cxLanguageInput.value = '';
  cxCodeInput.value = '';
  cxTagsInput.value = '';
  cxDescriptionInput.value = '';
  cxBundleIdInput.value = '';
}

setupCodexEntriesCreateForm(document.getElementById('forms-container'));
// @inject-forms

async function init() {
  await loadList();
}

init();


function setupCodexEntriesCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  cxFormTitleEl = document.createElement('h3');
  cxFormTitleEl.className = 'text-sm font-semibold text-gray-900';
  cxFormTitleEl.textContent = 'New Snippet';
  wrapper.appendChild(cxFormTitleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  cxTitleInput = document.createElement('input');
  cxTitleInput.type = 'text';
  cxTitleInput.name = 'title';
  cxTitleInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  cxTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(cxTitleInput);

  const languageLabel = document.createElement('label');
  languageLabel.className = 'block text-sm font-medium text-gray-700';
  languageLabel.textContent = 'Language';
  cxLanguageInput = document.createElement('input');
  cxLanguageInput.type = 'text';
  cxLanguageInput.name = 'language';
  cxLanguageInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  form.appendChild(languageLabel);
  form.appendChild(cxLanguageInput);

  const codeLabel = document.createElement('label');
  codeLabel.className = 'block text-sm font-medium text-gray-700';
  codeLabel.textContent = 'Code';
  cxCodeInput = document.createElement('textarea');
  cxCodeInput.name = 'code';
  cxCodeInput.rows = 8;
  cxCodeInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm font-mono focus:outline-none focus:ring-1 focus:ring-gray-900';
  cxCodeInput.required = true;
  form.appendChild(codeLabel);
  form.appendChild(cxCodeInput);

  const tagsLabel = document.createElement('label');
  tagsLabel.className = 'block text-sm font-medium text-gray-700';
  tagsLabel.textContent = 'Tags';
  cxTagsInput = document.createElement('input');
  cxTagsInput.type = 'text';
  cxTagsInput.name = 'tags';
  cxTagsInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  form.appendChild(tagsLabel);
  form.appendChild(cxTagsInput);

  const descriptionLabel = document.createElement('label');
  descriptionLabel.className = 'block text-sm font-medium text-gray-700';
  descriptionLabel.textContent = 'Description';
  cxDescriptionInput = document.createElement('textarea');
  cxDescriptionInput.name = 'description';
  cxDescriptionInput.rows = 3;
  cxDescriptionInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  form.appendChild(descriptionLabel);
  form.appendChild(cxDescriptionInput);

  const bundleIdLabel = document.createElement('label');
  bundleIdLabel.className = 'block text-sm font-medium text-gray-700';
  bundleIdLabel.textContent = 'Bundle Id';
  cxBundleIdInput = document.createElement('input');
  cxBundleIdInput.type = 'text';
  cxBundleIdInput.name = 'bundle_id';
  cxBundleIdInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  form.appendChild(bundleIdLabel);
  form.appendChild(cxBundleIdInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  cxSubmitBtn = document.createElement('button');
  cxSubmitBtn.type = 'submit';
  cxSubmitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  cxSubmitBtn.textContent = 'Save Snippet';
  btnRow.appendChild(cxSubmitBtn);

  cxCancelBtn = document.createElement('button');
  cxCancelBtn.type = 'button';
  cxCancelBtn.className =
    'px-4 py-2 text-sm rounded border border-gray-300 hover:bg-gray-50 transition-colors hidden';
  cxCancelBtn.textContent = 'Cancel';
  cxCancelBtn.addEventListener('click', resetSnippetFormToCreateMode);
  btnRow.appendChild(cxCancelBtn);

  form.appendChild(btnRow);

  cxErrEl = document.createElement('p');
  cxErrEl.className = 'text-sm text-red-600 hidden';
  form.appendChild(cxErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    cxSubmitBtn.disabled = true;
    cxErrEl.classList.add('hidden');

    const data = {
      title: cxTitleInput.value,
      language: cxLanguageInput.value,
      code: cxCodeInput.value,
      tags: cxTagsInput.value,
      description: cxDescriptionInput.value,
      bundle_id: cxBundleIdInput.value,
    };

    const res = editingId
      ? await put('/api/codex_entries/' + editingId, data)
      : await post('/api/codex_entries_create', data);

    cxSubmitBtn.disabled = false;
    if (res.ok) {
      resetSnippetFormToCreateMode();
      await loadList();
    } else {
      cxErrEl.textContent = res.error ?? 'Something went wrong.';
      cxErrEl.classList.remove('hidden');
    }
  });

  wrapper.appendChild(form);
  container.appendChild(wrapper);
}

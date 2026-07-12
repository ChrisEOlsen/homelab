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

const app = document.getElementById('app');

// Delete-error element: created once, inserted as a sibling of #app so it
// survives render()'s replaceChildren() re-renders.
const deleteErrEl = document.createElement('p');
deleteErrEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', deleteErrEl);

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
    p.className = 'text-sm text-danger';
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
    p.className = 'text-sm text-ink-dim';
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
  card.className = 'border border-hairline bg-surface p-4 space-y-4';

  if (items.length > 1) {
    const header = document.createElement('div');
    header.className = 'flex items-center gap-2 font-mono text-xs text-ink-dim';
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
  list.className = 'space-y-4 divide-y divide-hairline';
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
  titleEl.className = 'text-sm font-medium text-ink';
  titleEl.textContent = item.title;
  info.appendChild(titleEl);

  const metaEl = document.createElement('p');
  metaEl.className = 'font-mono text-xs text-ink-dim';
  metaEl.textContent = item.language + (item.tags ? ' · ' + item.tags : '');
  info.appendChild(metaEl);

  if (item.description) {
    const descEl = document.createElement('p');
    descEl.className = 'font-mono text-xs text-ink-dim mt-1';
    descEl.textContent = item.description;
    info.appendChild(descEl);
  }

  headerRow.appendChild(info);

  const actions = document.createElement('div');
  actions.className = 'flex items-center gap-2 shrink-0';

  const editBtn = document.createElement('button');
  editBtn.type = 'button';
  editBtn.className =
    'px-3 py-1.5 text-xs font-mono border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  editBtn.textContent = 'Edit';
  editBtn.addEventListener('click', () => populateFormForEdit(item));
  actions.appendChild(editBtn);

  const deleteBtn = document.createElement('button');
  deleteBtn.type = 'button';
  deleteBtn.className =
    'px-3 py-1.5 text-xs font-mono border border-danger text-danger hover:bg-danger/10 transition-colors';
  deleteBtn.textContent = 'Delete';
  deleteBtn.addEventListener('click', async () => {
    deleteBtn.disabled = true;
    deleteErrEl.classList.add('hidden');
    const res = await del('/api/codex_entries/' + item.id);
    if (res.ok) {
      if (editingId === item.id) resetSnippetFormToCreateMode();
      await loadList();
    } else {
      deleteBtn.disabled = false;
      deleteErrEl.textContent = res.error ?? 'Failed to delete.';
      deleteErrEl.classList.remove('hidden');
    }
  });
  actions.appendChild(deleteBtn);

  headerRow.appendChild(actions);
  wrap.appendChild(headerRow);

  const pre = document.createElement('pre');
  pre.className = 'bg-canvas text-ink text-xs p-3 overflow-x-auto border border-hairline font-mono';
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
  wrapper.className = 'border border-hairline bg-surface p-5 space-y-3 mt-4';

  cxFormTitleEl = document.createElement('h3');
  cxFormTitleEl.className = 'font-mono text-xs tracking-widest text-ink-dim uppercase';
  cxFormTitleEl.textContent = 'New Snippet';
  wrapper.appendChild(cxFormTitleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  cxTitleInput = document.createElement('input');
  cxTitleInput.type = 'text';
  cxTitleInput.name = 'title';
  cxTitleInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  cxTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(cxTitleInput);

  const languageLabel = document.createElement('label');
  languageLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  languageLabel.textContent = 'Language';
  cxLanguageInput = document.createElement('input');
  cxLanguageInput.type = 'text';
  cxLanguageInput.name = 'language';
  cxLanguageInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(languageLabel);
  form.appendChild(cxLanguageInput);

  const codeLabel = document.createElement('label');
  codeLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  codeLabel.textContent = 'Code';
  cxCodeInput = document.createElement('textarea');
  cxCodeInput.name = 'code';
  cxCodeInput.rows = 8;
  cxCodeInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm font-mono text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  cxCodeInput.required = true;
  form.appendChild(codeLabel);
  form.appendChild(cxCodeInput);

  const tagsLabel = document.createElement('label');
  tagsLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  tagsLabel.textContent = 'Tags';
  cxTagsInput = document.createElement('input');
  cxTagsInput.type = 'text';
  cxTagsInput.name = 'tags';
  cxTagsInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(tagsLabel);
  form.appendChild(cxTagsInput);

  const descriptionLabel = document.createElement('label');
  descriptionLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  descriptionLabel.textContent = 'Description';
  cxDescriptionInput = document.createElement('textarea');
  cxDescriptionInput.name = 'description';
  cxDescriptionInput.rows = 3;
  cxDescriptionInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(descriptionLabel);
  form.appendChild(cxDescriptionInput);

  const bundleIdLabel = document.createElement('label');
  bundleIdLabel.className = 'block text-xs font-mono uppercase tracking-wide text-ink-dim mb-1';
  bundleIdLabel.textContent = 'Bundle Id';
  cxBundleIdInput = document.createElement('input');
  cxBundleIdInput.type = 'text';
  cxBundleIdInput.name = 'bundle_id';
  cxBundleIdInput.className = 'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(bundleIdLabel);
  form.appendChild(cxBundleIdInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  cxSubmitBtn = document.createElement('button');
  cxSubmitBtn.type = 'submit';
  cxSubmitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs font-mono uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  cxSubmitBtn.textContent = 'Save Snippet';
  btnRow.appendChild(cxSubmitBtn);

  cxCancelBtn = document.createElement('button');
  cxCancelBtn.type = 'button';
  cxCancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-mono uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors hidden';
  cxCancelBtn.textContent = 'Cancel';
  cxCancelBtn.addEventListener('click', resetSnippetFormToCreateMode);
  btnRow.appendChild(cxCancelBtn);

  form.appendChild(btnRow);

  cxErrEl = document.createElement('p');
  cxErrEl.className = 'text-sm text-danger mt-2 hidden';
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

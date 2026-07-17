import { get, post, put, del } from '/static/js/lib/api.js';
import { registerServiceWorker } from '/static/js/lib/push.js';
registerServiceWorker();
import { createModal } from '/static/js/lib/modal.js';
import { renderMarkdown } from '/static/js/lib/markdown.js';

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
const drawerBackdrop = document.getElementById('mobile-drawer-backdrop');

function openDrawer() {
  drawer.classList.remove('translate-x-full');
  drawerBackdrop.classList.remove('hidden');
  navToggle.setAttribute('aria-expanded', 'true');
}
function closeDrawer() {
  drawer.classList.add('translate-x-full');
  drawerBackdrop.classList.add('hidden');
  navToggle.setAttribute('aria-expanded', 'false');
}
navToggle.addEventListener('click', openDrawer);
navClose.addEventListener('click', closeDrawer);
drawerBackdrop.addEventListener('click', closeDrawer);
document.addEventListener('keydown', (e) => {
  if (e.key === 'Escape') closeDrawer();
});
drawer.querySelectorAll('a').forEach((a) => a.addEventListener('click', closeDrawer));

// ---- Row actions menu (kebab dropdown) ----
function makeActionsMenu(actions) {
  const wrap = document.createElement('div');
  wrap.className = 'relative shrink-0';

  const toggleBtn = document.createElement('button');
  toggleBtn.type = 'button';
  toggleBtn.className =
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

const errEl = document.createElement('p');
errEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', errEl);

function showErr(msg) {
  errEl.textContent = msg;
  errEl.classList.remove('hidden');
}
function clearErr() {
  errEl.classList.add('hidden');
}

// Module state
let entries = [];
let searchQuery = '';
let sortMode = 'name'; // 'name' | 'date' | 'language'
let expandedFolders = new Set();
let expandedFileIds = new Set();

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

function distinctFolders() {
  const set = new Set();
  entries.forEach((e) => {
    if (e.folder) set.add(e.folder);
  });
  return [...set].sort();
}

function sortEntries(list) {
  const arr = [...list];
  if (sortMode === 'date') {
    arr.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));
  } else if (sortMode === 'language') {
    arr.sort((a, b) => (a.language || '').localeCompare(b.language || '') || a.title.localeCompare(b.title));
  } else {
    arr.sort((a, b) => a.title.localeCompare(b.title));
  }
  return arr;
}

function matchesSearch(entry) {
  if (!searchQuery) return true;
  const q = searchQuery.toLowerCase();
  return (
    entry.title.toLowerCase().includes(q) ||
    (entry.description || '').toLowerCase().includes(q) ||
    (entry.folder || '').toLowerCase().includes(q)
  );
}

// ---- Tree building ----
// Folders aren't a separate table — a folder is just the common prefix of
// its entries' `folder` paths — so the tree is derived fresh from the flat
// entry list on every render.
function buildTree(list) {
  const root = { name: '', path: '', folders: new Map(), files: [] };
  list.forEach((entry) => {
    const parts = entry.folder ? entry.folder.split('/') : [];
    let node = root;
    let pathSoFar = '';
    parts.forEach((part) => {
      pathSoFar = pathSoFar ? pathSoFar + '/' + part : part;
      if (!node.folders.has(part)) {
        node.folders.set(part, { name: part, path: pathSoFar, folders: new Map(), files: [] });
      }
      node = node.folders.get(part);
    });
    node.files.push(entry);
  });
  return root;
}

function render() {
  app.replaceChildren();

  const controls = document.createElement('div');
  controls.className = 'flex flex-wrap items-center gap-2';

  const searchInput = document.createElement('input');
  searchInput.type = 'text';
  searchInput.placeholder = 'Search snippets...';
  searchInput.value = searchQuery;
  searchInput.className =
    'flex-1 min-w-[10rem] bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  searchInput.addEventListener('input', () => {
    searchQuery = searchInput.value;
    render();
    // render() rebuilds the input; keep typing uninterrupted.
    const newInput = app.querySelector('input[type="text"]');
    if (newInput) {
      newInput.focus();
      newInput.setSelectionRange(newInput.value.length, newInput.value.length);
    }
  });
  controls.appendChild(searchInput);

  const sortSelect = document.createElement('select');
  sortSelect.className =
    'bg-canvas border border-hairline px-3 py-2 text-sm text-ink focus:outline-none focus:border-accent';
  [
    ['name', 'Sort: Name'],
    ['date', 'Sort: Date'],
    ['language', 'Sort: Language'],
  ].forEach(([value, label]) => {
    const opt = document.createElement('option');
    opt.value = value;
    opt.textContent = label;
    if (value === sortMode) opt.selected = true;
    sortSelect.appendChild(opt);
  });
  sortSelect.addEventListener('change', () => {
    sortMode = sortSelect.value;
    render();
  });
  controls.appendChild(sortSelect);

  const newFolderBtn = document.createElement('button');
  newFolderBtn.type = 'button';
  newFolderBtn.className =
    'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  newFolderBtn.textContent = '+ New Folder';
  newFolderBtn.addEventListener('click', (e) => openFileModalForCreate(e.currentTarget, ''));
  controls.appendChild(newFolderBtn);

  const addFileBtn = document.createElement('button');
  addFileBtn.type = 'button';
  addFileBtn.className =
    'px-3 py-1.5 text-xs border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors';
  addFileBtn.textContent = '+ Add File';
  addFileBtn.addEventListener('click', (e) => openFileModalForCreate(e.currentTarget, null));
  controls.appendChild(addFileBtn);

  app.appendChild(controls);

  const tree = document.createElement('div');
  tree.className = 'mt-4 space-y-1';

  const root = buildTree(entries);
  const rootFolders = [...root.folders.values()].sort((a, b) => a.name.localeCompare(b.name));
  const searchActive = searchQuery.trim() !== '';

  let anyVisible = false;
  rootFolders.forEach((node) => {
    const el = renderFolderNode(node, searchActive);
    if (el) {
      tree.appendChild(el);
      anyVisible = true;
    }
  });

  const rootFiles = sortEntries(root.files.filter(matchesSearch));
  rootFiles.forEach((entry) => {
    tree.appendChild(renderFileRow(entry));
    anyVisible = true;
  });

  if (!anyVisible) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = entries.length === 0 ? 'No snippets yet.' : 'No matches.';
    tree.appendChild(p);
  }

  app.appendChild(tree);
}

function renderFolderNode(node, searchActive) {
  const childFolders = [...node.folders.values()].sort((a, b) => a.name.localeCompare(b.name));
  const childEls = [];
  childFolders.forEach((child) => {
    const el = renderFolderNode(child, searchActive);
    if (el) childEls.push(el);
  });

  const visibleFiles = sortEntries(node.files.filter(matchesSearch));

  if (searchActive && visibleFiles.length === 0 && childEls.length === 0) return null;

  const details = document.createElement('details');
  details.className = 'border border-hairline bg-surface';
  details.open = expandedFolders.has(node.path) || (searchActive && (visibleFiles.length > 0 || childEls.length > 0));
  details.addEventListener('toggle', () => {
    if (details.open) expandedFolders.add(node.path);
    else expandedFolders.delete(node.path);
  });

  const summary = document.createElement('summary');
  summary.className =
    'flex items-center justify-between gap-2 cursor-pointer select-none px-3 py-2 hover:bg-surface-raised transition-colors';

  const left = document.createElement('span');
  left.className = 'flex items-center gap-2 text-sm text-ink min-w-0';
  const icon = document.createElement('span');
  icon.className = 'text-ink-dim shrink-0';
  icon.textContent = '\u{1F4C1}'; // 📁 — decorative folder glyph, name conveys meaning on its own
  icon.setAttribute('aria-hidden', 'true');
  left.appendChild(icon);
  const nameSpan = document.createElement('span');
  nameSpan.className = 'truncate';
  nameSpan.textContent = node.name;
  left.appendChild(nameSpan);
  const countSpan = document.createElement('span');
  countSpan.className = 'text-xs text-ink-dim shrink-0';
  countSpan.textContent = String(countFiles(node));
  left.appendChild(countSpan);
  summary.appendChild(left);

  const menuHolder = document.createElement('span');
  menuHolder.addEventListener('click', (e) => e.preventDefault());
  menuHolder.appendChild(
    makeActionsMenu([
      { label: 'New File Here', onClick: () => openFileModalForCreate(null, node.path) },
      { label: 'Rename', onClick: () => openFolderRenameModal(node) },
      {
        label: 'Delete',
        danger: true,
        onClick: async () => {
          const count = countFiles(node);
          if (!window.confirm(`Delete folder "${node.name}" and ${count} snippet(s) inside it?`)) return;
          clearErr();
          const res = await post('/api/codex_folders_delete', { path: node.path });
          if (res.ok) {
            expandedFolders.delete(node.path);
            await loadList();
          } else {
            showErr(res.error ?? 'Failed to delete folder.');
          }
        },
      },
    ])
  );
  summary.appendChild(menuHolder);

  details.appendChild(summary);

  const body = document.createElement('div');
  body.className = 'pl-6 pb-2 space-y-1';
  childEls.forEach((el) => body.appendChild(el));
  visibleFiles.forEach((entry) => body.appendChild(renderFileRow(entry)));
  details.appendChild(body);

  return details;
}

function countFiles(node) {
  let count = node.files.length;
  node.folders.forEach((child) => {
    count += countFiles(child);
  });
  return count;
}

function renderFileRow(entry) {
  const wrap = document.createElement('div');
  wrap.className = 'border border-hairline bg-surface-raised';

  const row = document.createElement('div');
  row.className = 'flex items-center justify-between gap-2 px-3 py-2 cursor-pointer hover:bg-surface transition-colors';
  row.addEventListener('click', () => {
    if (expandedFileIds.has(entry.id)) expandedFileIds.delete(entry.id);
    else expandedFileIds.add(entry.id);
    render();
  });

  const left = document.createElement('span');
  left.className = 'flex items-center gap-2 text-sm text-ink min-w-0';
  const icon = document.createElement('span');
  icon.className = 'text-ink-dim shrink-0';
  icon.textContent = '\u{1F4C4}'; // 📄 — decorative file glyph, name conveys meaning on its own
  icon.setAttribute('aria-hidden', 'true');
  left.appendChild(icon);
  const nameSpan = document.createElement('span');
  nameSpan.className = 'truncate';
  nameSpan.textContent = entry.title;
  left.appendChild(nameSpan);
  if (entry.language) {
    const langSpan = document.createElement('span');
    langSpan.className = 'text-xs text-ink-dim shrink-0';
    langSpan.textContent = entry.language;
    left.appendChild(langSpan);
  }
  row.appendChild(left);

  const menuHolder = document.createElement('span');
  menuHolder.addEventListener('click', (e) => e.stopPropagation());
  menuHolder.appendChild(
    makeActionsMenu([
      { label: 'Edit', onClick: () => openFileModalForEdit(entry) },
      {
        label: 'Delete',
        danger: true,
        onClick: async () => {
          if (!window.confirm(`Delete "${entry.title}"?`)) return;
          clearErr();
          const res = await del('/api/codex_entries/' + entry.id);
          if (res.ok) {
            expandedFileIds.delete(entry.id);
            await loadList();
          } else {
            showErr(res.error ?? 'Failed to delete.');
          }
        },
      },
    ])
  );
  row.appendChild(menuHolder);

  wrap.appendChild(row);

  if (expandedFileIds.has(entry.id)) {
    wrap.appendChild(renderFileDetail(entry));
  }

  return wrap;
}

function renderFileDetail(entry) {
  const detail = document.createElement('div');
  detail.className = 'px-3 pb-3 space-y-3 border-t border-hairline pt-3';

  const codeWrap = document.createElement('div');
  codeWrap.className = 'relative';

  const pre = document.createElement('pre');
  pre.className = 'bg-canvas border border-hairline p-3 overflow-x-auto text-xs';
  const code = document.createElement('code');
  code.textContent = entry.code;
  pre.appendChild(code);
  codeWrap.appendChild(pre);

  const copyBtn = document.createElement('button');
  copyBtn.type = 'button';
  copyBtn.className =
    'absolute top-2 right-2 px-2 py-1 text-xs border border-hairline bg-surface text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  copyBtn.textContent = 'Copy';
  copyBtn.addEventListener('click', async () => {
    try {
      await navigator.clipboard.writeText(entry.code);
      copyBtn.textContent = 'Copied';
      setTimeout(() => (copyBtn.textContent = 'Copy'), 1200);
    } catch {
      copyBtn.textContent = 'Failed';
      setTimeout(() => (copyBtn.textContent = 'Copy'), 1200);
    }
  });
  codeWrap.appendChild(copyBtn);

  detail.appendChild(codeWrap);

  if (entry.description) {
    const notes = document.createElement('div');
    notes.className = 'text-sm text-ink';
    renderMarkdown(notes, entry.description);
    detail.appendChild(notes);
  }

  return detail;
}

// ---- File modal (create + edit) ----
let editingFileId = null;
const fileModal = createModal('file-modal-title');
let fileFormTitleEl, fileTitleInput, fileLanguageInput, fileFolderInput, folderDatalist, fileCodeInput, fileDescriptionInput, fileSubmitBtn, fileErrEl;

function buildFileModal() {
  fileModal.panel.classList.add('max-w-2xl');

  const heading = document.createElement('h3');
  heading.id = 'file-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'New File';
  fileModal.panel.appendChild(heading);
  fileFormTitleEl = heading;

  const form = document.createElement('form');
  form.className = 'space-y-3 max-h-[75vh] overflow-y-auto pr-1';

  const row1 = document.createElement('div');
  row1.className = 'flex gap-3';

  const titleWrap = document.createElement('div');
  titleWrap.className = 'flex-1';
  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  fileTitleInput = document.createElement('input');
  fileTitleInput.type = 'text';
  fileTitleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  fileTitleInput.required = true;
  titleWrap.appendChild(titleLabel);
  titleWrap.appendChild(fileTitleInput);
  row1.appendChild(titleWrap);

  const langWrap = document.createElement('div');
  langWrap.className = 'w-32';
  const langLabel = document.createElement('label');
  langLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  langLabel.textContent = 'Language';
  fileLanguageInput = document.createElement('input');
  fileLanguageInput.type = 'text';
  fileLanguageInput.placeholder = 'c';
  fileLanguageInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  langWrap.appendChild(langLabel);
  langWrap.appendChild(fileLanguageInput);
  row1.appendChild(langWrap);

  form.appendChild(row1);

  const folderLabel = document.createElement('label');
  folderLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  folderLabel.textContent = 'Folder';
  fileFolderInput = document.createElement('input');
  fileFolderInput.type = 'text';
  fileFolderInput.setAttribute('list', 'codex-folder-options');
  fileFolderInput.placeholder = 'e.g. Algorithms & Math (or Parent/Child to nest)';
  fileFolderInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  folderDatalist = document.createElement('datalist');
  folderDatalist.id = 'codex-folder-options';
  form.appendChild(folderLabel);
  form.appendChild(fileFolderInput);
  form.appendChild(folderDatalist);

  const folderHint = document.createElement('p');
  folderHint.className = 'text-xs text-ink-dim';
  folderHint.textContent = 'Typing a folder that doesn’t exist yet creates it — folders exist only as long as they hold at least one file.';
  form.appendChild(folderHint);

  const codeLabel = document.createElement('label');
  codeLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  codeLabel.textContent = 'Code';
  fileCodeInput = document.createElement('textarea');
  fileCodeInput.rows = 10;
  fileCodeInput.spellcheck = false;
  fileCodeInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-xs font-mono text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  fileCodeInput.required = true;
  form.appendChild(codeLabel);
  form.appendChild(fileCodeInput);

  const descLabel = document.createElement('label');
  descLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  descLabel.textContent = 'Notes (Markdown)';
  fileDescriptionInput = document.createElement('textarea');
  fileDescriptionInput.rows = 4;
  fileDescriptionInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(descLabel);
  form.appendChild(fileDescriptionInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => fileModal.close());
  btnRow.appendChild(cancelBtn);

  fileSubmitBtn = document.createElement('button');
  fileSubmitBtn.type = 'submit';
  fileSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  fileSubmitBtn.textContent = 'Add File';
  btnRow.appendChild(fileSubmitBtn);

  form.appendChild(btnRow);

  fileErrEl = document.createElement('p');
  fileErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(fileErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    fileSubmitBtn.disabled = true;
    fileErrEl.classList.add('hidden');

    const data = {
      title: fileTitleInput.value,
      language: fileLanguageInput.value,
      folder: fileFolderInput.value.trim(),
      code: fileCodeInput.value,
      description: fileDescriptionInput.value,
    };

    const res = editingFileId
      ? await put('/api/codex_entries/' + editingFileId, data)
      : await post('/api/codex_entries_create', data);

    fileSubmitBtn.disabled = false;
    if (res.ok) {
      if (data.folder) expandedFolders.add(data.folder);
      fileModal.close();
      await loadList();
    } else {
      fileErrEl.textContent = res.error ?? 'Something went wrong.';
      fileErrEl.classList.remove('hidden');
    }
  });

  fileModal.panel.appendChild(form);
}

function populateFolderDatalist() {
  folderDatalist.replaceChildren();
  distinctFolders().forEach((path) => {
    const opt = document.createElement('option');
    opt.value = path;
    folderDatalist.appendChild(opt);
  });
}

function openFileModalForCreate(trigger, folderPath) {
  editingFileId = null;
  fileFormTitleEl.textContent = 'New File';
  fileSubmitBtn.textContent = 'Add File';
  populateFolderDatalist();
  fileTitleInput.value = '';
  fileLanguageInput.value = '';
  fileFolderInput.value = folderPath ?? '';
  fileCodeInput.value = '';
  fileDescriptionInput.value = '';
  fileErrEl.classList.add('hidden');
  fileModal.open(trigger);
  if (folderPath === '') fileFolderInput.focus();
}

function openFileModalForEdit(entry) {
  editingFileId = entry.id;
  fileFormTitleEl.textContent = 'Edit File';
  fileSubmitBtn.textContent = 'Save Changes';
  populateFolderDatalist();
  fileTitleInput.value = entry.title;
  fileLanguageInput.value = entry.language ?? '';
  fileFolderInput.value = entry.folder ?? '';
  fileCodeInput.value = entry.code;
  fileDescriptionInput.value = entry.description ?? '';
  fileErrEl.classList.add('hidden');
  fileModal.open();
}

// ---- Folder rename modal ----
let renamingFolderNode = null;
const folderModal = createModal('folder-modal-title');
let folderNameInput, folderSubmitBtn, folderModalErrEl;

function buildFolderModal() {
  const heading = document.createElement('h3');
  heading.id = 'folder-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'Rename Folder';
  folderModal.panel.appendChild(heading);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const label = document.createElement('label');
  label.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  label.textContent = 'Folder name';
  folderNameInput = document.createElement('input');
  folderNameInput.type = 'text';
  folderNameInput.required = true;
  folderNameInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(label);
  form.appendChild(folderNameInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => folderModal.close());
  btnRow.appendChild(cancelBtn);

  folderSubmitBtn = document.createElement('button');
  folderSubmitBtn.type = 'submit';
  folderSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  folderSubmitBtn.textContent = 'Save';
  btnRow.appendChild(folderSubmitBtn);

  form.appendChild(btnRow);

  folderModalErrEl = document.createElement('p');
  folderModalErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(folderModalErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!renamingFolderNode) return;
    const newName = folderNameInput.value.trim();
    if (!newName) return;

    const parentPrefix = renamingFolderNode.path.includes('/')
      ? renamingFolderNode.path.slice(0, renamingFolderNode.path.lastIndexOf('/') + 1)
      : '';
    const newPath = parentPrefix + newName;

    folderSubmitBtn.disabled = true;
    folderModalErrEl.classList.add('hidden');
    const res = await post('/api/codex_folders_rename', { old_path: renamingFolderNode.path, new_path: newPath });
    folderSubmitBtn.disabled = false;
    if (res.ok) {
      expandedFolders.delete(renamingFolderNode.path);
      expandedFolders.add(newPath);
      folderModal.close();
      await loadList();
    } else {
      folderModalErrEl.textContent = res.error ?? 'Failed to rename folder.';
      folderModalErrEl.classList.remove('hidden');
    }
  });

  folderModal.panel.appendChild(form);
}

function openFolderRenameModal(node) {
  renamingFolderNode = node;
  folderNameInput.value = node.name;
  folderModalErrEl.classList.add('hidden');
  folderModal.open();
}

buildFileModal();
buildFolderModal();

async function init() {
  await loadList();
}

init();

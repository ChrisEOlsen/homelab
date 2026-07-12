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

// ---- Row actions menu (kebab dropdown) ----
// Replaces always-visible Edit/Delete buttons with a single "⋯" toggle and
// a small dropdown, so list rows read cleanly at a glance. `actions` is an
// array of { label, danger, onClick }; onClick may be async. Closes on an
// outside click, on Escape, or automatically after an action runs.
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

// Delete-error element: created once, inserted as a sibling of #app so it
// survives render()'s replaceChildren() re-renders.
const deleteErrEl = document.createElement('p');
deleteErrEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', deleteErrEl);

let categories = [];
let bookmarks = [];
let selectedCategoryId = 'all';

// Bookmark form state (shared between create and edit modes)
let editingId = null;
let bmFormTitleEl, bmSubmitBtn, bmCancelBtn, bmCategorySelect, bmTitleInput, bmUrlInput, bmDescriptionInput, bmErrEl;

async function loadList() {
  const res = await get('/api/bookmarks');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-danger';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  categories = res.data?.categories ?? [];
  bookmarks = res.data?.bookmarks ?? [];
  populateCategorySelect();
  render();
}

function populateCategorySelect() {
  if (!bmCategorySelect) return;
  const previousValue = bmCategorySelect.value;
  bmCategorySelect.replaceChildren();
  categories.forEach((cat) => {
    const opt = document.createElement('option');
    opt.value = String(cat.id);
    opt.textContent = cat.title;
    bmCategorySelect.appendChild(opt);
  });
  if (previousValue && categories.some((cat) => String(cat.id) === previousValue)) {
    bmCategorySelect.value = previousValue;
  }
}

function tabClass(active) {
  return (
    'px-3 py-1.5 text-xs border transition-colors ' +
    (active
      ? 'bg-accent text-canvas border-accent'
      : 'border-hairline text-ink-dim hover:bg-surface-raised hover:text-ink')
  );
}

function render() {
  app.replaceChildren();

  const tabsWrap = document.createElement('div');
  tabsWrap.className = 'flex flex-wrap gap-2';

  const allBtn = document.createElement('button');
  allBtn.type = 'button';
  allBtn.className = tabClass(selectedCategoryId === 'all');
  allBtn.textContent = 'All';
  allBtn.addEventListener('click', () => {
    selectedCategoryId = 'all';
    render();
  });
  tabsWrap.appendChild(allBtn);

  categories.forEach((cat) => {
    const tab = document.createElement('div');
    tab.className = 'flex items-center gap-1';

    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = tabClass(selectedCategoryId === cat.id);
    btn.textContent = cat.title;
    btn.addEventListener('click', () => {
      selectedCategoryId = cat.id;
      render();
    });
    tab.appendChild(btn);

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'text-xs text-danger/70 hover:text-danger px-1';
    delBtn.title = 'Delete category';
    delBtn.textContent = '×';
    delBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      delBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/bookmark_categories/' + cat.id);
      if (res.ok) {
        if (selectedCategoryId === cat.id) selectedCategoryId = 'all';
        await loadList();
      } else {
        delBtn.disabled = false;
        deleteErrEl.textContent = res.error ?? 'Failed to delete.';
        deleteErrEl.classList.remove('hidden');
      }
    });
    tab.appendChild(delBtn);

    tabsWrap.appendChild(tab);
  });

  app.appendChild(tabsWrap);

  const listWrap = document.createElement('div');
  listWrap.className = 'space-y-2 mt-4';

  const filtered =
    selectedCategoryId === 'all'
      ? bookmarks
      : bookmarks.filter((item) => item.category_id === selectedCategoryId);

  if (filtered.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No bookmarks yet.';
    listWrap.appendChild(p);
  } else {
    const ul = document.createElement('ul');
    ul.className = 'space-y-2';
    filtered.forEach((item) => {
      ul.appendChild(renderBookmarkRow(item));
    });
    listWrap.appendChild(ul);
  }

  app.appendChild(listWrap);
}

function renderBookmarkRow(item) {
  const li = document.createElement('li');
  li.className =
    'border border-hairline bg-surface-raised p-4 flex items-start justify-between gap-4';

  const info = document.createElement('div');
  info.className = 'flex-1 min-w-0';

  const link = document.createElement('a');
  link.href = item.url;
  link.target = '_blank';
  link.rel = 'noopener noreferrer';
  link.className = 'text-sm font-medium text-ink hover:underline';
  link.textContent = item.title;
  info.appendChild(link);

  const urlEl = document.createElement('p');
  urlEl.className = 'text-xs text-ink-dim truncate';
  urlEl.textContent = item.url;
  info.appendChild(urlEl);

  if (item.description) {
    const descEl = document.createElement('p');
    descEl.className = 'text-xs text-ink-dim mt-1';
    descEl.textContent = item.description;
    info.appendChild(descEl);
  }

  li.appendChild(info);

  li.appendChild(
    makeActionsMenu([
      { label: 'Edit', onClick: () => populateFormForEdit(item) },
      {
        label: 'Delete',
        danger: true,
        onClick: async () => {
          deleteErrEl.classList.add('hidden');
          const res = await del('/api/bookmarks/' + item.id);
          if (res.ok) {
            if (editingId === item.id) resetBookmarkFormToCreateMode();
            await loadList();
          } else {
            deleteErrEl.textContent = res.error ?? 'Failed to delete.';
            deleteErrEl.classList.remove('hidden');
          }
        },
      },
    ])
  );
  return li;
}

function populateFormForEdit(item) {
  editingId = item.id;
  bmFormTitleEl.textContent = 'Edit Bookmark';
  bmSubmitBtn.textContent = 'Save Changes';
  bmCancelBtn.classList.remove('hidden');
  bmCategorySelect.value = String(item.category_id);
  bmTitleInput.value = item.title;
  bmUrlInput.value = item.url;
  bmDescriptionInput.value = item.description ?? '';
  window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
}

function resetBookmarkFormToCreateMode() {
  editingId = null;
  bmFormTitleEl.textContent = 'New Bookmark';
  bmSubmitBtn.textContent = 'Add Bookmark';
  bmCancelBtn.classList.add('hidden');
  bmTitleInput.value = '';
  bmUrlInput.value = '';
  bmDescriptionInput.value = '';
}

setupBookmarkCategoriesCreateForm(document.getElementById('forms-container'));
setupBookmarksCreateForm(document.getElementById('forms-container'));
// @inject-forms

async function init() {
  await loadList();
}

init();

function setupBookmarkCategoriesCreateForm(container) {
  const { details, body } = makeCollapsibleSection(
    'New Category',
    'border border-hairline bg-surface p-5 space-y-3 mt-4'
  );

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  const titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  submitBtn.textContent = 'Add Category';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const data = { title: titleInput.value };
    const res = await post('/api/bookmark_categories_create', data);
    submitBtn.disabled = false;
    if (res.ok) {
      form.reset();
      await loadList();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  container.appendChild(details);
}

function setupBookmarksCreateForm(container) {
  const { details, body, labelEl } = makeCollapsibleSection(
    'New Bookmark',
    'border border-hairline bg-surface p-5 space-y-3 mt-4'
  );
  bmFormTitleEl = labelEl;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const categoryLabel = document.createElement('label');
  categoryLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  categoryLabel.textContent = 'Category';
  bmCategorySelect = document.createElement('select');
  bmCategorySelect.name = 'category_id';
  bmCategorySelect.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  bmCategorySelect.required = true;
  form.appendChild(categoryLabel);
  form.appendChild(bmCategorySelect);

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  bmTitleInput = document.createElement('input');
  bmTitleInput.type = 'text';
  bmTitleInput.name = 'title';
  bmTitleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  bmTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(bmTitleInput);

  const urlLabel = document.createElement('label');
  urlLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  urlLabel.textContent = 'Url';
  bmUrlInput = document.createElement('input');
  bmUrlInput.type = 'text';
  bmUrlInput.name = 'url';
  bmUrlInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  bmUrlInput.required = true;
  form.appendChild(urlLabel);
  form.appendChild(bmUrlInput);

  const descriptionLabel = document.createElement('label');
  descriptionLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  descriptionLabel.textContent = 'Description';
  bmDescriptionInput = document.createElement('input');
  bmDescriptionInput.type = 'text';
  bmDescriptionInput.name = 'description';
  bmDescriptionInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(descriptionLabel);
  form.appendChild(bmDescriptionInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  bmSubmitBtn = document.createElement('button');
  bmSubmitBtn.type = 'submit';
  bmSubmitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  bmSubmitBtn.textContent = 'Add Bookmark';
  btnRow.appendChild(bmSubmitBtn);

  bmCancelBtn = document.createElement('button');
  bmCancelBtn.type = 'button';
  bmCancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors hidden';
  bmCancelBtn.textContent = 'Cancel';
  bmCancelBtn.addEventListener('click', resetBookmarkFormToCreateMode);
  btnRow.appendChild(bmCancelBtn);

  form.appendChild(btnRow);

  bmErrEl = document.createElement('p');
  bmErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(bmErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    bmSubmitBtn.disabled = true;
    bmErrEl.classList.add('hidden');

    const data = {
      category_id: parseInt(bmCategorySelect.value, 10),
      title: bmTitleInput.value,
      url: bmUrlInput.value,
      description: bmDescriptionInput.value,
    };

    const res = editingId
      ? await put('/api/bookmarks/' + editingId, data)
      : await post('/api/bookmarks_create', data);

    bmSubmitBtn.disabled = false;
    if (res.ok) {
      resetBookmarkFormToCreateMode();
      await loadList();
    } else {
      bmErrEl.textContent = res.error ?? 'Something went wrong.';
      bmErrEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  container.appendChild(details);

  populateCategorySelect();
}

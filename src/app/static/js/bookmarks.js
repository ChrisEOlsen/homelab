import { get, post, put, del } from '/static/js/lib/api.js';
import { registerServiceWorker } from '/static/js/lib/push.js';
registerServiceWorker();
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

// Module state
let categories = [];
let bookmarks = [];
let selectedCategoryId = 'all';

// ---- Category modal ----
let editingCategoryId = null;
const categoryModal = createModal('category-modal-title');
let catFormTitleEl, catTitleInput, catSubmitBtn, catErrEl;

function buildCategoryModal() {
  const heading = document.createElement('h3');
  heading.id = 'category-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'New Category';
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
    const title = catTitleInput.value;

    const res = editingCategoryId
      ? await put('/api/bookmark_categories/' + editingCategoryId, { title })
      : await post('/api/bookmark_categories_create', { title });

    catSubmitBtn.disabled = false;
    if (res.ok) {
      categoryModal.close();
      await loadList();
    } else {
      catErrEl.textContent = res.error ?? 'Something went wrong.';
      catErrEl.classList.remove('hidden');
    }
  });

  categoryModal.panel.appendChild(form);
}

function openCategoryModalForCreate(trigger) {
  editingCategoryId = null;
  catFormTitleEl.textContent = 'New Category';
  catSubmitBtn.textContent = 'Add Category';
  catTitleInput.value = '';
  catErrEl.classList.add('hidden');
  categoryModal.open(trigger);
}

function openCategoryModalForEdit(cat) {
  editingCategoryId = cat.id;
  catFormTitleEl.textContent = 'Edit Category';
  catSubmitBtn.textContent = 'Save Changes';
  catTitleInput.value = cat.title;
  catErrEl.classList.add('hidden');
  categoryModal.open();
}

// ---- Bookmark modal ----
let editingId = null;
const bookmarkModal = createModal('bookmark-modal-title');
let bmFormTitleEl, bmSubmitBtn, bmCategorySelect, bmTitleInput, bmUrlInput, bmDescriptionInput, bmErrEl;

function buildBookmarkModal() {
  const heading = document.createElement('h3');
  heading.id = 'bookmark-modal-title';
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = 'New Bookmark';
  bookmarkModal.panel.appendChild(heading);
  bmFormTitleEl = heading;

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
  btnRow.className = 'flex items-center justify-end gap-2';

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = 'Cancel';
  cancelBtn.addEventListener('click', () => bookmarkModal.close());
  btnRow.appendChild(cancelBtn);

  bmSubmitBtn = document.createElement('button');
  bmSubmitBtn.type = 'submit';
  bmSubmitBtn.className =
    'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  bmSubmitBtn.textContent = 'Add Bookmark';
  btnRow.appendChild(bmSubmitBtn);

  form.appendChild(btnRow);

  bmErrEl = document.createElement('p');
  bmErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(bmErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!bmCategorySelect.value) {
      bmErrEl.textContent = 'Create a category first.';
      bmErrEl.classList.remove('hidden');
      return;
    }
    bmSubmitBtn.disabled = true;
    bmErrEl.classList.add('hidden');

    const data = {
      category_id: parseInt(bmCategorySelect.value, 10),
      title: bmTitleInput.value,
      url: bmUrlInput.value,
      description: bmDescriptionInput.value,
    };

    const res = editingId ? await put('/api/bookmarks/' + editingId, data) : await post('/api/bookmarks_create', data);

    bmSubmitBtn.disabled = false;
    if (res.ok) {
      bookmarkModal.close();
      await loadList();
    } else {
      bmErrEl.textContent = res.error ?? 'Something went wrong.';
      bmErrEl.classList.remove('hidden');
    }
  });

  bookmarkModal.panel.appendChild(form);
}

function populateCategorySelect() {
  bmCategorySelect.replaceChildren();
  categories.forEach((cat) => {
    const opt = document.createElement('option');
    opt.value = String(cat.id);
    opt.textContent = cat.title;
    bmCategorySelect.appendChild(opt);
  });
}

function openBookmarkModalForCreate(trigger) {
  editingId = null;
  bmFormTitleEl.textContent = 'New Bookmark';
  bmSubmitBtn.textContent = 'Add Bookmark';
  populateCategorySelect();
  // Always default to whichever category is currently selected in the
  // sidebar, never a stale value left over from a previous add.
  const desired = selectedCategoryId !== 'all' ? String(selectedCategoryId) : categories[0] ? String(categories[0].id) : '';
  if (desired) bmCategorySelect.value = desired;
  bmTitleInput.value = '';
  bmUrlInput.value = '';
  bmDescriptionInput.value = '';
  bmErrEl.classList.add('hidden');
  bookmarkModal.open(trigger);
}

function openBookmarkModalForEdit(item) {
  editingId = item.id;
  bmFormTitleEl.textContent = 'Edit Bookmark';
  bmSubmitBtn.textContent = 'Save Changes';
  populateCategorySelect();
  bmCategorySelect.value = String(item.category_id);
  bmTitleInput.value = item.title;
  bmUrlInput.value = item.url;
  bmDescriptionInput.value = item.description ?? '';
  bmErrEl.classList.add('hidden');
  bookmarkModal.open();
}

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
  if (selectedCategoryId !== 'all' && !categories.some((c) => c.id === selectedCategoryId)) {
    selectedCategoryId = 'all';
  }
  render();
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

  const ul = document.createElement('ul');
  ul.className = 'space-y-1';

  const allLi = document.createElement('li');
  const allBtn = document.createElement('button');
  allBtn.type = 'button';
  allBtn.className =
    'w-full text-left px-3 py-2 text-sm border transition-colors ' +
    (selectedCategoryId === 'all'
      ? 'bg-accent text-canvas border-accent'
      : 'bg-surface text-ink-dim border-hairline hover:bg-surface-raised hover:text-ink');
  allBtn.textContent = 'All';
  allBtn.addEventListener('click', () => {
    selectedCategoryId = 'all';
    render();
  });
  allLi.appendChild(allBtn);
  ul.appendChild(allLi);

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
    titleSpan.addEventListener('click', () => {
      selectedCategoryId = cat.id;
      render();
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
              if (!window.confirm('Delete this category and all its bookmarks?')) return;
              deleteErrEl.classList.add('hidden');
              const res = await del('/api/bookmark_categories/' + cat.id);
              if (res.ok) {
                if (selectedCategoryId === cat.id) selectedCategoryId = 'all';
                await loadList();
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

  const header = document.createElement('div');
  header.className = 'flex items-center justify-between gap-4';

  const heading = document.createElement('h2');
  heading.className = 'text-lg font-semibold text-ink';
  heading.textContent = selectedCategoryId === 'all' ? 'All Bookmarks' : categories.find((c) => c.id === selectedCategoryId)?.title ?? '';
  header.appendChild(heading);

  const addBookmarkBtn = document.createElement('button');
  addBookmarkBtn.type = 'button';
  addBookmarkBtn.className =
    'px-3 py-1.5 text-xs border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors';
  addBookmarkBtn.textContent = '+ Add Bookmark';
  addBookmarkBtn.disabled = categories.length === 0;
  addBookmarkBtn.addEventListener('click', (e) => openBookmarkModalForCreate(e.currentTarget));
  header.appendChild(addBookmarkBtn);

  section.appendChild(header);

  const filtered =
    selectedCategoryId === 'all' ? bookmarks : bookmarks.filter((item) => item.category_id === selectedCategoryId);

  if (filtered.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = categories.length === 0 ? 'Create a category to get started.' : 'No bookmarks yet.';
    section.appendChild(p);
    return section;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-2';
  filtered.forEach((item) => {
    ul.appendChild(renderBookmarkRow(item));
  });
  section.appendChild(ul);

  return section;
}

function renderBookmarkRow(item) {
  const li = document.createElement('li');
  li.className = 'border border-hairline bg-surface-raised p-4 flex items-start justify-between gap-4';

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
      { label: 'Edit', onClick: () => openBookmarkModalForEdit(item) },
      {
        label: 'Delete',
        danger: true,
        onClick: async () => {
          deleteErrEl.classList.add('hidden');
          const res = await del('/api/bookmarks/' + item.id);
          if (res.ok) {
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

buildCategoryModal();
buildBookmarkModal();

async function init() {
  await loadList();
}

init();

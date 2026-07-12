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
// `toggleClassName` lets a caller override the toggle button's look (e.g.
// the todo-lists sidebar needs different contrast on its active/accent row).
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

// Delete-error element: created once, inserted as a sibling of #app so it
// survives render()'s replaceChildren() re-renders (used by every delete
// action on this page, including ones nested in the detail panel).
const deleteErrEl = document.createElement('p');
deleteErrEl.className = 'text-sm text-danger mt-2 hidden';
app.insertAdjacentElement('afterend', deleteErrEl);

// Module state
let lists = [];
let todos = [];
let activeListId = null;

// List form state (shared between create and rename modes)
let editingListId = null;
let editingListSortOrder = 0;
let listFormTitleEl, listTitleInput, listSubmitBtn, listCancelBtn, listErrEl;

// Todo form state (shared between create and edit modes)
let editingTodoId = null;
let editingTodoListId = null;
let editingTodoIsDone = false;
let editingTodoSortOrder = 0;
let todoFormTitleEl, todoListSelect, todoTitleInput, todoDescriptionInput, todoSubmitBtn, todoCancelBtn, todoErrEl;

// Inline subtasks state. The bulk /api/todos response now includes every
// subtask up front (grouped here by todo_id), so every todo's subtasks are
// already in memory on page load — needed so mobile can show them all by
// default with no per-item fetch. `expandedTodoIds` still tracks the
// desktop show/hide toggle (several todos can be expanded independently);
// on mobile the toggle button is hidden and subtasks always render
// regardless of this set (see the `md:hidden` class logic below).
let expandedTodoIds = new Set();
let subtasksByTodoId = new Map();

async function loadList() {
  const res = await get('/api/todos');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-danger';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  lists = res.data?.lists ?? [];
  todos = res.data?.todos ?? [];

  subtasksByTodoId = new Map();
  (res.data?.subtasks ?? []).forEach((sub) => {
    if (!subtasksByTodoId.has(sub.todo_id)) subtasksByTodoId.set(sub.todo_id, []);
    subtasksByTodoId.get(sub.todo_id).push(sub);
  });
  // The bulk endpoint orders subtasks newest-first overall; sort each
  // todo's own group back to oldest-first, matching prior per-todo order.
  subtasksByTodoId.forEach((subs) => subs.sort((a, b) => new Date(a.created_at) - new Date(b.created_at)));

  if (activeListId === null || !lists.some((l) => l.id === activeListId)) {
    activeListId = lists.length ? lists[0].id : null;
  }

  renderTodoListSelectOptions();
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
    'Lists',
    'w-full sm:w-56 shrink-0 border border-hairline bg-surface p-4 space-y-2'
  );

  if (lists.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No lists yet.';
    body.appendChild(p);
    return details;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-1';

  lists.forEach((list) => {
    const li = document.createElement('li');
    li.className =
      'group flex items-center justify-between gap-2 px-3 py-2 text-sm cursor-pointer border ' +
      (list.id === activeListId
        ? 'bg-accent text-canvas border-accent'
        : 'bg-surface text-ink-dim border-hairline hover:bg-surface-raised hover:text-ink');

    const titleSpan = document.createElement('span');
    titleSpan.className = 'flex-1 truncate';
    titleSpan.textContent = list.title;
    titleSpan.addEventListener('click', () => {
      activeListId = list.id;
      render();
    });
    li.appendChild(titleSpan);

    const toggleClass =
      'text-xs px-1.5 py-0.5 leading-none ' +
      (list.id === activeListId ? 'text-canvas/70 hover:text-canvas' : 'text-ink-dim hover:text-ink');

    li.appendChild(
      makeActionsMenu(
        [
          { label: 'Edit', onClick: () => populateListFormForEdit(list) },
          {
            label: 'Delete',
            danger: true,
            onClick: async () => {
              if (!window.confirm('Delete this list and all its todos?')) return;
              deleteErrEl.classList.add('hidden');
              const res = await del('/api/todo_lists/' + list.id);
              if (res.ok) {
                if (editingListId === list.id) resetListFormToCreateMode();
                if (activeListId === list.id) activeListId = null;
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
  return details;
}

function renderMain() {
  const section = document.createElement('section');
  section.className = 'flex-1 space-y-4 min-w-0';

  if (activeListId === null) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'Create a list to get started.';
    section.appendChild(p);
    return section;
  }

  const activeList = lists.find((l) => l.id === activeListId);
  const listTodos = todos.filter((t) => t.list_id === activeListId);

  const header = document.createElement('div');
  header.className = 'flex items-center justify-between gap-4';

  const heading = document.createElement('h2');
  heading.className = 'text-lg font-semibold text-ink';
  heading.textContent = activeList ? activeList.title : '';
  header.appendChild(heading);

  const clearBtn = document.createElement('button');
  clearBtn.type = 'button';
  clearBtn.className =
    'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  clearBtn.textContent = 'Clear Completed';
  clearBtn.addEventListener('click', async () => {
    if (!window.confirm('Delete all completed todos in this list?')) return;
    clearBtn.disabled = true;
    await post('/api/todo_lists/' + activeListId + '/clear_completed');
    await loadList();
  });
  header.appendChild(clearBtn);

  section.appendChild(header);

  if (listTodos.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No todos in this list yet.';
    section.appendChild(p);
    return section;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-2';

  listTodos.forEach((item) => {
    const li = document.createElement('li');
    li.draggable = true;
    li.dataset.id = String(item.id);
    li.className =
      'border border-hairline bg-surface-raised p-3 flex flex-col gap-3' +
      (item.is_done ? ' opacity-60' : '');

    const topRow = document.createElement('div');
    topRow.className = 'flex items-center gap-3';

    const dragHandle = document.createElement('span');
    dragHandle.className = 'text-ink-dim cursor-grab select-none shrink-0';
    dragHandle.textContent = '⠿';
    topRow.appendChild(dragHandle);

    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = item.is_done;
    checkbox.className = 'shrink-0';
    checkbox.addEventListener('change', async () => {
      checkbox.disabled = true;
      await post('/api/todos/' + item.id + '/toggle');
      await loadList();
    });
    topRow.appendChild(checkbox);

    const info = document.createElement('div');
    info.className = 'flex-1 min-w-0';
    const titleEl = document.createElement('p');
    titleEl.className = 'text-sm font-medium text-ink truncate' + (item.is_done ? ' line-through' : '');
    titleEl.textContent = item.title;
    info.appendChild(titleEl);
    if (item.description) {
      const descEl = document.createElement('p');
      descEl.className = 'text-xs text-ink-dim truncate';
      descEl.textContent = item.description;
      info.appendChild(descEl);
    }
    topRow.appendChild(info);

    const actions = document.createElement('div');
    actions.className = 'flex items-center gap-2 shrink-0';

    // Hidden on mobile: subtasks always render there (see subWrap below),
    // so a toggle with no visible effect at that width would just confuse.
    const subtasksBtn = document.createElement('button');
    subtasksBtn.type = 'button';
    subtasksBtn.className =
      'hidden md:inline-block px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
    subtasksBtn.textContent = expandedTodoIds.has(item.id)
      ? 'Hide Subtasks'
      : item.subtask_count > 0
        ? `Subtasks (${item.subtask_count})`
        : 'Subtasks';
    subtasksBtn.addEventListener('click', () => {
      if (expandedTodoIds.has(item.id)) {
        expandedTodoIds.delete(item.id);
      } else {
        expandedTodoIds.add(item.id);
      }
      render();
    });
    actions.appendChild(subtasksBtn);

    // The inline add-subtask form is desktop-only (see renderInlineSubtasks)
    // — on mobile, adding a subtask happens through this menu instead, since
    // there's no room for both an always-open form and a real mobile layout.
    const todoActions = [];
    if (window.matchMedia('(max-width: 767px)').matches) {
      todoActions.push({
        label: 'Add Subtask',
        onClick: async () => {
          const title = window.prompt('New subtask title:');
          if (!title || !title.trim()) return;
          deleteErrEl.classList.add('hidden');
          const res = await post('/api/subtasks_create', { todo_id: item.id, title });
          if (res.ok) {
            // subWrap is already in the DOM (see below) — no full render()
            // needed, just refresh this one todo's scoped subtasks block.
            expandedTodoIds.add(item.id);
            await loadSubtasks(item.id);
          } else {
            deleteErrEl.textContent = res.error ?? 'Failed to add subtask.';
            deleteErrEl.classList.remove('hidden');
          }
        },
      });
    }
    todoActions.push(
      { label: 'Edit', onClick: () => populateTodoFormForEdit(item) },
      {
        label: 'Delete',
        danger: true,
        onClick: async () => {
          deleteErrEl.classList.add('hidden');
          const res = await del('/api/todos/' + item.id);
          if (res.ok) {
            if (editingTodoId === item.id) resetTodoFormToCreateMode();
            expandedTodoIds.delete(item.id);
            subtasksByTodoId.delete(item.id);
            await loadList();
          } else {
            deleteErrEl.textContent = res.error ?? 'Failed to delete.';
            deleteErrEl.classList.remove('hidden');
          }
        },
      }
    );
    actions.appendChild(makeActionsMenu(todoActions));

    topRow.appendChild(actions);
    li.appendChild(topRow);

    // Always in the DOM (data's already loaded — see loadList) so mobile
    // can show it unconditionally. On mobile it's always visible; on
    // desktop `md:hidden` hides it unless this todo is expanded.
    const subWrap = document.createElement('div');
    subWrap.id = 'subtasks-inline-' + item.id;
    subWrap.className =
      'pl-8 border-t border-hairline pt-3' + (expandedTodoIds.has(item.id) ? '' : ' md:hidden');
    subWrap.appendChild(renderInlineSubtasks(item.id));
    li.appendChild(subWrap);

    li.addEventListener('dragstart', (e) => {
      e.dataTransfer.setData('text/plain', String(item.id));
      e.dataTransfer.effectAllowed = 'move';
      li.classList.add('opacity-50');
    });
    li.addEventListener('dragend', () => li.classList.remove('opacity-50'));
    li.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
    });
    li.addEventListener('drop', async (e) => {
      e.preventDefault();
      const draggedId = Number(e.dataTransfer.getData('text/plain'));
      if (!draggedId || draggedId === item.id) return;
      const draggedEl = ul.querySelector(`[data-id="${draggedId}"]`);
      if (!draggedEl) return;
      const rect = li.getBoundingClientRect();
      const insertBefore = e.clientY - rect.top < rect.height / 2;
      ul.insertBefore(draggedEl, insertBefore ? li : li.nextSibling);
      const order = Array.from(ul.children).map((el) => Number(el.dataset.id));
      await put('/api/todos_reorder', { order });
      await loadList();
    });

    ul.appendChild(li);
  });

  section.appendChild(ul);

  return section;
}

// Refetches one todo's subtasks after a create/toggle/delete and refreshes
// just that todo's inline block. Not used for the initial render — the
// bulk /api/todos response already has every todo's subtasks (see loadList).
async function loadSubtasks(todoId) {
  const res = await get('/api/todos/' + todoId + '/details');
  subtasksByTodoId.set(todoId, res.ok ? (res.data.subtasks ?? []) : []);
  refreshSubtasksInline(todoId);
}

// Refreshes only one todo's inline subtasks block in place, without
// touching the sidebar, the todo list, or any other todo's expand area or
// in-progress form — mirrors the scoped-refresh pattern used elsewhere on
// this page so an unrelated action never destroys focused input elsewhere.
function refreshSubtasksInline(todoId) {
  const container = document.getElementById('subtasks-inline-' + todoId);
  if (!container) return;
  container.replaceChildren(renderInlineSubtasks(todoId));
}

function renderInlineSubtasks(todoId) {
  // No "loading" state needed — every todo's subtasks arrive with the
  // initial bulk load (loadList); a todo with none simply isn't in the map.
  const subtasks = subtasksByTodoId.get(todoId) ?? [];

  const wrap = document.createElement('div');
  wrap.className = 'space-y-2';

  if (subtasks.length > 0) {
    const list = document.createElement('ul');
    list.className = 'space-y-1';

    subtasks.forEach((sub) => {
      const li = document.createElement('li');
      li.className = 'flex items-center gap-2 bg-surface-raised border border-hairline px-2 py-1.5';

      const checkbox = document.createElement('input');
      checkbox.type = 'checkbox';
      checkbox.checked = sub.is_done;
      checkbox.className = 'shrink-0';
      checkbox.addEventListener('change', async () => {
        checkbox.disabled = true;
        await post('/api/subtasks/' + sub.id + '/toggle');
        await loadSubtasks(todoId);
      });
      li.appendChild(checkbox);

      const title = document.createElement('span');
      title.className = 'flex-1 text-sm text-ink' + (sub.is_done ? ' line-through text-ink-dim' : '');
      title.textContent = sub.title;
      li.appendChild(title);

      li.appendChild(
        makeActionsMenu([
          {
            label: 'Edit',
            onClick: async () => {
              const newTitle = window.prompt('Edit subtask', sub.title);
              if (newTitle === null || !newTitle.trim() || newTitle === sub.title) return;
              deleteErrEl.classList.add('hidden');
              const res = await put('/api/subtasks/' + sub.id, { title: newTitle });
              if (res.ok) {
                await loadSubtasks(todoId);
              } else {
                deleteErrEl.textContent = res.error ?? 'Failed to save.';
                deleteErrEl.classList.remove('hidden');
              }
            },
          },
          {
            label: 'Delete',
            danger: true,
            onClick: async () => {
              deleteErrEl.classList.add('hidden');
              const res = await del('/api/subtasks/' + sub.id);
              if (res.ok) {
                await loadSubtasks(todoId);
              } else {
                deleteErrEl.textContent = res.error ?? 'Failed to delete.';
                deleteErrEl.classList.remove('hidden');
              }
            },
          },
        ])
      );

      list.appendChild(li);
    });

    wrap.appendChild(list);
  } else {
    // Hidden on mobile: adding the first subtask there happens via the
    // todo's own "⋯" menu ("Add Subtask") instead of this inline form —
    // see the mobile-only branch in renderMain's actions menu below.
    const p = document.createElement('p');
    p.className = 'hidden md:block text-sm text-ink-dim';
    p.textContent = 'No subtasks yet.';
    wrap.appendChild(p);
  }

  // Hidden on mobile for the same reason — desktop keeps this inline form.
  const form = document.createElement('form');
  form.className = 'hidden md:flex items-center gap-2';

  const input = document.createElement('input');
  input.type = 'text';
  input.placeholder = 'New subtask';
  input.className =
    'flex-1 bg-canvas border border-hairline px-2 py-1 text-sm text-ink focus:outline-none focus:border-accent';
  form.appendChild(input);

  const addBtn = document.createElement('button');
  addBtn.type = 'submit';
  addBtn.className = 'px-3 py-1 text-xs border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors';
  addBtn.textContent = 'Add';
  form.appendChild(addBtn);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!input.value.trim()) return;
    addBtn.disabled = true;
    await post('/api/subtasks_create', { todo_id: todoId, title: input.value });
    input.value = '';
    addBtn.disabled = false;
    await loadSubtasks(todoId);
  });

  wrap.appendChild(form);
  return wrap;
}

function renderTodoListSelectOptions() {
  if (!todoListSelect) return;
  const previousValue = todoListSelect.value;
  todoListSelect.replaceChildren();
  lists.forEach((list) => {
    const opt = document.createElement('option');
    opt.value = String(list.id);
    opt.textContent = list.title;
    todoListSelect.appendChild(opt);
  });
  if (editingTodoId !== null) {
    // Preserve the list of the todo currently being edited across reloads
    // triggered by unrelated actions (toggling, deleting, reordering, etc.)
    // elsewhere on the page — otherwise the select silently resets to its
    // first option and a subsequent Save would move the todo to the wrong list.
    todoListSelect.value = String(editingTodoListId);
    return;
  }
  const desired = previousValue || (activeListId !== null ? String(activeListId) : '');
  if (desired && lists.some((l) => String(l.id) === desired)) {
    todoListSelect.value = desired;
  }
}

function populateTodoFormForEdit(item) {
  editingTodoId = item.id;
  editingTodoListId = item.list_id;
  editingTodoIsDone = item.is_done;
  editingTodoSortOrder = item.sort_order;
  todoFormTitleEl.textContent = 'Edit Todo';
  todoSubmitBtn.textContent = 'Save Changes';
  todoCancelBtn.classList.remove('hidden');

  todoListSelect.value = String(item.list_id);
  todoTitleInput.value = item.title;
  todoDescriptionInput.value = item.description ?? '';
  window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
}

function resetTodoFormToCreateMode() {
  editingTodoId = null;
  editingTodoListId = null;
  editingTodoIsDone = false;
  editingTodoSortOrder = 0;
  todoFormTitleEl.textContent = 'New Todo';
  todoSubmitBtn.textContent = 'Add Todo';
  todoCancelBtn.classList.add('hidden');
  todoTitleInput.value = '';
  todoDescriptionInput.value = '';
  renderTodoListSelectOptions();
}

function populateListFormForEdit(list) {
  editingListId = list.id;
  editingListSortOrder = list.sort_order;
  listFormTitleEl.textContent = 'Edit List';
  listSubmitBtn.textContent = 'Save Changes';
  listCancelBtn.classList.remove('hidden');
  listTitleInput.value = list.title;
  window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' });
}

function resetListFormToCreateMode() {
  editingListId = null;
  editingListSortOrder = 0;
  listFormTitleEl.textContent = 'New List';
  listSubmitBtn.textContent = 'Add List';
  listCancelBtn.classList.add('hidden');
  listTitleInput.value = '';
}

setupTodoListsCreateForm(document.getElementById('forms-container'));
setupTodosCreateForm(document.getElementById('forms-container'));
// @inject-forms

async function init() {
  await loadList();
}

init();

function setupTodoListsCreateForm(container) {
  const { details, body, labelEl } = makeCollapsibleSection(
    'New List',
    'border border-hairline bg-surface p-5 space-y-3 mt-4'
  );
  listFormTitleEl = labelEl;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  listTitleInput = document.createElement('input');
  listTitleInput.type = 'text';
  listTitleInput.name = 'title';
  listTitleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  listTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(listTitleInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  listSubmitBtn = document.createElement('button');
  listSubmitBtn.type = 'submit';
  listSubmitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  listSubmitBtn.textContent = 'Add List';
  btnRow.appendChild(listSubmitBtn);

  listCancelBtn = document.createElement('button');
  listCancelBtn.type = 'button';
  listCancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors hidden';
  listCancelBtn.textContent = 'Cancel';
  listCancelBtn.addEventListener('click', resetListFormToCreateMode);
  btnRow.appendChild(listCancelBtn);

  form.appendChild(btnRow);

  listErrEl = document.createElement('p');
  listErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(listErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    listSubmitBtn.disabled = true;
    listErrEl.classList.add('hidden');
    const data = { title: listTitleInput.value, sort_order: editingListSortOrder };

    const res = editingListId
      ? await put('/api/todo_lists/' + editingListId, data)
      : await post('/api/todo_lists_create', { title: data.title });

    listSubmitBtn.disabled = false;
    if (res.ok) {
      resetListFormToCreateMode();
      await loadList();
    } else {
      listErrEl.textContent = res.error ?? 'Something went wrong.';
      listErrEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  container.appendChild(details);
}

function setupTodosCreateForm(container) {
  const { details, body, labelEl } = makeCollapsibleSection(
    'New Todo',
    'border border-hairline bg-surface p-5 space-y-3 mt-4'
  );
  todoFormTitleEl = labelEl;

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const listLabel = document.createElement('label');
  listLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  listLabel.textContent = 'List';
  todoListSelect = document.createElement('select');
  todoListSelect.name = 'list_id';
  todoListSelect.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  todoListSelect.required = true;
  form.appendChild(listLabel);
  form.appendChild(todoListSelect);

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  titleLabel.textContent = 'Title';
  todoTitleInput = document.createElement('input');
  todoTitleInput.type = 'text';
  todoTitleInput.name = 'title';
  todoTitleInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  todoTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(todoTitleInput);

  const descriptionLabel = document.createElement('label');
  descriptionLabel.className = 'block text-xs uppercase tracking-wide text-ink-dim mb-1';
  descriptionLabel.textContent = 'Description';
  todoDescriptionInput = document.createElement('input');
  todoDescriptionInput.type = 'text';
  todoDescriptionInput.name = 'description';
  todoDescriptionInput.className =
    'mt-1 block w-full bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(descriptionLabel);
  form.appendChild(todoDescriptionInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  todoSubmitBtn = document.createElement('button');
  todoSubmitBtn.type = 'submit';
  todoSubmitBtn.className = 'px-4 py-2 border border-accent text-accent text-xs uppercase tracking-wide hover:bg-accent hover:text-canvas transition-colors';
  todoSubmitBtn.textContent = 'Add Todo';
  btnRow.appendChild(todoSubmitBtn);

  todoCancelBtn = document.createElement('button');
  todoCancelBtn.type = 'button';
  todoCancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs uppercase tracking-wide hover:text-ink hover:bg-surface-raised transition-colors hidden';
  todoCancelBtn.textContent = 'Cancel';
  todoCancelBtn.addEventListener('click', resetTodoFormToCreateMode);
  btnRow.appendChild(todoCancelBtn);

  form.appendChild(btnRow);

  todoErrEl = document.createElement('p');
  todoErrEl.className = 'text-sm text-danger mt-2 hidden';
  form.appendChild(todoErrEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!todoListSelect.value) {
      todoErrEl.textContent = 'Create a list first.';
      todoErrEl.classList.remove('hidden');
      return;
    }
    todoSubmitBtn.disabled = true;
    todoErrEl.classList.add('hidden');
    const listId = Number(todoListSelect.value);
    const title = todoTitleInput.value;
    const description = todoDescriptionInput.value;

    const res = editingTodoId
      ? await put('/api/todos/' + editingTodoId, {
          list_id: listId,
          title,
          description,
          is_done: editingTodoIsDone,
          sort_order: editingTodoSortOrder,
        })
      : await post('/api/todos_create', { list_id: listId, title, description });

    todoSubmitBtn.disabled = false;
    if (res.ok) {
      resetTodoFormToCreateMode();
      await loadList();
    } else {
      todoErrEl.textContent = res.error ?? 'Something went wrong.';
      todoErrEl.classList.remove('hidden');
    }
  });

  body.appendChild(form);
  container.appendChild(details);
}

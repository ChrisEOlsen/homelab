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

// Detail panel state (subtasks + todo blocks for one expanded todo)
let detailTodoId = null;
let detailData = null;

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
      detailTodoId = null;
      detailData = null;
      render();
    });
    li.appendChild(titleSpan);

    const actions = document.createElement('span');
    actions.className = 'flex items-center gap-1 shrink-0';

    const renameBtn = document.createElement('button');
    renameBtn.type = 'button';
    renameBtn.className =
      'text-xs px-1 ' +
      (list.id === activeListId ? 'text-canvas/70 hover:text-canvas' : 'text-ink-dim hover:text-ink');
    renameBtn.textContent = 'Edit';
    renameBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      populateListFormForEdit(list);
    });
    actions.appendChild(renameBtn);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'text-xs px-1 ' +
      (list.id === activeListId ? 'text-canvas/70 hover:text-canvas' : 'text-ink-dim hover:text-danger');
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      if (!window.confirm('Delete this list and all its todos?')) return;
      deleteBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/todo_lists/' + list.id);
      if (res.ok) {
        if (editingListId === list.id) resetListFormToCreateMode();
        if (activeListId === list.id) activeListId = null;
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
      'border border-hairline bg-surface-raised p-3 flex items-center gap-3' +
      (item.is_done ? ' opacity-60' : '');

    const checkbox = document.createElement('input');
    checkbox.type = 'checkbox';
    checkbox.checked = item.is_done;
    checkbox.className = 'shrink-0';
    checkbox.addEventListener('change', async () => {
      checkbox.disabled = true;
      await post('/api/todos/' + item.id + '/toggle');
      await loadList();
    });
    li.appendChild(checkbox);

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
    li.appendChild(info);

    const actions = document.createElement('div');
    actions.className = 'flex items-center gap-2 shrink-0';

    const detailsBtn = document.createElement('button');
    detailsBtn.type = 'button';
    detailsBtn.className =
      'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
    detailsBtn.textContent = detailTodoId === item.id ? 'Hide Details' : 'Details';
    detailsBtn.addEventListener('click', async () => {
      if (detailTodoId === item.id) {
        detailTodoId = null;
        detailData = null;
        render();
        return;
      }
      detailTodoId = item.id;
      detailData = null;
      render();
      await loadTodoDetails(item.id);
    });
    actions.appendChild(detailsBtn);

    const editBtn = document.createElement('button');
    editBtn.type = 'button';
    editBtn.className =
      'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
    editBtn.textContent = 'Edit';
    editBtn.addEventListener('click', () => populateTodoFormForEdit(item));
    actions.appendChild(editBtn);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'px-3 py-1.5 text-xs border border-danger text-danger hover:bg-danger/10 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async () => {
      deleteBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/todos/' + item.id);
      if (res.ok) {
        if (editingTodoId === item.id) resetTodoFormToCreateMode();
        if (detailTodoId === item.id) {
          detailTodoId = null;
          detailData = null;
        }
        await loadList();
      } else {
        deleteBtn.disabled = false;
        deleteErrEl.textContent = res.error ?? 'Failed to delete.';
        deleteErrEl.classList.remove('hidden');
      }
    });
    actions.appendChild(deleteBtn);

    li.appendChild(actions);

    const dragHandle = document.createElement('span');
    dragHandle.className = 'text-ink-dim cursor-grab select-none shrink-0';
    dragHandle.textContent = '⠿';
    li.insertBefore(dragHandle, li.firstChild);

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

  if (detailTodoId !== null && listTodos.some((t) => t.id === detailTodoId)) {
    const detailContainer = document.createElement('div');
    detailContainer.id = 'todo-detail-panel';
    detailContainer.appendChild(renderDetailPanel());
    section.appendChild(detailContainer);
  }

  return section;
}

async function loadTodoDetails(id) {
  const res = await get('/api/todos/' + id + '/details');
  if (detailTodoId !== id) return; // stale response from an already-closed/changed panel
  detailData = res.ok ? res.data : null;
  refreshDetailPanel();
}

// Refreshes only the detail panel's own subtree (subtasks + blocks) in place,
// without touching the sidebar or todo list. Used by actions scoped entirely
// to the detail panel (subtask/block create, toggle, update, delete) so an
// in-flight edit elsewhere on the page (e.g. another block's focused
// textarea, a list rename form) is never destroyed by an unrelated
// full-page rebuild. Falls back to a no-op if the panel isn't currently
// mounted (e.g. it was closed while a request was in flight).
function refreshDetailPanel() {
  const container = document.getElementById('todo-detail-panel');
  if (!container) return;
  container.replaceChildren(renderDetailPanel());
}

function renderDetailPanel() {
  const panel = document.createElement('div');
  panel.className = 'border border-hairline bg-surface-raised p-4 space-y-4';

  const headerRow = document.createElement('div');
  headerRow.className = 'flex items-center justify-between gap-4';

  const heading = document.createElement('h3');
  heading.className = 'text-xs tracking-widest text-ink-dim uppercase';
  heading.textContent = detailData ? 'Details: ' + detailData.todo.title : 'Loading details...';
  headerRow.appendChild(heading);

  const closeBtn = document.createElement('button');
  closeBtn.type = 'button';
  closeBtn.className = 'text-xs text-ink-dim hover:text-ink';
  closeBtn.textContent = 'Close';
  closeBtn.addEventListener('click', () => {
    detailTodoId = null;
    detailData = null;
    render();
  });
  headerRow.appendChild(closeBtn);

  panel.appendChild(headerRow);

  if (!detailData) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'Loading...';
    panel.appendChild(p);
    return panel;
  }

  const todoId = detailData.todo.id;
  panel.appendChild(renderSubtasksSection(todoId, detailData.subtasks ?? []));
  panel.appendChild(renderBlocksSection(todoId, detailData.blocks ?? []));

  return panel;
}

function renderSubtasksSection(todoId, subtasks) {
  const wrap = document.createElement('div');
  wrap.className = 'space-y-2';

  const heading = document.createElement('h4');
  heading.className = 'text-xs text-ink-dim uppercase tracking-wide';
  heading.textContent = 'Subtasks';
  wrap.appendChild(heading);

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
        await loadTodoDetails(todoId);
      });
      li.appendChild(checkbox);

      const title = document.createElement('span');
      title.className = 'flex-1 text-sm text-ink' + (sub.is_done ? ' line-through text-ink-dim' : '');
      title.textContent = sub.title;
      li.appendChild(title);

      const deleteBtn = document.createElement('button');
      deleteBtn.type = 'button';
      deleteBtn.className = 'text-xs text-ink-dim hover:text-danger shrink-0';
      deleteBtn.textContent = 'Delete';
      deleteBtn.addEventListener('click', async () => {
        deleteBtn.disabled = true;
        deleteErrEl.classList.add('hidden');
        const res = await del('/api/subtasks/' + sub.id);
        if (res.ok) {
          await loadTodoDetails(todoId);
        } else {
          deleteBtn.disabled = false;
          deleteErrEl.textContent = res.error ?? 'Failed to delete.';
          deleteErrEl.classList.remove('hidden');
        }
      });
      li.appendChild(deleteBtn);

      list.appendChild(li);
    });

    wrap.appendChild(list);
  } else {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No subtasks yet.';
    wrap.appendChild(p);
  }

  const form = document.createElement('form');
  form.className = 'flex items-center gap-2';

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
    await loadTodoDetails(todoId);
  });

  wrap.appendChild(form);
  return wrap;
}

function renderBlocksSection(todoId, blocks) {
  const wrap = document.createElement('div');
  wrap.className = 'space-y-2';

  const heading = document.createElement('h4');
  heading.className = 'text-xs text-ink-dim uppercase tracking-wide';
  heading.textContent = 'Sections';
  wrap.appendChild(heading);

  if (blocks.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = 'No sections yet.';
    wrap.appendChild(p);
  }

  blocks.forEach((block) => {
    const box = document.createElement('div');
    box.className = 'bg-surface-raised border border-hairline p-3 space-y-2';

    const rowTop = document.createElement('div');
    rowTop.className = 'flex items-center gap-2';

    const headerInput = document.createElement('input');
    headerInput.type = 'text';
    headerInput.value = block.header;
    headerInput.className =
      'flex-1 bg-canvas border border-hairline px-2 py-1 text-sm font-medium text-ink focus:outline-none focus:border-accent';
    rowTop.appendChild(headerInput);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className = 'text-xs text-ink-dim hover:text-danger shrink-0';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async () => {
      deleteBtn.disabled = true;
      deleteErrEl.classList.add('hidden');
      const res = await del('/api/todo_blocks/' + block.id);
      if (res.ok) {
        await loadTodoDetails(todoId);
      } else {
        deleteBtn.disabled = false;
        deleteErrEl.textContent = res.error ?? 'Failed to delete.';
        deleteErrEl.classList.remove('hidden');
      }
    });
    rowTop.appendChild(deleteBtn);

    box.appendChild(rowTop);

    const contentArea = document.createElement('textarea');
    contentArea.value = block.content;
    contentArea.rows = 3;
    contentArea.className =
      'block w-full bg-canvas border border-hairline px-2 py-1 text-sm text-ink focus:outline-none focus:border-accent';
    box.appendChild(contentArea);

    const saveErrEl = document.createElement('p');
    saveErrEl.className = 'text-xs text-danger hidden';
    box.appendChild(saveErrEl);

    // A block's own save is purely a field-level update: no block/subtask is
    // created or deleted and no ids change, so there is nothing structural to
    // reload. Re-fetching and rebuilding the whole blocks section here would
    // recreate this exact input/textarea from scratch, which — since save
    // fires on blur and runs async — could wipe out keystrokes the user has
    // already typed into this (or another) block by the time the response
    // comes back. Instead, on success we just keep the in-memory
    // detailData.blocks entry (this same `block` object) in sync with what's
    // already on screen and stop; on failure we show an inline error without
    // touching the field's value so the user's input is preserved.
    const saveBlock = async () => {
      const header = headerInput.value;
      const content = contentArea.value;
      const res = await put('/api/todo_blocks/' + block.id, {
        header,
        content,
        sort_order: block.sort_order,
      });
      if (res.ok) {
        saveErrEl.classList.add('hidden');
        block.header = header;
        block.content = content;
        return;
      }
      saveErrEl.textContent = res.error ?? 'Failed to save.';
      saveErrEl.classList.remove('hidden');
    };
    headerInput.addEventListener('blur', saveBlock);
    contentArea.addEventListener('blur', saveBlock);

    wrap.appendChild(box);
  });

  const addBtn = document.createElement('button');
  addBtn.type = 'button';
  addBtn.className =
    'px-3 py-1.5 text-xs border border-hairline text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors';
  addBtn.textContent = 'Add Section';
  addBtn.addEventListener('click', async () => {
    addBtn.disabled = true;
    await post('/api/todo_blocks_create', { todo_id: todoId });
    await loadTodoDetails(todoId);
  });
  wrap.appendChild(addBtn);

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

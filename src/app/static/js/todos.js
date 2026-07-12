import { get, post, put, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

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

async function loadList() {
  const res = await get('/api/todos');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-red-600';
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
  const aside = document.createElement('aside');
  aside.className = 'w-full sm:w-56 shrink-0 space-y-2';

  if (lists.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No lists yet.';
    aside.appendChild(p);
    return aside;
  }

  const ul = document.createElement('ul');
  ul.className = 'space-y-1';

  lists.forEach((list) => {
    const li = document.createElement('li');
    li.className =
      'group flex items-center justify-between gap-2 rounded px-3 py-2 text-sm cursor-pointer border ' +
      (list.id === activeListId
        ? 'bg-gray-900 text-white border-gray-900'
        : 'bg-white text-gray-700 border-gray-200 hover:bg-gray-50');

    const titleSpan = document.createElement('span');
    titleSpan.className = 'flex-1 truncate';
    titleSpan.textContent = list.title;
    titleSpan.addEventListener('click', () => {
      activeListId = list.id;
      render();
    });
    li.appendChild(titleSpan);

    const actions = document.createElement('span');
    actions.className = 'flex items-center gap-1 shrink-0';

    const renameBtn = document.createElement('button');
    renameBtn.type = 'button';
    renameBtn.className =
      'text-xs px-1 rounded ' +
      (list.id === activeListId ? 'text-gray-200 hover:text-white' : 'text-gray-400 hover:text-gray-700');
    renameBtn.textContent = 'Edit';
    renameBtn.addEventListener('click', (e) => {
      e.stopPropagation();
      populateListFormForEdit(list);
    });
    actions.appendChild(renameBtn);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'text-xs px-1 rounded ' +
      (list.id === activeListId ? 'text-gray-200 hover:text-white' : 'text-gray-400 hover:text-red-600');
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async (e) => {
      e.stopPropagation();
      if (!window.confirm('Delete this list and all its todos?')) return;
      await del('/api/todo_lists/' + list.id);
      if (editingListId === list.id) resetListFormToCreateMode();
      if (activeListId === list.id) activeListId = null;
      await loadList();
    });
    actions.appendChild(deleteBtn);

    li.appendChild(actions);
    ul.appendChild(li);
  });

  aside.appendChild(ul);
  return aside;
}

function renderMain() {
  const section = document.createElement('section');
  section.className = 'flex-1 space-y-4 min-w-0';

  if (activeListId === null) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'Create a list to get started.';
    section.appendChild(p);
    return section;
  }

  const activeList = lists.find((l) => l.id === activeListId);
  const listTodos = todos.filter((t) => t.list_id === activeListId);

  const header = document.createElement('div');
  header.className = 'flex items-center justify-between gap-4';

  const heading = document.createElement('h2');
  heading.className = 'text-lg font-semibold text-gray-900';
  heading.textContent = activeList ? activeList.title : '';
  header.appendChild(heading);

  const clearBtn = document.createElement('button');
  clearBtn.type = 'button';
  clearBtn.className =
    'px-3 py-1.5 text-xs rounded border border-gray-300 hover:bg-gray-50 transition-colors';
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
    p.className = 'text-sm text-gray-500';
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
      'border border-gray-200 rounded-lg p-3 bg-white flex items-center gap-3' +
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
    titleEl.className = 'text-sm font-medium text-gray-900 truncate' + (item.is_done ? ' line-through' : '');
    titleEl.textContent = item.title;
    info.appendChild(titleEl);
    if (item.description) {
      const descEl = document.createElement('p');
      descEl.className = 'text-xs text-gray-500 truncate';
      descEl.textContent = item.description;
      info.appendChild(descEl);
    }
    li.appendChild(info);

    const actions = document.createElement('div');
    actions.className = 'flex items-center gap-2 shrink-0';

    const editBtn = document.createElement('button');
    editBtn.type = 'button';
    editBtn.className =
      'px-3 py-1.5 text-xs rounded border border-gray-300 hover:bg-gray-50 transition-colors';
    editBtn.textContent = 'Edit';
    editBtn.addEventListener('click', () => populateTodoFormForEdit(item));
    actions.appendChild(editBtn);

    const deleteBtn = document.createElement('button');
    deleteBtn.type = 'button';
    deleteBtn.className =
      'px-3 py-1.5 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors';
    deleteBtn.textContent = 'Delete';
    deleteBtn.addEventListener('click', async () => {
      deleteBtn.disabled = true;
      await del('/api/todos/' + item.id);
      if (editingTodoId === item.id) resetTodoFormToCreateMode();
      await loadList();
    });
    actions.appendChild(deleteBtn);

    li.appendChild(actions);

    const dragHandle = document.createElement('span');
    dragHandle.className = 'text-gray-300 cursor-grab select-none shrink-0';
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
  return section;
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
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  listFormTitleEl = document.createElement('h3');
  listFormTitleEl.className = 'text-sm font-semibold text-gray-900';
  listFormTitleEl.textContent = 'New List';
  wrapper.appendChild(listFormTitleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  listTitleInput = document.createElement('input');
  listTitleInput.type = 'text';
  listTitleInput.name = 'title';
  listTitleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  listTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(listTitleInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  listSubmitBtn = document.createElement('button');
  listSubmitBtn.type = 'submit';
  listSubmitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  listSubmitBtn.textContent = 'Add List';
  btnRow.appendChild(listSubmitBtn);

  listCancelBtn = document.createElement('button');
  listCancelBtn.type = 'button';
  listCancelBtn.className =
    'px-4 py-2 text-sm rounded border border-gray-300 hover:bg-gray-50 transition-colors hidden';
  listCancelBtn.textContent = 'Cancel';
  listCancelBtn.addEventListener('click', resetListFormToCreateMode);
  btnRow.appendChild(listCancelBtn);

  form.appendChild(btnRow);

  listErrEl = document.createElement('p');
  listErrEl.className = 'text-sm text-red-600 hidden';
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

  wrapper.appendChild(form);
  container.appendChild(wrapper);
}

function setupTodosCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  todoFormTitleEl = document.createElement('h3');
  todoFormTitleEl.className = 'text-sm font-semibold text-gray-900';
  todoFormTitleEl.textContent = 'New Todo';
  wrapper.appendChild(todoFormTitleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const listLabel = document.createElement('label');
  listLabel.className = 'block text-sm font-medium text-gray-700';
  listLabel.textContent = 'List';
  todoListSelect = document.createElement('select');
  todoListSelect.name = 'list_id';
  todoListSelect.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  todoListSelect.required = true;
  form.appendChild(listLabel);
  form.appendChild(todoListSelect);

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  todoTitleInput = document.createElement('input');
  todoTitleInput.type = 'text';
  todoTitleInput.name = 'title';
  todoTitleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  todoTitleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(todoTitleInput);

  const descriptionLabel = document.createElement('label');
  descriptionLabel.className = 'block text-sm font-medium text-gray-700';
  descriptionLabel.textContent = 'Description';
  todoDescriptionInput = document.createElement('input');
  todoDescriptionInput.type = 'text';
  todoDescriptionInput.name = 'description';
  todoDescriptionInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  form.appendChild(descriptionLabel);
  form.appendChild(todoDescriptionInput);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  todoSubmitBtn = document.createElement('button');
  todoSubmitBtn.type = 'submit';
  todoSubmitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  todoSubmitBtn.textContent = 'Add Todo';
  btnRow.appendChild(todoSubmitBtn);

  todoCancelBtn = document.createElement('button');
  todoCancelBtn.type = 'button';
  todoCancelBtn.className =
    'px-4 py-2 text-sm rounded border border-gray-300 hover:bg-gray-50 transition-colors hidden';
  todoCancelBtn.textContent = 'Cancel';
  todoCancelBtn.addEventListener('click', resetTodoFormToCreateMode);
  btnRow.appendChild(todoCancelBtn);

  form.appendChild(btnRow);

  todoErrEl = document.createElement('p');
  todoErrEl.className = 'text-sm text-red-600 hidden';
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

  wrapper.appendChild(form);
  container.appendChild(wrapper);
}

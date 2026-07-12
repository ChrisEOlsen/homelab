import { get, post, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

let categories = [];
let goals = [];
let milestones = [];
let activeCategoryId = null;

// Goal form element refs
let vgCategorySelect;

// Milestone form element refs
let vmGoalSelect;

async function loadList() {
  const res = await get('/api/vision_board');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-red-600';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  categories = res.data?.categories ?? [];
  goals = res.data?.goals ?? [];
  milestones = res.data?.milestones ?? [];

  if (activeCategoryId === null || !categories.some((c) => c.id === activeCategoryId)) {
    activeCategoryId = categories.length ? categories[0].id : null;
  }

  populateCategorySelect();
  populateGoalSelect();
  render();
}

function goalsByCategory() {
  return goals.reduce((acc, goal) => {
    (acc[goal.category_id] = acc[goal.category_id] || []).push(goal);
    return acc;
  }, {});
}

function milestonesByGoal() {
  return milestones.reduce((acc, m) => {
    (acc[m.goal_id] = acc[m.goal_id] || []).push(m);
    return acc;
  }, {});
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

  if (categories.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No categories yet. Add one below to get started.';
    app.appendChild(p);
    return;
  }

  const tabsWrap = document.createElement('div');
  tabsWrap.className = 'flex flex-wrap gap-2';

  categories.forEach((cat) => {
    const tab = document.createElement('div');
    tab.className = 'flex items-center gap-1';

    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = tabClass(cat.id === activeCategoryId);
    btn.textContent = cat.title;
    btn.addEventListener('click', () => {
      activeCategoryId = cat.id;
      populateGoalSelect();
      render();
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
      await del('/api/vision_categories/' + cat.id);
      await loadList();
    });
    tab.appendChild(delBtn);

    tabsWrap.appendChild(tab);
  });

  app.appendChild(tabsWrap);

  const goalMap = goalsByCategory();
  const milestoneMap = milestonesByGoal();
  const activeGoals = goalMap[activeCategoryId] ?? [];

  const goalsWrap = document.createElement('div');
  goalsWrap.className = 'space-y-4 mt-4';

  if (activeGoals.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No goals in this category yet.';
    goalsWrap.appendChild(p);
  } else {
    activeGoals.forEach((goal) => {
      goalsWrap.appendChild(renderGoalCard(goal, milestoneMap[goal.id] ?? []));
    });
  }

  app.appendChild(goalsWrap);
}

function renderGoalCard(goal, goalMilestones) {
  const card = document.createElement('div');
  card.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3';

  const header = document.createElement('div');
  header.className = 'flex items-start justify-between gap-4';

  const titleWrap = document.createElement('div');
  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = goal.title;
  const yearEl = document.createElement('p');
  yearEl.className = 'text-xs text-gray-400';
  yearEl.textContent = 'Target: ' + goal.target_year;
  titleWrap.appendChild(titleEl);
  titleWrap.appendChild(yearEl);
  header.appendChild(titleWrap);

  const goalDelBtn = document.createElement('button');
  goalDelBtn.type = 'button';
  goalDelBtn.className =
    'px-3 py-1.5 text-xs rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors shrink-0';
  goalDelBtn.textContent = 'Delete';
  goalDelBtn.addEventListener('click', async () => {
    goalDelBtn.disabled = true;
    await del('/api/vision_goals/' + goal.id);
    await loadList();
  });
  header.appendChild(goalDelBtn);

  card.appendChild(header);

  const pct =
    goalMilestones.length === 0
      ? 0
      : Math.round((goalMilestones.filter((m) => m.is_done).length / goalMilestones.length) * 100);

  const barOuter = document.createElement('div');
  barOuter.className = 'w-full bg-gray-100 rounded-full h-2';
  const barInner = document.createElement('div');
  barInner.className = 'bg-gray-900 h-2 rounded-full';
  barInner.style.width = pct + '%';
  barOuter.appendChild(barInner);
  card.appendChild(barOuter);

  const pctLabel = document.createElement('p');
  pctLabel.className = 'text-xs text-gray-500';
  pctLabel.textContent = pct + '% complete';
  card.appendChild(pctLabel);

  if (goalMilestones.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No milestones yet.';
    card.appendChild(p);
  } else {
    const list = document.createElement('ul');
    list.className = 'space-y-1';
    goalMilestones.forEach((m) => {
      list.appendChild(renderMilestoneRow(m));
    });
    card.appendChild(list);
  }

  return card;
}

function renderMilestoneRow(m) {
  const li = document.createElement('li');
  li.className = 'flex items-center gap-2';

  const checkbox = document.createElement('input');
  checkbox.type = 'checkbox';
  checkbox.checked = !!m.is_done;
  checkbox.addEventListener('change', async () => {
    checkbox.disabled = true;
    await post('/api/vision_milestones/' + m.id + '/toggle');
    await loadList();
  });
  li.appendChild(checkbox);

  const label = document.createElement('span');
  label.className = m.is_done ? 'text-sm text-gray-400 line-through flex-1' : 'text-sm text-gray-900 flex-1';
  label.textContent = m.title;
  li.appendChild(label);

  const delBtn = document.createElement('button');
  delBtn.type = 'button';
  delBtn.className = 'text-xs text-red-400 hover:text-red-600 px-1';
  delBtn.title = 'Delete milestone';
  delBtn.textContent = '×';
  delBtn.addEventListener('click', async () => {
    delBtn.disabled = true;
    await del('/api/vision_milestones/' + m.id);
    await loadList();
  });
  li.appendChild(delBtn);

  return li;
}

function populateCategorySelect() {
  if (!vgCategorySelect) return;
  const previousValue = vgCategorySelect.value;
  vgCategorySelect.replaceChildren();
  categories.forEach((cat) => {
    const opt = document.createElement('option');
    opt.value = String(cat.id);
    opt.textContent = cat.title;
    vgCategorySelect.appendChild(opt);
  });
  if (previousValue && categories.some((cat) => String(cat.id) === previousValue)) {
    vgCategorySelect.value = previousValue;
  } else if (activeCategoryId !== null) {
    vgCategorySelect.value = String(activeCategoryId);
  }
}

function populateGoalSelect() {
  if (!vmGoalSelect) return;
  const previousValue = vmGoalSelect.value;
  const activeGoals = goalsByCategory()[activeCategoryId] ?? [];
  vmGoalSelect.replaceChildren();
  activeGoals.forEach((goal) => {
    const opt = document.createElement('option');
    opt.value = String(goal.id);
    opt.textContent = goal.title;
    vmGoalSelect.appendChild(opt);
  });
  if (previousValue && activeGoals.some((goal) => String(goal.id) === previousValue)) {
    vmGoalSelect.value = previousValue;
  }
}

setupVisionCategoriesCreateForm(document.getElementById('forms-container'));
setupVisionGoalsCreateForm(document.getElementById('forms-container'));
setupVisionMilestonesCreateForm(document.getElementById('forms-container'));
// @inject-forms

async function init() {
  await loadList();
}

init();

function setupVisionCategoriesCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = 'New Category';
  wrapper.appendChild(titleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  const titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Category';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-red-600 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const data = { title: titleInput.value };
    const res = await post('/api/vision_categories_create', data);
    submitBtn.disabled = false;
    if (res.ok) {
      form.reset();
      await loadList();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  wrapper.appendChild(form);
  container.appendChild(wrapper);
}

function setupVisionGoalsCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = 'New Goal';
  wrapper.appendChild(titleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const categoryLabel = document.createElement('label');
  categoryLabel.className = 'block text-sm font-medium text-gray-700';
  categoryLabel.textContent = 'Category';
  vgCategorySelect = document.createElement('select');
  vgCategorySelect.name = 'category_id';
  vgCategorySelect.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  vgCategorySelect.required = true;
  form.appendChild(categoryLabel);
  form.appendChild(vgCategorySelect);

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  const titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const targetYearLabel = document.createElement('label');
  targetYearLabel.className = 'block text-sm font-medium text-gray-700';
  targetYearLabel.textContent = 'Target Year';
  const targetYearInput = document.createElement('input');
  targetYearInput.type = 'number';
  targetYearInput.name = 'target_year';
  targetYearInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  targetYearInput.required = true;
  form.appendChild(targetYearLabel);
  form.appendChild(targetYearInput);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Goal';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-red-600 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const data = {
      category_id: parseInt(vgCategorySelect.value, 10),
      title: titleInput.value,
      target_year: parseInt(targetYearInput.value, 10),
    };
    const res = await post('/api/vision_goals_create', data);
    submitBtn.disabled = false;
    if (res.ok) {
      form.reset();
      await loadList();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  wrapper.appendChild(form);
  container.appendChild(wrapper);

  populateCategorySelect();
}

function setupVisionMilestonesCreateForm(container) {
  const wrapper = document.createElement('div');
  wrapper.className = 'border border-gray-200 rounded-lg p-4 bg-white space-y-3 mt-4';

  const titleEl = document.createElement('h3');
  titleEl.className = 'text-sm font-semibold text-gray-900';
  titleEl.textContent = 'New Milestone';
  wrapper.appendChild(titleEl);

  const form = document.createElement('form');
  form.className = 'space-y-3';

  const goalLabel = document.createElement('label');
  goalLabel.className = 'block text-sm font-medium text-gray-700';
  goalLabel.textContent = 'Goal';
  vmGoalSelect = document.createElement('select');
  vmGoalSelect.name = 'goal_id';
  vmGoalSelect.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  vmGoalSelect.required = true;
  form.appendChild(goalLabel);
  form.appendChild(vmGoalSelect);

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  const titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.className =
    'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  titleInput.required = true;
  form.appendChild(titleLabel);
  form.appendChild(titleInput);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  submitBtn.textContent = 'Add Milestone';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-red-600 hidden';
  form.appendChild(errEl);

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const data = {
      goal_id: parseInt(vmGoalSelect.value, 10),
      title: titleInput.value,
    };
    const res = await post('/api/vision_milestones_create', data);
    submitBtn.disabled = false;
    if (res.ok) {
      form.reset();
      await loadList();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  wrapper.appendChild(form);
  container.appendChild(wrapper);

  populateGoalSelect();
}

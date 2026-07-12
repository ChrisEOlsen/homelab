import { get, post, put, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

const MONTH_NAMES = [
  'January', 'February', 'March', 'April', 'May', 'June',
  'July', 'August', 'September', 'October', 'November', 'December',
];

const MOODS = ['neutral', 'happy', 'great', 'sad', 'angry', 'tired'];

let entries = [];
let selectedId = null;

// Detail form element refs (recreated on each render of the detail pane)
let jTitleInput, jMoodSelect, jDateInput, jContentInput, jErrEl, jSaveBtn, jDeleteBtn, jNewBtn;

async function loadList() {
  const res = await get('/api/journal');
  if (!res.ok) {
    app.replaceChildren();
    const p = document.createElement('p');
    p.className = 'text-sm text-red-600';
    p.textContent = res.error ?? 'Failed to load.';
    app.appendChild(p);
    return;
  }
  entries = res.data ?? [];
  if (selectedId !== null && !entries.some((e) => e.id === selectedId)) {
    selectedId = null;
  }
  render();
}

function groupByMonth(items) {
  const groups = new Map();
  items.forEach((item) => {
    const key = (item.entry_date || '').slice(0, 7) || 'unknown';
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(item);
  });
  // Newest month first
  return new Map([...groups.entries()].sort((a, b) => (a[0] < b[0] ? 1 : -1)));
}

function monthLabel(key) {
  const parts = key.split('-');
  if (parts.length !== 2) return key;
  const year = Number(parts[0]);
  const monthIdx = Number(parts[1]) - 1;
  if (Number.isNaN(year) || monthIdx < 0 || monthIdx > 11) return key;
  return MONTH_NAMES[monthIdx] + ' ' + year;
}

function render() {
  app.replaceChildren();

  const layout = document.createElement('div');
  layout.className = 'flex gap-6 items-start';

  layout.appendChild(renderSidebar());
  layout.appendChild(renderDetail());

  app.appendChild(layout);
}

function renderSidebar() {
  const sidebar = document.createElement('div');
  sidebar.className = 'w-64 shrink-0 space-y-4';

  jNewBtn = document.createElement('button');
  jNewBtn.type = 'button';
  jNewBtn.className = 'w-full px-3 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  jNewBtn.textContent = 'New Entry';
  jNewBtn.addEventListener('click', async () => {
    jNewBtn.disabled = true;
    const res = await post('/api/journal_entries_create', {});
    jNewBtn.disabled = false;
    if (res.ok && res.data && typeof res.data.id !== 'undefined') {
      selectedId = res.data.id;
      await loadList();
    }
  });
  sidebar.appendChild(jNewBtn);

  if (entries.length === 0) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'No entries yet.';
    sidebar.appendChild(p);
    return sidebar;
  }

  const groups = groupByMonth(entries);
  groups.forEach((items, key) => {
    const monthWrap = document.createElement('div');
    monthWrap.className = 'space-y-1';

    const header = document.createElement('p');
    header.className = 'text-xs font-medium uppercase tracking-wide text-gray-400 pt-2';
    header.textContent = monthLabel(key);
    monthWrap.appendChild(header);

    const list = document.createElement('ul');
    list.className = 'space-y-1';

    items.forEach((item) => {
      const li = document.createElement('li');
      const link = document.createElement('button');
      link.type = 'button';
      link.className =
        'w-full text-left px-2 py-1.5 rounded text-sm truncate transition-colors ' +
        (item.id === selectedId
          ? 'bg-gray-900 text-white'
          : 'text-gray-700 hover:bg-gray-100');
      link.textContent = (item.title && item.title.trim() !== '' ? item.title : '(untitled)') + ' · ' + item.entry_date;
      link.addEventListener('click', () => {
        selectedId = item.id;
        render();
      });
      li.appendChild(link);
      list.appendChild(li);
    });

    monthWrap.appendChild(list);
    sidebar.appendChild(monthWrap);
  });

  return sidebar;
}

function renderDetail() {
  const detail = document.createElement('div');
  detail.className = 'flex-1 min-w-0 border border-gray-200 rounded-lg bg-white p-4 space-y-3';

  const entry = entries.find((e) => e.id === selectedId);

  if (!entry) {
    const p = document.createElement('p');
    p.className = 'text-sm text-gray-500';
    p.textContent = 'Select an entry, or create a new one.';
    detail.appendChild(p);
    return detail;
  }

  const titleLabel = document.createElement('label');
  titleLabel.className = 'block text-sm font-medium text-gray-700';
  titleLabel.textContent = 'Title';
  jTitleInput = document.createElement('input');
  jTitleInput.type = 'text';
  jTitleInput.value = entry.title ?? '';
  jTitleInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  detail.appendChild(titleLabel);
  detail.appendChild(jTitleInput);

  const metaRow = document.createElement('div');
  metaRow.className = 'flex gap-4';

  const moodWrap = document.createElement('div');
  moodWrap.className = 'flex-1';
  const moodLabel = document.createElement('label');
  moodLabel.className = 'block text-sm font-medium text-gray-700';
  moodLabel.textContent = 'Mood';
  jMoodSelect = document.createElement('select');
  jMoodSelect.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  MOODS.forEach((m) => {
    const opt = document.createElement('option');
    opt.value = m;
    opt.textContent = m;
    if (m === entry.mood) opt.selected = true;
    jMoodSelect.appendChild(opt);
  });
  moodWrap.appendChild(moodLabel);
  moodWrap.appendChild(jMoodSelect);
  metaRow.appendChild(moodWrap);

  const dateWrap = document.createElement('div');
  dateWrap.className = 'flex-1';
  const dateLabel = document.createElement('label');
  dateLabel.className = 'block text-sm font-medium text-gray-700';
  dateLabel.textContent = 'Date';
  jDateInput = document.createElement('input');
  jDateInput.type = 'date';
  jDateInput.value = entry.entry_date ?? '';
  jDateInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  dateWrap.appendChild(dateLabel);
  dateWrap.appendChild(jDateInput);
  metaRow.appendChild(dateWrap);

  detail.appendChild(metaRow);

  const contentLabel = document.createElement('label');
  contentLabel.className = 'block text-sm font-medium text-gray-700';
  contentLabel.textContent = 'Content';
  jContentInput = document.createElement('textarea');
  jContentInput.rows = 10;
  jContentInput.value = entry.content ?? '';
  jContentInput.className = 'mt-1 block w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-1 focus:ring-gray-900';
  detail.appendChild(contentLabel);
  detail.appendChild(jContentInput);

  jErrEl = document.createElement('p');
  jErrEl.className = 'text-sm text-red-600 hidden';
  detail.appendChild(jErrEl);

  const btnRow = document.createElement('div');
  btnRow.className = 'flex items-center gap-2';

  jSaveBtn = document.createElement('button');
  jSaveBtn.type = 'button';
  jSaveBtn.className = 'px-4 py-2 bg-gray-900 text-white text-sm rounded hover:bg-gray-700 transition-colors';
  jSaveBtn.textContent = 'Save';
  jSaveBtn.addEventListener('click', async () => {
    jSaveBtn.disabled = true;
    jErrEl.classList.add('hidden');
    const data = {
      title: jTitleInput.value,
      content: jContentInput.value,
      mood: jMoodSelect.value,
      entry_date: jDateInput.value,
    };
    const res = await put('/api/journal_entries/' + entry.id, data);
    jSaveBtn.disabled = false;
    if (res.ok) {
      await loadList();
    } else {
      jErrEl.textContent = res.error ?? 'Something went wrong.';
      jErrEl.classList.remove('hidden');
    }
  });
  btnRow.appendChild(jSaveBtn);

  jDeleteBtn = document.createElement('button');
  jDeleteBtn.type = 'button';
  jDeleteBtn.className = 'px-4 py-2 text-sm rounded border border-red-200 text-red-600 hover:bg-red-50 transition-colors';
  jDeleteBtn.textContent = 'Delete';
  jDeleteBtn.addEventListener('click', async () => {
    jDeleteBtn.disabled = true;
    await del('/api/journal_entries/' + entry.id);
    if (selectedId === entry.id) selectedId = null;
    await loadList();
  });
  btnRow.appendChild(jDeleteBtn);

  detail.appendChild(btnRow);

  return detail;
}

// @inject-forms

async function init() {
  await loadList();
}

init();

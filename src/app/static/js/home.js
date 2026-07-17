import { get, post, put, del } from '/static/js/lib/api.js';

const focusListEl = document.getElementById('focus-list');
const remindersListEl = document.getElementById('reminders-list');
const shortcutsGridEl = document.getElementById('shortcuts-grid');
const focusFormMount = document.getElementById('focus-form-mount');
const shortcutFormMount = document.getElementById('shortcut-form-mount');

// Delete-error elements: created once, inserted as siblings of their list
// containers so they survive the containers' replaceChildren() re-renders.
const focusDeleteErrEl = document.createElement('p');
focusDeleteErrEl.className = 'text-sm text-danger mt-2 hidden';
focusListEl.insertAdjacentElement('afterend', focusDeleteErrEl);

const shortcutDeleteErrEl = document.createElement('p');
shortcutDeleteErrEl.className = 'text-sm text-danger mt-2 hidden';
shortcutsGridEl.insertAdjacentElement('afterend', shortcutDeleteErrEl);

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

// ---- Helpers ----
function formatRemindAt(value) {
  const d = new Date(value);
  if (isNaN(d.getTime())) return value;
  return d.toLocaleString(undefined, { dateStyle: 'medium', timeStyle: 'short' });
}

function shortcutHost(url) {
  try {
    return new URL(url).hostname.replace(/^www\./, '');
  } catch {
    return url;
  }
}

function renderEmpty(container, message) {
  container.replaceChildren();
  const p = document.createElement('p');
  p.className = 'text-sm text-ink-dim';
  p.textContent = message;
  container.appendChild(p);
}

function renderError(container, message) {
  container.replaceChildren();
  const p = document.createElement('p');
  p.className = 'text-sm text-danger';
  p.textContent = message;
  container.appendChild(p);
}

// ---- Focus notes ----
function renderFocuses(items) {
  if (items.length === 0) {
    renderEmpty(focusListEl, 'No focuses set. What matters right now?');
    return;
  }
  const ol = document.createElement('ol');
  ol.className = 'space-y-2';
  items.forEach((item, index) => {
    const li = document.createElement('li');
    li.className = 'flex items-start gap-3 border border-hairline bg-surface-raised px-3 py-2';

    const num = document.createElement('span');
    num.className = 'text-xs text-accent shrink-0 pt-0.5';
    num.textContent = String(index + 1).padStart(2, '0');
    li.appendChild(num);

    const text = document.createElement('p');
    text.className = 'flex-1 min-w-0 text-sm text-ink break-words';
    text.textContent = item.text;
    li.appendChild(text);

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'shrink-0 text-xs text-ink-dim hover:text-danger transition-colors px-1';
    delBtn.textContent = '×';
    delBtn.setAttribute('aria-label', 'Delete focus: ' + item.text);
    delBtn.addEventListener('click', async () => {
      delBtn.disabled = true;
      focusDeleteErrEl.classList.add('hidden');
      const res = await del('/api/focuses/' + item.id);
      if (res.ok) {
        await loadDashboard();
      } else {
        delBtn.disabled = false;
        focusDeleteErrEl.textContent = res.error ?? 'Failed to delete.';
        focusDeleteErrEl.classList.remove('hidden');
      }
    });
    li.appendChild(delBtn);

    ol.appendChild(li);
  });
  focusListEl.replaceChildren(ol);
}

// ---- Upcoming reminders (read-only) ----
function renderReminders(items) {
  if (items.length === 0) {
    renderEmpty(remindersListEl, 'Nothing upcoming — add a reminder to see it here.');
    return;
  }
  const ul = document.createElement('ul');
  ul.className = 'space-y-2';
  items.forEach((item) => {
    const li = document.createElement('li');
    li.className = 'flex items-center justify-between gap-3 border border-hairline bg-surface-raised px-3 py-2';

    const left = document.createElement('div');
    left.className = 'flex flex-1 items-center gap-2 min-w-0';

    const dot = document.createElement('span');
    dot.className = 'h-1.5 w-1.5 rounded-full bg-ok shrink-0';
    dot.setAttribute('aria-hidden', 'true');
    left.appendChild(dot);

    const title = document.createElement('span');
    title.className = 'text-sm text-ink truncate min-w-0';
    title.textContent = item.title;
    left.appendChild(title);

    li.appendChild(left);

    const time = document.createElement('span');
    time.className = 'text-xs text-ink-dim shrink-0 tabular-nums';
    time.textContent = formatRemindAt(item.remind_at);
    li.appendChild(time);

    ul.appendChild(li);
  });
  remindersListEl.replaceChildren(ul);
}

// ---- Shortcuts ----
function renderShortcuts(items) {
  if (items.length === 0) {
    renderEmpty(shortcutsGridEl, 'No shortcuts yet. Add your first one below.');
    return;
  }
  const frag = document.createDocumentFragment();
  items.forEach((item) => {
    const tile = document.createElement('div');
    tile.draggable = true;
    tile.dataset.id = String(item.id);
    tile.className = 'relative border border-hairline bg-surface-raised hover:border-accent transition-colors p-3 cursor-grab';

    tile.addEventListener('dragstart', (e) => {
      e.dataTransfer.setData('text/plain', String(item.id));
      e.dataTransfer.effectAllowed = 'move';
      tile.classList.add('opacity-50');
    });
    tile.addEventListener('dragend', () => tile.classList.remove('opacity-50'));
    tile.addEventListener('dragover', (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
    });
    tile.addEventListener('drop', async (e) => {
      e.preventDefault();
      const draggedId = Number(e.dataTransfer.getData('text/plain'));
      if (!draggedId || draggedId === item.id) return;
      const draggedEl = shortcutsGridEl.querySelector(`[data-id="${draggedId}"]`);
      if (!draggedEl) return;
      const rect = tile.getBoundingClientRect();
      const insertBefore = e.clientX - rect.left < rect.width / 2;
      shortcutsGridEl.insertBefore(draggedEl, insertBefore ? tile : tile.nextSibling);
      const order = Array.from(shortcutsGridEl.children).map((el) => Number(el.dataset.id));
      await put('/api/shortcuts_reorder', { order });
      await loadDashboard();
    });

    const delBtn = document.createElement('button');
    delBtn.type = 'button';
    delBtn.className = 'absolute top-1 right-1 text-xs text-ink-dim hover:text-danger transition-colors px-1.5 py-0.5';
    delBtn.textContent = '×';
    delBtn.setAttribute('aria-label', 'Delete shortcut: ' + item.title);
    delBtn.addEventListener('click', async () => {
      delBtn.disabled = true;
      shortcutDeleteErrEl.classList.add('hidden');
      const res = await del('/api/shortcuts/' + item.id);
      if (res.ok) {
        await loadDashboard();
      } else {
        delBtn.disabled = false;
        shortcutDeleteErrEl.textContent = res.error ?? 'Failed to delete.';
        shortcutDeleteErrEl.classList.remove('hidden');
      }
    });
    tile.appendChild(delBtn);

    const link = document.createElement('a');
    link.href = item.url;
    link.className = 'block pr-4';

    const title = document.createElement('p');
    title.className = 'text-sm font-medium text-ink truncate';
    title.textContent = item.title;
    link.appendChild(title);

    const host = document.createElement('p');
    host.className = 'text-xs text-ink-dim truncate mt-0.5';
    host.textContent = shortcutHost(item.url);
    link.appendChild(host);

    tile.appendChild(link);
    frag.appendChild(tile);
  });
  shortcutsGridEl.replaceChildren(frag);
}

// ---- Load ----
async function loadDashboard() {
  const res = await get('/api/dashboard');
  if (!res.ok) {
    const message = res.error ?? 'Failed to load.';
    renderError(focusListEl, message);
    renderError(remindersListEl, message);
    renderError(shortcutsGridEl, message);
    return;
  }
  const data = res.data ?? {};
  renderFocuses(data.focuses ?? []);
  renderReminders(data.reminders ?? []);
  renderShortcuts(data.shortcuts ?? []);
}

// ---- Add-focus form (hand-built: home.js has no @inject-forms marker) ----
function setupFocusForm() {
  const form = document.createElement('form');
  form.className = 'flex gap-2';

  const input = document.createElement('input');
  input.type = 'text';
  input.name = 'text';
  input.placeholder = 'Add a focus…';
  input.required = true;
  input.className =
    'flex-1 bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(input);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className =
    'text-xs uppercase tracking-wide px-3 py-2 border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors shrink-0';
  submitBtn.textContent = 'Add';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-danger mt-2 hidden';

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const res = await post('/api/focuses_create', { text: input.value });
    submitBtn.disabled = false;
    if (res.ok) {
      input.value = '';
      await loadDashboard();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  focusFormMount.appendChild(form);
  focusFormMount.appendChild(errEl);
}

// ---- Add-shortcut form (hand-built: home.js has no @inject-forms marker) ----
function setupShortcutForm() {
  const form = document.createElement('form');
  form.className = 'flex flex-col sm:flex-row gap-2';

  const titleInput = document.createElement('input');
  titleInput.type = 'text';
  titleInput.name = 'title';
  titleInput.placeholder = 'Title';
  titleInput.required = true;
  titleInput.className =
    'flex-1 bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(titleInput);

  const urlInput = document.createElement('input');
  urlInput.type = 'text';
  urlInput.name = 'url';
  urlInput.placeholder = 'URL (e.g. nas.local)';
  urlInput.required = true;
  urlInput.className =
    'flex-1 bg-canvas border border-hairline px-3 py-2 text-sm text-ink placeholder:text-ink-dim focus:outline-none focus:border-accent';
  form.appendChild(urlInput);

  const submitBtn = document.createElement('button');
  submitBtn.type = 'submit';
  submitBtn.className =
    'text-xs uppercase tracking-wide px-3 py-2 border border-accent text-accent hover:bg-accent hover:text-canvas transition-colors shrink-0';
  submitBtn.textContent = 'Add';
  form.appendChild(submitBtn);

  const errEl = document.createElement('p');
  errEl.className = 'text-sm text-danger mt-2 hidden';

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    submitBtn.disabled = true;
    errEl.classList.add('hidden');
    const res = await post('/api/shortcuts_create', { title: titleInput.value, url: urlInput.value });
    submitBtn.disabled = false;
    if (res.ok) {
      titleInput.value = '';
      urlInput.value = '';
      await loadDashboard();
    } else {
      errEl.textContent = res.error ?? 'Something went wrong.';
      errEl.classList.remove('hidden');
    }
  });

  shortcutFormMount.appendChild(form);
  shortcutFormMount.appendChild(errEl);
}

setupFocusForm();
setupShortcutForm();

async function init() {
  await loadDashboard();
}

init();

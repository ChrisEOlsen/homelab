// Site chrome: the sticky header, the mobile drawer, the clock and the
// theme toggles — mounted from one place instead of being copy-pasted
// into every page. Adding or removing a feature page is a one-line edit
// to NAV_ITEMS below; nothing else in the app knows the nav exists.
//
// This module self-mounts on import, so a page includes it with a plain
// script tag and needs no markup of its own:
//
//   <script type="module" src="/static/js/lib/nav.js"></script>
//   <script type="module" src="/static/js/lib/theme.js"></script>
//
// Order matters: theme.js wires the [data-theme-toggle] buttons that
// live inside this chrome, so nav.js must run first. Module scripts
// execute in document order, which is what guarantees that.

const NAV_ITEMS = [
  { href: '/static/pages/home.html', label: 'Dashboard' },
  { href: '/static/pages/todos.html', label: 'Tasks' },
  { href: '/static/pages/bookmarks.html', label: 'Bookmarks' },
  { href: '/static/pages/logger.html', label: 'Logger' },
  { href: '/static/pages/finances.html', label: 'Finances' },
];

// `/` is served by HomeGET as home.html, so it has to fold into the same
// key the hrefs above produce.
function currentPageFile() {
  const path = window.location.pathname;
  if (path === '/' || path === '') return 'home.html';
  return path.slice(path.lastIndexOf('/') + 1);
}

// The theme toggle appears twice (header and drawer footer), so its
// markup is built rather than duplicated. SVG needs createElementNS —
// createElement would produce inert HTML-namespaced elements.
const SVG_NS = 'http://www.w3.org/2000/svg';

const SUN_PATH =
  'M10 15a5 5 0 100-10 5 5 0 000 10zM10 0a1 1 0 011 1v1a1 1 0 11-2 0V1a1 1 0 011-1zm0 16a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM3.05 3.05a1 1 0 011.414 0l.707.707a1 1 0 11-1.414 1.414l-.707-.707a1 1 0 010-1.414zm11.78 11.78a1 1 0 011.415 0l.707.707a1 1 0 01-1.415 1.415l-.707-.707a1 1 0 010-1.415zM0 10a1 1 0 011-1h1a1 1 0 110 2H1a1 1 0 01-1-1zm16 0a1 1 0 011-1h1a1 1 0 110 2h-1a1 1 0 01-1-1zM3.05 16.95a1 1 0 010-1.414l.707-.707a1 1 0 111.414 1.414l-.707.707a1 1 0 01-1.414 0zm11.78-11.78a1 1 0 010-1.414l.707-.707a1 1 0 111.415 1.414l-.707.707a1 1 0 01-1.415 0z';

const MOON_PATH = 'M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z';

function svgIcon(className, pathData) {
  const svg = document.createElementNS(SVG_NS, 'svg');
  svg.setAttribute('class', className);
  svg.setAttribute('viewBox', '0 0 20 20');
  svg.setAttribute('fill', 'currentColor');
  svg.setAttribute('aria-hidden', 'true');
  const path = document.createElementNS(SVG_NS, 'path');
  path.setAttribute('d', pathData);
  svg.appendChild(path);
  return svg;
}

function themeToggle() {
  const btn = document.createElement('button');
  btn.type = 'button';
  btn.setAttribute('data-theme-toggle', '');
  btn.setAttribute('aria-pressed', 'false');
  btn.setAttribute('aria-label', 'Toggle dark mode');
  btn.className =
    'inline-flex items-center justify-center h-7 w-7 text-ink-dim hover:text-ink transition-colors';
  btn.appendChild(svgIcon('theme-icon-sun w-4 h-4', SUN_PATH));
  btn.appendChild(svgIcon('theme-icon-moon w-4 h-4 hidden', MOON_PATH));
  return btn;
}

// The accent dot that marks the current page in both navs.
function currentDot() {
  const dot = document.createElement('span');
  dot.className = 'h-1.5 w-1.5 rounded-full bg-accent';
  dot.setAttribute('aria-hidden', 'true');
  return dot;
}

function clockRow(clockId) {
  const wrap = document.createElement('span');
  wrap.className = 'flex items-center gap-1.5';
  const time = document.createElement('time');
  time.id = clockId;
  time.textContent = '--:--:--';
  wrap.appendChild(time);
  return wrap;
}

// ---- Nav lists ----
// Both navs are the same list in two skins, so they share one builder.
// Labels go in via textContent — the values here are internal, but the
// house rule is textContent everywhere and it costs nothing to keep.

const DESKTOP_SKIN = {
  link: 'px-3 py-1.5 text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors whitespace-nowrap',
  current: 'px-3 py-1.5 flex items-center gap-1.5 text-ink whitespace-nowrap',
  divider: false,
};

const MOBILE_SKIN = {
  link: 'px-3 py-3 text-ink-dim hover:text-ink hover:bg-surface-raised transition-colors',
  current: 'px-3 py-3 flex items-center gap-2 text-ink',
  divider: true,
};

function navList(skin, current) {
  const nodes = NAV_ITEMS.map((item, i) => {
    const isCurrent = item.href.endsWith('/' + current);
    const node = document.createElement(isCurrent ? 'span' : 'a');

    if (isCurrent) {
      node.setAttribute('aria-current', 'page');
      node.className = skin.current;
      node.appendChild(currentDot());
      node.appendChild(document.createTextNode(item.label));
    } else {
      node.href = item.href;
      node.className = skin.link;
      node.textContent = item.label;
    }

    // The drawer separates rows with hairlines; the last row sits on the
    // list's own edge and would double it.
    if (skin.divider && i < NAV_ITEMS.length - 1) {
      node.className += ' border-b border-hairline';
    }
    return node;
  });

  return nodes;
}

// ---- Chrome ----

function buildHeader(current) {
  const header = document.createElement('header');
  header.className = 'sticky top-0 z-20 border-b border-hairline bg-surface';

  const bar = document.createElement('div');
  bar.className =
    'max-w-6xl mx-auto px-4 sm:px-6 h-14 flex items-center justify-between gap-4';

  const brand = document.createElement('a');
  brand.href = '/static/pages/home.html';
  brand.className = 'flex items-center gap-2 text-sm font-medium shrink-0';
  const beacon = document.createElement('span');
  beacon.className = 'h-2 w-2 rounded-full bg-accent led-pulse';
  beacon.setAttribute('aria-hidden', 'true');
  brand.appendChild(beacon);
  brand.appendChild(document.createTextNode('Homelab'));

  const nav = document.createElement('nav');
  nav.setAttribute('aria-label', 'Feature pages');
  nav.className =
    'hidden md:flex items-center divide-x divide-hairline border border-hairline bg-surface text-xs overflow-x-auto';
  nav.append(...navList(DESKTOP_SKIN, current));

  const meta = document.createElement('div');
  meta.className =
    'hidden md:flex items-center gap-3 shrink-0 text-xs text-ink-dim tabular-nums';
  meta.appendChild(clockRow('clock'));
  meta.appendChild(themeToggle());

  const toggle = document.createElement('button');
  toggle.id = 'nav-toggle';
  toggle.type = 'button';
  toggle.className =
    'md:hidden flex flex-col justify-center gap-1.5 h-9 w-9 shrink-0';
  toggle.setAttribute('aria-expanded', 'false');
  toggle.setAttribute('aria-controls', 'mobile-drawer');
  const toggleLabel = document.createElement('span');
  toggleLabel.className = 'sr-only';
  toggleLabel.textContent = 'Toggle navigation';
  toggle.appendChild(toggleLabel);
  for (let i = 0; i < 3; i++) {
    const bar3 = document.createElement('span');
    bar3.className = 'block h-px w-5 bg-ink';
    bar3.setAttribute('aria-hidden', 'true');
    toggle.appendChild(bar3);
  }

  bar.append(brand, nav, meta, toggle);
  header.appendChild(bar);
  return header;
}

function buildDrawer(current) {
  const backdrop = document.createElement('div');
  backdrop.id = 'mobile-drawer-backdrop';
  backdrop.className = 'md:hidden fixed inset-0 bg-black/60 z-30 hidden';

  const drawer = document.createElement('aside');
  drawer.id = 'mobile-drawer';
  drawer.className =
    'md:hidden fixed top-0 right-0 h-full w-72 max-w-[80vw] bg-surface border-l border-hairline z-40 translate-x-full transition-transform duration-200 ease-out flex flex-col';

  const head = document.createElement('div');
  head.className =
    'h-14 flex items-center justify-between px-4 border-b border-hairline shrink-0';
  const heading = document.createElement('span');
  heading.className = 'text-xs text-ink-dim';
  heading.textContent = 'Menu';
  const close = document.createElement('button');
  close.id = 'nav-close';
  close.type = 'button';
  close.className = 'text-ink-dim hover:text-ink text-lg leading-none px-2 py-1';
  const times = document.createElement('span');
  times.setAttribute('aria-hidden', 'true');
  times.textContent = '×';
  const closeLabel = document.createElement('span');
  closeLabel.className = 'sr-only';
  closeLabel.textContent = 'Close menu';
  close.append(times, closeLabel);
  head.append(heading, close);

  const nav = document.createElement('nav');
  nav.setAttribute('aria-label', 'Feature pages (mobile)');
  nav.className = 'flex flex-col p-2 text-sm overflow-y-auto';
  nav.append(...navList(MOBILE_SKIN, current));

  const foot = document.createElement('div');
  foot.className =
    'mt-auto px-4 py-3 border-t border-hairline text-xs text-ink-dim flex items-center justify-between gap-1.5 tabular-nums shrink-0';
  foot.appendChild(clockRow('clock-mobile'));
  foot.appendChild(themeToggle());

  drawer.append(head, nav, foot);
  return { backdrop, drawer };
}

// ---- Behavior ----

function startClock() {
  const clock = document.getElementById('clock');
  const clockMobile = document.getElementById('clock-mobile');

  function tick() {
    const text = new Date().toLocaleTimeString([], { hour12: false });
    if (clock) clock.textContent = text;
    if (clockMobile) clockMobile.textContent = text;
  }

  tick();
  setInterval(tick, 1000);
}

function wireDrawer() {
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
}

export function mountNav(current = currentPageFile()) {
  const { backdrop, drawer } = buildDrawer(current);
  // body is `flex flex-col` with main stretching — the header has to be
  // the first child to sit above it in flow.
  document.body.prepend(buildHeader(current), backdrop, drawer);
  startClock();
  wireDrawer();
}

mountNav();

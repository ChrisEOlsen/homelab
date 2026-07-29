const STORAGE_KEY = 'theme';

function getStoredTheme() {
  try {
    return localStorage.getItem(STORAGE_KEY);
  } catch (e) {
    return null;
  }
}

// Dark is the app's default: the Uplink theme is built around emitted
// light on a deep field, and the light theme is the daylight variant of
// it rather than the other way round. An explicit choice still wins, and
// the toggle writes one on first use.
function getEffectiveTheme() {
  const stored = getStoredTheme();
  if (stored === 'light' || stored === 'dark') return stored;
  return 'dark';
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  document.querySelectorAll('[data-theme-toggle]').forEach((btn) => {
    const sun = btn.querySelector('.theme-icon-sun');
    const moon = btn.querySelector('.theme-icon-moon');
    const isDark = theme === 'dark';
    if (sun) sun.classList.toggle('hidden', !isDark);
    if (moon) moon.classList.toggle('hidden', isDark);
    btn.setAttribute('aria-pressed', String(isDark));
  });
}

function toggleTheme() {
  const current = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light';
  const next = current === 'dark' ? 'light' : 'dark';
  applyTheme(next);
  try {
    localStorage.setItem(STORAGE_KEY, next);
  } catch (e) {
    // storage unavailable (e.g. private browsing) — theme still applies for this session
  }
}

export function initTheme() {
  applyTheme(getEffectiveTheme());
  document.querySelectorAll('[data-theme-toggle]').forEach((btn) => {
    btn.addEventListener('click', toggleTheme);
  });
}

initTheme();

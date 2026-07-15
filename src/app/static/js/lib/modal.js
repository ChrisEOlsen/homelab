// Centered dialog: dark scrim backdrop, bg-surface panel, Escape/backdrop
// click to close, minimal Tab focus trap, focus returns to the trigger on
// close. Shared by any page that needs an add/edit modal instead of an
// inline form (Bookmarks, Logger, Codex, ...).
export function createModal(titleId) {
  const backdrop = document.createElement('div');
  backdrop.className = 'fixed inset-0 bg-black/60 z-50 hidden flex items-center justify-center p-4';

  const panel = document.createElement('div');
  panel.className = 'bg-surface border border-hairline p-5 w-full max-w-md space-y-3';
  panel.setAttribute('role', 'dialog');
  panel.setAttribute('aria-modal', 'true');
  if (titleId) panel.setAttribute('aria-labelledby', titleId);
  panel.addEventListener('click', (e) => e.stopPropagation());
  backdrop.appendChild(panel);

  let triggerEl = null;

  function onKeydown(e) {
    if (e.key === 'Escape') {
      close();
      return;
    }
    if (e.key !== 'Tab') return;
    const focusable = panel.querySelectorAll('button, input, select, textarea, a[href]');
    if (focusable.length === 0) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault();
      last.focus();
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault();
      first.focus();
    }
  }

  function open(trigger) {
    triggerEl = trigger ?? document.activeElement;
    backdrop.classList.remove('hidden');
    document.addEventListener('keydown', onKeydown);
    const firstField = panel.querySelector('input, select, textarea');
    (firstField ?? panel).focus?.();
  }

  function close() {
    backdrop.classList.add('hidden');
    document.removeEventListener('keydown', onKeydown);
    if (triggerEl instanceof HTMLElement) triggerEl.focus();
    triggerEl = null;
  }

  backdrop.addEventListener('click', close);
  document.body.appendChild(backdrop);

  return { backdrop, panel, open, close };
}

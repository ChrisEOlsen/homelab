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

// Replays the panel's entrance animation. A modal that's reused across
// opens (hidden rather than removed) won't re-run its CSS animation on
// its own — display:none -> flex doesn't restart one — so the class has
// to come off, layout has to be forced, and it goes back on.
export function playDialogEntrance(panel) {
  panel.classList.remove('dialog-enter');
  void panel.offsetWidth;
  panel.classList.add('dialog-enter');
}

let confirmSeq = 0;

// A one-shot confirmation dialog, resolving true only if the confirming
// button is pressed — Escape, the backdrop and Cancel all resolve false.
// Drop-in shaped for the window.confirm() calls it replaces:
//   if (!(await confirmAction({ ... }))) return;
// Unlike window.confirm it doesn't block the main thread, and it can say
// what's about to happen in the app's own voice.
export function confirmAction({
  title,
  message = '',
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = false,
  trigger = null,
} = {}) {
  const titleId = 'confirm-title-' + ++confirmSeq;
  const { backdrop, panel, open, close } = createModal(titleId);

  const heading = document.createElement('h3');
  heading.id = titleId;
  heading.className = 'text-sm font-semibold text-ink';
  heading.textContent = title;
  panel.appendChild(heading);

  if (message) {
    const p = document.createElement('p');
    p.className = 'text-sm text-ink-dim';
    p.textContent = message;
    panel.appendChild(p);
  }

  const row = document.createElement('div');
  row.className = 'flex items-center justify-end gap-2 pt-1';
  panel.appendChild(row);

  const cancelBtn = document.createElement('button');
  cancelBtn.type = 'button';
  cancelBtn.className =
    'px-4 py-2 border border-hairline text-ink-dim text-xs font-medium hover:text-ink hover:bg-surface-raised transition-colors';
  cancelBtn.textContent = cancelLabel;
  row.appendChild(cancelBtn);

  const confirmBtn = document.createElement('button');
  confirmBtn.type = 'button';
  confirmBtn.className = danger
    ? 'px-4 py-2 border border-danger text-danger text-xs font-medium hover:bg-danger hover:text-canvas transition-colors'
    : 'px-4 py-2 border border-accent text-accent text-xs font-medium hover:bg-accent hover:text-canvas transition-colors';
  confirmBtn.textContent = confirmLabel;
  row.appendChild(confirmBtn);

  return new Promise((resolve) => {
    let settled = false;

    // createModal already closes on Escape and backdrop click, but it has
    // no way to report which way the dialog went — these listeners run
    // alongside its own and carry the answer out.
    function settle(result) {
      if (settled) return;
      settled = true;
      document.removeEventListener('keydown', onEscape, true);
      close();
      backdrop.remove();
      resolve(result);
    }
    function onEscape(e) {
      if (e.key === 'Escape') settle(false);
    }

    cancelBtn.addEventListener('click', () => settle(false));
    confirmBtn.addEventListener('click', () => settle(true));
    backdrop.addEventListener('click', () => settle(false));
    document.addEventListener('keydown', onEscape, true);

    open(trigger);
    playDialogEntrance(panel);
    confirmBtn.focus();
  });
}

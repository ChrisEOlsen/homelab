// Shared list-motion helpers. Every duration here has a matching
// transition/animation duration in input.css — they live together in one
// module so two pages animating the same way can't drift apart silently.

export const CLEAR_ROW_MS = 380;
export const CLEAR_STAGGER_MS = 45;

export const prefersReducedMotion = () =>
  window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Collapses a row out of a list: pin its current height, then let the
// .task-clearing transition run it to zero. Height has to be an explicit
// number first — `auto` is not an animatable starting value.
//
// Resolves once the row has finished collapsing (but is still in the DOM),
// so a caller can wait for the visual departure before it reloads.
export function collapseRowOut(el, delay = 0) {
  if (prefersReducedMotion()) {
    el.remove();
    return Promise.resolve();
  }
  el.style.height = el.getBoundingClientRect().height + 'px';
  void el.offsetHeight;
  return new Promise((resolve) => {
    setTimeout(() => {
      el.classList.add('task-clearing');
      setTimeout(resolve, CLEAR_ROW_MS);
    }, delay);
  });
}

// Collapses a set of rows out together, each starting a beat after the one
// before it so the stack unzips top to bottom instead of blinking out.
export function collapseRowsOut(rows) {
  return Promise.all(rows.map((el, i) => collapseRowOut(el, i * CLEAR_STAGGER_MS)));
}

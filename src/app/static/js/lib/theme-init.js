// Theme bootstrap — the pre-paint step that must run before first render.
//
// This was an inline <script> in every page shell until the template's CSP
// (script-src 'self') made inline script a load-time violation: the browser
// would refuse to run it and the dark default would flash white before
// theme.js loaded. As an external module in <head> it is same-origin and
// allowed, and being in <head> (not deferred via type="module", which would
// run after parse) keeps it running before the body paints.
//
// It is deliberately NOT an ES module: module scripts are deferred by spec,
// which would defeat the point of running before paint. A plain classic
// script executes on parse, exactly when the old inline block did.
(function () {
  try {
    var t = localStorage.getItem('theme');
    if (t !== 'light' && t !== 'dark') t = 'dark';
    document.documentElement.dataset.theme = t;
  } catch (e) {}
})();
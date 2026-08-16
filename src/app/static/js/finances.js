import { get, post, del } from '/static/js/lib/api.js';

const app = document.getElementById('app');

async function init() {
  render();
}

function render() {
  const p = document.createElement('p');
  p.className = 'text-sm text-gray-500';
  p.textContent = 'TODO: implement Finances page.';
  app.replaceChildren();
  app.appendChild(p);
}

// @inject-forms

init();

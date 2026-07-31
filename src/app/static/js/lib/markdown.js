// Minimal safe markdown -> DOM renderer. No HTML string ever gets built or
// parsed — every node is created with createElement/textContent, so there is
// no injection surface and no need for a sanitizer. Supports: headings,
// bold/italic, inline code, fenced code blocks, unordered/ordered lists,
// links, blockquotes, pipe tables, inline $math$, and paragraphs. Anything
// unsupported just renders as plain text, which is the safe failure mode.
//
// The math support is typographic, not a LaTeX engine: real superscripts and
// subscripts plus symbol substitution (\times -> ×, \sum -> ∑). It covers
// the algebra and complexity notation these notes actually use. Anything
// beyond that — matrices, integrals with limits, nested fractions — renders
// as recognisable text rather than correctly typeset math.

// Renders markdown source into `container` (an existing DOM element).
export function renderMarkdown(container, source) {
  container.replaceChildren();
  // Strip \r so \r\n-saved content matches the same way \n content does —
  // otherwise a trailing \r can make an end-anchored regex (e.g. the heading
  // match below) disagree with an unanchored one used elsewhere for the same
  // line, which previously left `i` stuck and looped forever.
  const lines = (source ?? '').replace(/\r/g, '').split('\n');
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === '') {
      i++;
      continue;
    }

    // Fenced code block: ```lang ... ```
    if (line.trim().startsWith('```')) {
      const lang = line.trim().slice(3).trim();
      const codeLines = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith('```')) {
        codeLines.push(lines[i]);
        i++;
      }
      i++; // skip closing fence
      const pre = document.createElement('pre');
      pre.className = 'bg-canvas border border-hairline p-3 overflow-x-auto text-xs';
      const code = document.createElement('code');
      if (lang) code.className = 'language-' + lang;
      code.textContent = codeLines.join('\n');
      pre.appendChild(code);
      container.appendChild(pre);
      continue;
    }

    // Heading: # .. ######
    const headingMatch = line.match(/^(#{1,6})\s+(.*)$/);
    if (headingMatch) {
      const level = headingMatch[1].length;
      const h = document.createElement('h' + Math.min(level + 2, 6)); // offset so it nests under the page's own h1/h2
      h.className = headingClass(level);
      appendInline(h, headingMatch[2]);
      container.appendChild(h);
      i++;
      continue;
    }

    // Blockquote: consecutive lines starting with >
    if (line.trim().startsWith('>')) {
      const quoteLines = [];
      while (i < lines.length && lines[i].trim().startsWith('>')) {
        quoteLines.push(lines[i].trim().replace(/^>\s?/, ''));
        i++;
      }
      const bq = document.createElement('blockquote');
      bq.className = 'border-l-2 border-accent pl-3 text-ink-dim italic';
      appendInline(bq, quoteLines.join(' '));
      container.appendChild(bq);
      continue;
    }

    // Horizontal rule: --- / *** / ___ alone on a line.
    if (/^\s*([-*_])\1{2,}\s*$/.test(line)) {
      const hr = document.createElement('hr');
      hr.className = 'border-hairline my-3';
      container.appendChild(hr);
      i++;
      continue;
    }

    // Pipe table: a header row, an alignment row, then body rows. The
    // alignment row is what distinguishes a table from a paragraph that
    // happens to contain pipes, so both must be present to start one.
    if (isTableRow(line) && i + 1 < lines.length && isAlignmentRow(lines[i + 1])) {
      const headers = splitTableRow(line);
      const alignments = splitTableRow(lines[i + 1]).map(alignmentOf);
      i += 2;

      const bodyRows = [];
      while (i < lines.length && isTableRow(lines[i])) {
        bodyRows.push(splitTableRow(lines[i]));
        i++;
      }

      container.appendChild(buildTable(headers, alignments, bodyRows));
      continue;
    }

    // List: consecutive lines starting with -/*/+ or "1."
    const isBullet = (l) => /^\s*[-*+]\s+/.test(l);
    const isOrdered = (l) => /^\s*\d+\.\s+/.test(l);
    if (isBullet(line) || isOrdered(line)) {
      const ordered = isOrdered(line);
      const list = document.createElement(ordered ? 'ol' : 'ul');
      list.className = (ordered ? 'list-decimal' : 'list-disc') + ' list-inside space-y-1';
      while (i < lines.length && (ordered ? isOrdered(lines[i]) : isBullet(lines[i]))) {
        const itemText = lines[i].replace(ordered ? /^\s*\d+\.\s+/ : /^\s*[-*+]\s+/, '');
        const li = document.createElement('li');
        appendInline(li, itemText);
        list.appendChild(li);
        i++;
      }
      container.appendChild(list);
      continue;
    }

    // Paragraph: this line plus any immediately-following non-blank,
    // non-special lines. The current line is always consumed unconditionally
    // (i always advances by at least 1 here) — every branch above already
    // ruled out blank/fence/heading/blockquote/list for it, so this is just
    // the fallback catch-all, and it must never be able to stall on it.
    const paraLines = [line];
    i++;
    while (
      i < lines.length &&
      lines[i].trim() !== '' &&
      !lines[i].trim().startsWith('```') &&
      !lines[i].match(/^#{1,6}\s+/) &&
      !lines[i].trim().startsWith('>') &&
      !isBullet(lines[i]) &&
      !isOrdered(lines[i]) &&
      !/^\s*([-*_])\1{2,}\s*$/.test(lines[i]) &&
      !(isTableRow(lines[i]) && i + 1 < lines.length && isAlignmentRow(lines[i + 1]))
    ) {
      paraLines.push(lines[i]);
      i++;
    }
    const p = document.createElement('p');
    appendInline(p, paraLines.join(' '));
    container.appendChild(p);
  }
}

// ---- Tables ----

function isTableRow(line) {
  const trimmed = line.trim();
  return trimmed.startsWith('|') && trimmed.endsWith('|') && trimmed.length > 1;
}

// The row under the header: cells of dashes, optionally colon-anchored.
function isAlignmentRow(line) {
  if (!isTableRow(line)) return false;
  const cells = splitTableRow(line);
  return cells.length > 0 && cells.every((cell) => /^:?-+:?$/.test(cell));
}

function splitTableRow(line) {
  const trimmed = line.trim();
  return trimmed
    .slice(1, -1)
    .split('|')
    .map((cell) => cell.trim());
}

function alignmentOf(cell) {
  const left = cell.startsWith(':');
  const right = cell.endsWith(':');
  if (left && right) return 'center';
  if (right) return 'right';
  return 'left';
}

function alignmentClass(alignment) {
  if (alignment === 'center') return 'text-center';
  if (alignment === 'right') return 'text-right';
  return 'text-left';
}

function buildTable(headers, alignments, bodyRows) {
  // Wrapped so a wide table scrolls sideways instead of forcing the page to.
  const wrapper = document.createElement('div');
  wrapper.className = 'overflow-x-auto my-2';

  const table = document.createElement('table');
  table.className = 'min-w-full text-sm border border-hairline';

  const thead = document.createElement('thead');
  const headRow = document.createElement('tr');
  headers.forEach((cell, index) => {
    const th = document.createElement('th');
    th.className =
      'border border-hairline px-2 py-1 text-xs font-medium text-ink-dim ' +
      alignmentClass(alignments[index]);
    appendInline(th, cell);
    headRow.appendChild(th);
  });
  thead.appendChild(headRow);
  table.appendChild(thead);

  const tbody = document.createElement('tbody');
  bodyRows.forEach((row) => {
    const tr = document.createElement('tr');
    // A short row leaves trailing cells empty rather than dropping the row.
    for (let c = 0; c < headers.length; c++) {
      const td = document.createElement('td');
      td.className =
        'border border-hairline px-2 py-1 text-ink align-top ' + alignmentClass(alignments[c]);
      appendInline(td, row[c] ?? '');
      tr.appendChild(td);
    }
    tbody.appendChild(tr);
  });
  table.appendChild(tbody);

  wrapper.appendChild(table);
  return wrapper;
}

// ---- Math ----

// Commands seen in these notes, plus the neighbouring ones it would be odd to
// omit. An unknown command renders as its own name, which stays readable.
const MATH_SYMBOLS = {
  times: '×', cdot: '·', div: '÷', pm: '±', mp: '∓',
  le: '≤', leq: '≤', ge: '≥', geq: '≥', ne: '≠', neq: '≠',
  approx: '≈', equiv: '≡', sim: '∼', propto: '∝',
  rightarrow: '→', to: '→', leftarrow: '←', leftrightarrow: '↔',
  Rightarrow: '⇒', Leftarrow: '⇐', mapsto: '↦',
  sum: '∑', prod: '∏', int: '∫', sqrt: '√', infty: '∞', partial: '∂',
  in: '∈', notin: '∉', subset: '⊂', subseteq: '⊆', supset: '⊃',
  cup: '∪', cap: '∩', emptyset: '∅',
  forall: '∀', exists: '∃', neg: '¬', land: '∧', lor: '∨',
  ldots: '…', cdots: '⋯', dots: '…',
  alpha: 'α', beta: 'β', gamma: 'γ', delta: 'δ', epsilon: 'ε', zeta: 'ζ',
  eta: 'η', theta: 'θ', kappa: 'κ', lambda: 'λ', mu: 'μ', nu: 'ν', xi: 'ξ',
  rho: 'ρ', sigma: 'σ', tau: 'τ', phi: 'φ', chi: 'χ', psi: 'ψ', omega: 'ω',
  Gamma: 'Γ', Delta: 'Δ', Theta: 'Θ', Lambda: 'Λ', Pi: 'Π', Sigma: 'Σ',
  Phi: 'Φ', Psi: 'Ψ', Omega: 'Ω', pi: 'π',
};

// Function names that must stay upright rather than inheriting math italics.
const MATH_UPRIGHT = ['log', 'ln', 'exp', 'max', 'min', 'sin', 'cos', 'tan', 'gcd', 'lg', 'mod'];

// Reads a {...} group, or a single character when there are no braces —
// so both x^{10} and x^2 work.
function readMathGroup(src, start) {
  if (src[start] !== '{') {
    return { content: src[start] ?? '', next: start + 1 };
  }
  let depth = 0;
  for (let i = start; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}') {
      depth--;
      if (depth === 0) return { content: src.slice(start + 1, i), next: i + 1 };
    }
  }
  // Unbalanced brace: take the rest, so input can't hang the parser.
  return { content: src.slice(start + 1), next: src.length };
}

function appendMath(parent, src) {
  let i = 0;
  let text = '';

  const flush = () => {
    if (text) {
      parent.appendChild(document.createTextNode(text));
      text = '';
    }
  };

  while (i < src.length) {
    const ch = src[i];

    if (ch === '\\') {
      // "\\" is a row break inside environments like cases/matrix. There is
      // no 2-D layout here, so rows are flattened onto one line.
      if (src[i + 1] === '\\') {
        text += '; ';
        i += 2;
        continue;
      }
      const name = (src.slice(i + 1).match(/^[a-zA-Z]+/) || [''])[0];
      if (!name) {
        // An escaped character such as \$ or \{ is taken literally.
        text += src[i + 1] ?? '';
        i += 2;
        continue;
      }
      i += 1 + name.length;

      // \begin{cases} … \end{cases} and friends: the environment name itself
      // is markup, not content, so it is dropped rather than printed.
      if (name === 'begin' || name === 'end') {
        i = readMathGroup(src, i).next;
        continue;
      }

      if (name === 'text' || name === 'mathrm') {
        const group = readMathGroup(src, i);
        i = group.next;
        flush();
        const span = document.createElement('span');
        span.className = 'not-italic';
        span.textContent = group.content;
        parent.appendChild(span);
        continue;
      }
      if (name === 'vec') {
        const group = readMathGroup(src, i);
        i = group.next;
        text += group.content + '⃗'; // combining arrow above
        continue;
      }
      if (name === 'frac') {
        const numerator = readMathGroup(src, i);
        const denominator = readMathGroup(src, numerator.next);
        i = denominator.next;
        text += numerator.content + '⁄' + denominator.content;
        continue;
      }
      if (MATH_UPRIGHT.includes(name)) {
        flush();
        const span = document.createElement('span');
        span.className = 'not-italic';
        span.textContent = name;
        parent.appendChild(span);
        continue;
      }
      text += MATH_SYMBOLS[name] ?? name;
      continue;
    }

    if (ch === '^' || ch === '_') {
      const group = readMathGroup(src, i + 1);
      i = group.next;
      flush();
      const el = document.createElement(ch === '^' ? 'sup' : 'sub');
      appendMath(el, group.content); // nested math renders inside
      parent.appendChild(el);
      continue;
    }

    // Braces used purely for grouping contribute nothing themselves, and &
    // is a column separator in environments — both are markup, not content.
    if (ch === '{' || ch === '}' || ch === '&') {
      if (ch === '&') text += ' ';
      i++;
      continue;
    }

    text += ch;
    i++;
  }

  flush();
}

function appendMathSpan(el, latex) {
  const span = document.createElement('span');
  span.className = 'font-serif italic';
  appendMath(span, latex);
  el.appendChild(span);
}

function headingClass(level) {
  if (level <= 2) return 'text-base font-semibold text-ink mt-3 mb-1';
  return 'text-sm font-semibold text-ink mt-2 mb-1';
}

// Parses inline spans (bold, italic, inline code, links) within a line and
// appends the resulting nodes to `el`. Falls back to plain textContent for
// anything that doesn't match — never builds an HTML string.
function appendInline(el, text) {
  // Order matters. Code spans come first so ** or _ inside `code` isn't
  // touched, and math comes next for the same reason: x_i and n^2 are
  // subscript and superscript inside $...$, not italics. A $ preceded by a
  // backslash is an escaped literal and never opens a span.
  const tokenPattern =
    /(`[^`]+`)|((?<!\\)\$[^$\n]+?(?<!\\)\$)|(\*\*[^*]+\*\*)|(__[^_]+__)|(\*[^*]+\*)|(_[^_]+_)|(\[[^\]]+\]\([^)]+\))/;
  let remaining = text;

  while (remaining.length > 0) {
    const match = remaining.match(tokenPattern);
    if (!match) {
      el.appendChild(document.createTextNode(remaining));
      break;
    }
    const idx = match.index;
    if (idx > 0) {
      el.appendChild(document.createTextNode(remaining.slice(0, idx)));
    }
    const token = match[0];

    if (token.startsWith('`')) {
      const code = document.createElement('code');
      code.className = 'bg-canvas border border-hairline px-1 text-xs';
      code.textContent = token.slice(1, -1);
      el.appendChild(code);
    } else if (token.startsWith('$')) {
      appendMathSpan(el, token.slice(1, -1).trim());
    } else if (token.startsWith('**') || token.startsWith('__')) {
      const strong = document.createElement('strong');
      strong.className = 'font-semibold text-ink';
      strong.textContent = token.slice(2, -2);
      el.appendChild(strong);
    } else if (token.startsWith('*') || token.startsWith('_')) {
      const em = document.createElement('em');
      em.textContent = token.slice(1, -1);
      el.appendChild(em);
    } else if (token.startsWith('[')) {
      const linkMatch = token.match(/^\[([^\]]+)\]\(([^)]+)\)$/);
      const a = document.createElement('a');
      a.textContent = linkMatch[1];
      // Only allow http(s) targets — anything else (javascript:, data:, etc.)
      // renders as plain text instead of a clickable link.
      const href = linkMatch[2];
      if (/^https?:\/\//i.test(href)) {
        a.href = href;
        a.target = '_blank';
        a.rel = 'noopener noreferrer';
        a.className = 'text-accent hover:underline';
        el.appendChild(a);
      } else {
        el.appendChild(document.createTextNode(linkMatch[1]));
      }
    }

    remaining = remaining.slice(idx + token.length);
  }
}

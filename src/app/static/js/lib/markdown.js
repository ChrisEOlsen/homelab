// Minimal safe markdown -> DOM renderer. No HTML string ever gets built or
// parsed — every node is created with createElement/textContent, so there is
// no injection surface and no need for a sanitizer. Supports: headings,
// bold/italic, inline code, fenced code blocks, unordered/ordered lists,
// links, blockquotes, and paragraphs. Anything unsupported just renders as
// plain text, which is the safe failure mode.

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
      !isOrdered(lines[i])
    ) {
      paraLines.push(lines[i]);
      i++;
    }
    const p = document.createElement('p');
    appendInline(p, paraLines.join(' '));
    container.appendChild(p);
  }
}

function headingClass(level) {
  if (level <= 2) return 'text-base font-semibold text-ink mt-3 mb-1';
  return 'text-sm font-semibold text-ink mt-2 mb-1';
}

// Parses inline spans (bold, italic, inline code, links) within a line and
// appends the resulting nodes to `el`. Falls back to plain textContent for
// anything that doesn't match — never builds an HTML string.
function appendInline(el, text) {
  // Order matters: code spans first so ** or _ inside `code` isn't touched.
  const tokenPattern = /(`[^`]+`)|(\*\*[^*]+\*\*)|(__[^_]+__)|(\*[^*]+\*)|(_[^_]+_)|(\[[^\]]+\]\([^)]+\))/;
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

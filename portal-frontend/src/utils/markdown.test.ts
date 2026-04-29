import { describe, it, expect } from 'vitest';
import { renderMarkdown, parseFrontmatter } from './markdown';

describe('renderMarkdown', () => {
  it('renders headings to HTML', async () => {
    const out = await renderMarkdown('# Hello');
    expect(out).toContain('<h1>Hello</h1>');
  });

  it('renders GFM tables', async () => {
    const md = '| a | b |\n|---|---|\n| 1 | 2 |';
    const out = await renderMarkdown(md);
    expect(out).toContain('<table>');
    expect(out).toContain('<th>a</th>');
  });

  it('renders fenced code with syntax highlighting classes', async () => {
    const out = await renderMarkdown('```js\nconst x = 1;\n```');
    expect(out).toMatch(/<code[^>]*class="[^"]*hljs[^"]*"/);
  });
});

describe('parseFrontmatter', () => {
  it('returns empty data when no frontmatter block', () => {
    const { data, content } = parseFrontmatter('just text');
    expect(data).toEqual({});
    expect(content).toBe('just text');
  });

  it('parses scalar and quoted values', () => {
    const raw = '---\ntitle: "Hello World"\nslug: my-post\n---\nBody';
    const { data, content } = parseFrontmatter(raw);
    expect(data.title).toBe('Hello World');
    expect(data.slug).toBe('my-post');
    expect(content).toBe('Body');
  });

  it('parses JSON array values', () => {
    const raw = '---\ntags: ["api","sms"]\n---\nx';
    const { data } = parseFrontmatter(raw);
    expect(data.tags).toEqual(['api', 'sms']);
  });

  it('leaves malformed JSON arrays as raw strings', () => {
    const raw = '---\ntags: [bad\n---\nx';
    const { data } = parseFrontmatter(raw);
    expect(data.tags).toBe('[bad');
  });
});

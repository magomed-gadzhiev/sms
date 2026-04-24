import { describe, it, expect, vi, beforeEach } from 'vitest';
import { exportToCsv } from './csvExport';

describe('exportToCsv', () => {
  beforeEach(() => {
    // jsdom lacks URL.createObjectURL; stub both.
    const createdUrls: string[] = [];
    (URL as unknown as { createObjectURL: (b: Blob) => string }).createObjectURL = vi.fn((b: Blob) => {
      createdUrls.push('blob:' + String(createdUrls.length));
      return 'blob:' + String(createdUrls.length - 1);
    });
    (URL as unknown as { revokeObjectURL: (u: string) => void }).revokeObjectURL = vi.fn();
  });

  it('creates an anchor, clicks it, and cleans it up', () => {
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    exportToCsv('report.csv', ['a', 'b'], [[1, 2]]);
    expect(clickSpy).toHaveBeenCalledOnce();
    // After export, the anchor must be removed from body.
    expect(document.body.querySelector('a[download="report.csv"]')).toBeNull();
    expect(URL.revokeObjectURL).toHaveBeenCalledOnce();
  });

  it('escapes double quotes by doubling them', () => {
    const blobs: Blob[] = [];
    (URL as unknown as { createObjectURL: (b: Blob) => string }).createObjectURL = vi.fn((b: Blob) => {
      blobs.push(b);
      return 'blob:mock';
    });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    exportToCsv('x.csv', ['col'], [['value "quoted" here']]);

    // Read the blob — it must contain "" for each original ".
    return blobs[0].text().then((text) => {
      expect(text).toContain('"value ""quoted"" here"');
    });
  });

  it('renders null/undefined cells as empty strings', () => {
    const blobs: Blob[] = [];
    (URL as unknown as { createObjectURL: (b: Blob) => string }).createObjectURL = vi.fn((b: Blob) => {
      blobs.push(b);
      return 'blob:mock';
    });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    exportToCsv('x.csv', ['a', 'b', 'c'], [[null, undefined, 'x']]);

    return blobs[0].text().then((text) => {
      expect(text).toContain('"","","x"');
    });
  });

  it('includes UTF-8 BOM so Excel opens Cyrillic correctly', async () => {
    const blobs: Blob[] = [];
    (URL as unknown as { createObjectURL: (b: Blob) => string }).createObjectURL = vi.fn((b: Blob) => {
      blobs.push(b);
      return 'blob:mock';
    });
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});

    exportToCsv('x.csv', ['имя'], [['тест']]);
    // Inspect raw bytes — .text() strips the BOM when decoding UTF-8.
    const buf = await blobs[0].arrayBuffer();
    const bytes = new Uint8Array(buf);
    expect(bytes[0]).toBe(0xef);
    expect(bytes[1]).toBe(0xbb);
    expect(bytes[2]).toBe(0xbf);
  });

  it('logs an error but still revokes object URL when click throws', () => {
    vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {
      throw new Error('click blocked');
    });
    const errSpy = vi.spyOn(console, 'error').mockImplementation(() => {});
    exportToCsv('x.csv', ['a'], [[1]]);
    expect(errSpy).toHaveBeenCalled();
    expect(URL.revokeObjectURL).toHaveBeenCalledOnce();
  });
});

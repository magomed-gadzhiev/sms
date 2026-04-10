export function exportToCsv(filename: string, headers: string[], rows: (string | number | null | undefined)[][]): void {
  const escape = (cell: string | number | null | undefined) => {
    const str = cell == null ? '' : String(cell);
    return `"${str.replace(/"/g, '""')}"`;
  };
  const csv = [
    headers.map(escape).join(','),
    ...rows.map(r => r.map(escape).join(','))
  ].join('\n');
  const blob = new Blob(['\uFEFF' + csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  try {
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  } catch (error) {
    console.error('CSV export failed:', error);
  } finally {
    URL.revokeObjectURL(url);
  }
}

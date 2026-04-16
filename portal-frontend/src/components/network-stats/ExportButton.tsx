import { useState, useRef, useEffect } from 'react';
import { Download, Loader2 } from 'lucide-react';

interface ExportButtonProps {
  onExport: (format: 'csv' | 'xlsx') => void;
  status: { status: string; job_id: string } | null;
}

export function ExportButton({ onExport, status }: ExportButtonProps) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  const isProcessing = status?.status === 'pending' || status?.status === 'processing';

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(v => !v)}
        disabled={isProcessing}
        className="flex items-center gap-1 px-2.5 py-1 rounded-md text-xs border border-gray-200 text-gray-500 hover:border-gray-300 disabled:opacity-50"
      >
        {isProcessing ? <Loader2 size={14} className="animate-spin" /> : <Download size={14} />}
        {isProcessing ? 'Экспорт...' : 'Экспорт'}
      </button>
      {open && !isProcessing && (
        <div className="absolute right-0 mt-1 w-32 rounded-md border border-gray-200 bg-white shadow-lg z-10">
          <button onClick={() => { onExport('csv'); setOpen(false); }} className="block w-full text-left px-3 py-2 text-xs hover:bg-gray-50">CSV</button>
          <button onClick={() => { onExport('xlsx'); setOpen(false); }} className="block w-full text-left px-3 py-2 text-xs hover:bg-gray-50">XLSX</button>
        </div>
      )}
    </div>
  );
}

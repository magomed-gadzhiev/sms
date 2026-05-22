import React from 'react';

interface SmsPreviewProps {
  sender: string;
  body: string;
  timestamp: string;
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  } catch {
    return '';
  }
}

export const SmsPreview: React.FC<SmsPreviewProps> = ({ sender, body, timestamp }) => (
  <div className="flex justify-center py-2">
    <div
      className="w-56 rounded-3xl border-4 border-gray-800 bg-gray-900 p-3 shadow-xl"
      style={{ minHeight: '180px' }}
    >
      {/* Status bar */}
      <div className="flex justify-between items-center mb-3 px-1">
        <span className="text-white text-xs font-semibold">SMS</span>
        <div className="flex gap-1">
          <div className="w-1 h-1 bg-white dark:bg-slate-900 rounded-full" />
          <div className="w-1 h-1 bg-white dark:bg-slate-900 rounded-full" />
          <div className="w-1 h-1 bg-white dark:bg-slate-900 rounded-full" />
        </div>
      </div>
      {/* Sender */}
      <p className="text-center text-xs text-gray-400 dark:text-slate-500 mb-2">{sender}</p>
      {/* Bubble */}
      <div className="bg-gray-700 rounded-2xl rounded-tl-sm p-3">
        <p className="text-white text-xs leading-relaxed break-words">{body}</p>
        <p className="text-gray-400 dark:text-slate-500 text-right text-xs mt-1">{formatTime(timestamp)}</p>
      </div>
    </div>
  </div>
);

// portal-frontend/src/components/layout/ModeSwitcher.tsx
import { useNavigate } from 'react-router-dom';

const LS_KEY = 'reseller_mode';

export type ResellerMode = 'own' | 'network';

export function getResellerMode(): ResellerMode {
  return (localStorage.getItem(LS_KEY) as ResellerMode) || 'own';
}

export function setResellerMode(mode: ResellerMode) {
  localStorage.setItem(LS_KEY, mode);
}

interface ModeSwitcherProps {
  currentMode: ResellerMode;
}

export function ModeSwitcher({ currentMode }: ModeSwitcherProps) {
  const navigate = useNavigate();

  function toggle() {
    if (currentMode === 'own') {
      setResellerMode('network');
      navigate('/network/dashboard');
    } else {
      setResellerMode('own');
      navigate('/command-center');
    }
  }

  const isNetwork = currentMode === 'network';

  return (
    <button
      onClick={toggle}
      className="flex items-center gap-2 w-full px-3 py-2.5 rounded-lg text-sm font-medium transition-colors border border-gray-200 hover:bg-gray-100"
      title={isNetwork ? 'Перейти к своему аккаунту' : 'Управление сетью'}
    >
      {isNetwork ? (
        <>
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
          <span>Свой аккаунт</span>
          <span className="ml-auto text-gray-400">&larr;</span>
        </>
      ) : (
        <>
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span>Управление сетью</span>
          <span className="ml-auto text-gray-400">&rarr;</span>
        </>
      )}
    </button>
  );
}

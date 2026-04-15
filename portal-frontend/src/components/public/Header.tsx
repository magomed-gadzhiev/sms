import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Menu, X } from 'lucide-react';

export function Header() {
  const { t, i18n } = useTranslation();
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);
  const isEn = location.pathname.startsWith('/en');
  const prefix = isEn ? '/en' : '';

  const links = [
    { to: `${prefix}/features`, label: t('nav.features') },
    { to: `${prefix}/pricing`, label: t('nav.pricing') },
    { to: `${prefix}/docs`, label: t('nav.docs') },
    { to: `${prefix}/blog`, label: t('nav.blog') },
    { to: `${prefix}/about`, label: t('nav.about') },
  ];

  function toggleLang() {
    const newLang = isEn ? 'ru' : 'en';
    i18n.changeLanguage(newLang);
    localStorage.setItem('lang', newLang);
    const path = location.pathname.replace(/^\/en/, '') || '/';
    if (newLang === 'en') {
      window.location.href = `/en${path}`;
    } else {
      window.location.href = path;
    }
  }

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-100">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-between h-16">
        <Link to={prefix || '/'} className="text-xl font-bold bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent">
          SMS Platform
        </Link>
        <nav className="hidden md:flex items-center gap-6">
          {links.map((l) => (
            <Link key={l.to} to={l.to} className="text-sm text-gray-600 hover:text-indigo-600 transition-colors">{l.label}</Link>
          ))}
        </nav>
        <div className="hidden md:flex items-center gap-3">
          <button onClick={toggleLang} className="text-sm text-gray-500 hover:text-gray-700 px-2 py-1 rounded">{isEn ? 'RU' : 'EN'}</button>
          <Link to="/login" className="text-sm text-gray-600 hover:text-indigo-600">{t('nav.login')}</Link>
          <Link to="/register" className="text-sm bg-gradient-to-r from-indigo-600 to-purple-600 text-white px-4 py-2 rounded-xl hover:opacity-90 transition-opacity">{t('nav.getStarted')}</Link>
        </div>
        <button onClick={() => setMobileOpen(!mobileOpen)} className="md:hidden text-gray-600" aria-label="Menu">
          {mobileOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
      </div>
      {mobileOpen && (
        <div className="md:hidden bg-white border-t border-gray-100 px-4 pb-4">
          {links.map((l) => (
            <Link key={l.to} to={l.to} onClick={() => setMobileOpen(false)} className="block py-2 text-gray-600 hover:text-indigo-600">{l.label}</Link>
          ))}
          <div className="flex items-center gap-3 pt-3 border-t border-gray-100 mt-2">
            <button onClick={toggleLang} className="text-sm text-gray-500">{isEn ? 'RU' : 'EN'}</button>
            <Link to="/login" className="text-sm text-gray-600">{t('nav.login')}</Link>
            <Link to="/register" className="text-sm bg-gradient-to-r from-indigo-600 to-purple-600 text-white px-4 py-2 rounded-xl">{t('nav.getStarted')}</Link>
          </div>
        </div>
      )}
    </header>
  );
}

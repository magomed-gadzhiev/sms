import { Outlet, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Header } from '../public/Header';
import { Footer } from '../public/Footer';

export function PublicLayout() {
  const { i18n } = useTranslation();
  const location = useLocation();

  useEffect(() => {
    const isEn = location.pathname.startsWith('/en');
    const lang = isEn ? 'en' : 'ru';
    if (i18n.language !== lang) {
      i18n.changeLanguage(lang);
      localStorage.setItem('lang', lang);
    }
    document.documentElement.lang = lang;
  }, [location.pathname, i18n]);

  return (
    <div className="min-h-screen flex flex-col">
      <Header />
      <main className="flex-1 pt-16"><Outlet /></main>
      <Footer />
    </div>
  );
}

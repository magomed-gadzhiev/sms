import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function Footer() {
  const { t } = useTranslation();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  const columns = [
    { title: t('footer.product'), links: [
      { to: `${prefix}/features`, label: t('nav.features') },
      { to: `${prefix}/pricing`, label: t('nav.pricing') },
      { to: `${prefix}/docs`, label: t('nav.docs') },
    ]},
    { title: t('footer.company'), links: [
      { to: `${prefix}/about`, label: t('nav.about') },
      { to: `${prefix}/blog`, label: t('nav.blog') },
      { to: `${prefix}/contact`, label: t('nav.contact') },
    ]},
    { title: t('footer.legal'), links: [
      { to: '#', label: t('footer.terms') },
      { to: '#', label: t('footer.privacy') },
    ]},
  ];

  return (
    <footer className="bg-gray-900 text-gray-300">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
          {columns.map((col) => (
            <div key={col.title}>
              <h4 className="text-sm font-semibold text-white mb-3">{col.title}</h4>
              <ul className="space-y-2">
                {col.links.map((l) => (
                  <li key={l.to + l.label}><Link to={l.to} className="text-sm text-gray-400 hover:text-white transition-colors">{l.label}</Link></li>
                ))}
              </ul>
            </div>
          ))}
          <div>
            <h4 className="text-sm font-semibold text-white mb-3">{t('footer.support')}</h4>
            <ul className="space-y-2">
              <li><a href="https://t.me/sms_support" target="_blank" rel="noopener noreferrer" className="text-sm text-gray-400 hover:text-white transition-colors">{t('footer.telegram')}</a></li>
              <li><a href="mailto:support@smsplatform.com" className="text-sm text-gray-400 hover:text-white transition-colors">{t('footer.email')}</a></li>
            </ul>
          </div>
        </div>
        <div className="border-t border-gray-800 mt-10 pt-6 text-center text-sm text-gray-500">{t('footer.copyright')}</div>
      </div>
    </footer>
  );
}

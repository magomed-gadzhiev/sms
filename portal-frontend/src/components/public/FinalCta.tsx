import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function FinalCta() {
  const { t } = useTranslation();
  const location = useLocation();
  const prefix = location.pathname.startsWith('/en') ? '/en' : '';

  return (
    <section className="relative overflow-hidden bg-gradient-to-br from-indigo-900 via-indigo-700 to-purple-600 py-20">
      {/* Decorative radial gradients */}
      <div className="pointer-events-none absolute inset-0">
        <div className="absolute -top-20 -right-20 h-80 w-80 rounded-full bg-white/10 blur-3xl" />
        <div className="absolute -bottom-20 -left-20 h-80 w-80 rounded-full bg-purple-400/15 blur-3xl" />
      </div>

      <div className="relative mx-auto max-w-3xl px-4 text-center sm:px-6 lg:px-8">
        <h2 className="text-3xl font-bold text-white sm:text-4xl">
          {t('finalCta.title')}
        </h2>
        <p className="mx-auto mt-4 max-w-xl text-lg text-indigo-100">
          {t('finalCta.subtitle')}
        </p>
        <Link
          to={`${prefix}/register`}
          className="mt-8 inline-flex items-center rounded-xl bg-white px-8 py-3 text-base font-semibold text-indigo-700 shadow-lg transition-opacity hover:opacity-90"
        >
          {t('finalCta.cta')}
        </Link>
      </div>
    </section>
  );
}

import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Helmet } from 'react-helmet-async';

export function Hero() {
  const { t } = useTranslation();
  const location = useLocation();
  const prefix = location.pathname.startsWith('/en') ? '/en' : '';

  return (
    <>
      <Helmet>
        <title>{t('seo.landing.title')}</title>
        <meta name="description" content={t('seo.landing.desc')} />
        <meta property="og:title" content={t('seo.landing.title')} />
        <meta property="og:description" content={t('seo.landing.desc')} />
        <meta property="og:type" content="website" />
        <meta name="twitter:card" content="summary_large_image" />
        <meta name="twitter:title" content={t('seo.landing.title')} />
        <meta name="twitter:description" content={t('seo.landing.desc')} />
      </Helmet>

      <section className="relative overflow-hidden bg-gradient-to-br from-indigo-900 via-indigo-700 to-purple-600 pt-32 pb-20 sm:pt-40 sm:pb-28">
        {/* Decorative radial gradients */}
        <div className="pointer-events-none absolute inset-0">
          <div className="absolute -top-24 -left-24 h-96 w-96 rounded-full bg-white/10 blur-3xl" />
          <div className="absolute -bottom-32 -right-32 h-[28rem] w-[28rem] rounded-full bg-purple-400/20 blur-3xl" />
          <div className="absolute top-1/2 left-1/2 h-64 w-64 -translate-x-1/2 -translate-y-1/2 rounded-full bg-indigo-400/10 blur-2xl" />
        </div>

        <div className="relative mx-auto max-w-4xl px-4 text-center sm:px-6 lg:px-8">
          <h1 className="text-4xl font-extrabold tracking-tight text-white sm:text-5xl lg:text-6xl">
            {t('hero.title')}
          </h1>
          <p className="mx-auto mt-6 max-w-2xl text-lg text-indigo-100 sm:text-xl">
            {t('hero.subtitle')}
          </p>
          <div className="mt-10 flex flex-col items-center justify-center gap-4 sm:flex-row">
            <Link
              to={`${prefix}/register`}
              className="inline-flex items-center rounded-xl bg-white px-8 py-3 text-base font-semibold text-indigo-700 shadow-lg transition-opacity hover:opacity-90"
            >
              {t('hero.cta')}
            </Link>
            <Link
              to={`${prefix}/docs`}
              className="inline-flex items-center rounded-xl border-2 border-white/30 px-8 py-3 text-base font-semibold text-white backdrop-blur-sm transition-colors hover:border-white/60 hover:bg-white/10"
            >
              {t('hero.docs')}
            </Link>
          </div>
        </div>
      </section>
    </>
  );
}

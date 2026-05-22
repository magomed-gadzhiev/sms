import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { FinalCta } from '../../components/public/FinalCta';

const stats = [
  { value: '500M+', key: 'sms' },
  { value: '99.9%', key: 'uptime' },
  { value: '50+', key: 'providers' },
  { value: '200+', key: 'resellers' },
] as const;

export function AboutPage() {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('seo.about.title')}</title>
        <meta name="description" content={t('seo.about.desc')} />
      </Helmet>

      <section className="bg-gradient-to-b from-indigo-50 to-white pb-12 pt-16">
        <div className="mx-auto max-w-4xl px-4 text-center">
          <h1 className="mb-6 text-4xl font-bold text-gray-900">{t('about.title')}</h1>
          <p className="mx-auto max-w-2xl text-lg leading-relaxed text-gray-600">
            {t('about.mission')}
          </p>
          <p className="mx-auto mt-4 max-w-2xl text-lg leading-relaxed text-gray-600">
            {t('about.mission2')}
          </p>
        </div>
      </section>

      <section className="bg-gray-50 py-16">
        <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
          <h2 className="mb-10 text-center text-2xl font-bold text-gray-900">
            {t('about.statsTitle')}
          </h2>
          <div className="grid grid-cols-2 gap-8 lg:grid-cols-4">
            {stats.map((s) => (
              <div key={s.key} className="text-center">
                <p className="bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-3xl font-extrabold text-transparent sm:text-4xl">
                  {s.value}
                </p>
                <p className="mt-2 text-sm font-medium text-gray-500 sm:text-base">
                  {t(`stats.${s.key}`)}
                </p>
              </div>
            ))}
          </div>
        </div>
      </section>

      <FinalCta />
    </>
  );
}

import { useTranslation } from 'react-i18next';

export function Screenshot() {
  const { t } = useTranslation();

  return (
    <section className="py-20">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        <h2 className="text-center text-3xl font-bold text-gray-900 sm:text-4xl">
          {t('screenshot.title')}
        </h2>

        <div className="mt-12 overflow-hidden rounded-2xl bg-gradient-to-br from-gray-200 via-gray-100 to-gray-200 shadow-xl">
          <div className="flex h-80 items-center justify-center sm:h-[28rem]">
            <p className="text-lg text-gray-400">{t('screenshot.alt')}</p>
          </div>
        </div>
      </div>
    </section>
  );
}

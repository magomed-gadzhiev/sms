import { useTranslation } from 'react-i18next';

const stats = [
  { value: '500M+', key: 'sms' },
  { value: '99.9%', key: 'uptime' },
  { value: '50+', key: 'providers' },
  { value: '200+', key: 'resellers' },
] as const;

export function Stats() {
  const { t } = useTranslation();

  return (
    <section className="bg-gray-50 py-16">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        <div className="grid grid-cols-2 gap-8 lg:grid-cols-4">
          {stats.map((s) => (
            <div key={s.key} className="text-center">
              <p className="text-3xl font-extrabold bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent sm:text-4xl">
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
  );
}

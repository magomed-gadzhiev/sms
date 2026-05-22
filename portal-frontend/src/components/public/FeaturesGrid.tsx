import { useTranslation } from 'react-i18next';
import { Plug, Users, GitBranch, BarChart3, CreditCard, Webhook } from 'lucide-react';
import type { LucideIcon } from 'lucide-react';

const features: { key: string; Icon: LucideIcon }[] = [
  { key: 'providers', Icon: Plug },
  { key: 'multiTenant', Icon: Users },
  { key: 'routing', Icon: GitBranch },
  { key: 'analytics', Icon: BarChart3 },
  { key: 'billing', Icon: CreditCard },
  { key: 'api', Icon: Webhook },
];

export function FeaturesGrid() {
  const { t } = useTranslation();

  return (
    <section className="py-20">
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
        <div className="text-center">
          <h2 className="text-3xl font-bold text-gray-900 sm:text-4xl">{t('features.title')}</h2>
          <p className="mx-auto mt-4 max-w-2xl text-lg text-gray-500">{t('features.subtitle')}</p>
        </div>

        <div className="mt-16 grid gap-8 sm:grid-cols-2 lg:grid-cols-3">
          {features.map(({ key, Icon }) => (
            <div
              key={key}
              className="rounded-2xl bg-gray-50 p-6 transition-shadow hover:shadow-lg"
            >
              <div className="inline-flex h-12 w-12 items-center justify-center rounded-xl bg-gradient-to-br from-indigo-600 to-purple-600">
                <Icon className="h-6 w-6 text-white" />
              </div>
              <h3 className="mt-4 text-lg font-semibold text-gray-900">
                {t(`features.${key}.title`)}
              </h3>
              <p className="mt-2 text-gray-500">
                {t(`features.${key}.desc`)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

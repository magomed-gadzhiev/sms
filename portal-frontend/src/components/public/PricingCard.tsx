import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Check } from 'lucide-react';

interface PricingCardProps {
  plan: 'starter' | 'business' | 'enterprise';
}

export function PricingCard({ plan }: PricingCardProps) {
  const { t } = useTranslation();
  const location = useLocation();
  const prefix = location.pathname.startsWith('/en') ? '/en' : '';

  const highlighted = plan === 'business';
  const features = t(`pricing.${plan}.features`, { returnObjects: true }) as string[];

  return (
    <div
      className={`relative flex flex-col rounded-2xl bg-white p-8 ${
        highlighted
          ? 'border-2 border-indigo-500 shadow-xl'
          : 'border border-gray-200 shadow-sm'
      }`}
    >
      {highlighted && (
        <span className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-gradient-to-r from-indigo-600 to-purple-600 px-4 py-1 text-xs font-semibold text-white">
          Popular
        </span>
      )}

      <h3 className="text-xl font-bold text-gray-900">
        {t(`pricing.${plan}.name`)}
      </h3>
      <p className="mt-1 text-sm text-gray-500">
        {t(`pricing.${plan}.desc`)}
      </p>

      <p className="mt-6">
        <span className="text-4xl font-extrabold text-gray-900">
          {t(`pricing.${plan}.price`)}
        </span>
      </p>

      <ul className="mt-6 space-y-2 text-sm text-gray-600">
        <li>{t(`pricing.${plan}.smsLimit`)} SMS</li>
        <li>{t(`pricing.${plan}.subAccounts`)}</li>
        <li>{t(`pricing.${plan}.providerLimit`)}</li>
      </ul>

      <hr className="my-6 border-gray-100" />

      <ul className="flex-1 space-y-3">
        {features.map((feature) => (
          <li key={feature} className="flex items-start gap-2 text-sm text-gray-700">
            <Check className="mt-0.5 h-4 w-4 shrink-0 text-indigo-600" />
            {feature}
          </li>
        ))}
      </ul>

      <Link
        to={`${prefix}/register`}
        className={`mt-8 block rounded-xl py-3 text-center text-sm font-semibold transition-opacity hover:opacity-90 ${
          highlighted
            ? 'bg-gradient-to-r from-indigo-600 to-purple-600 text-white'
            : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
        }`}
      >
        {plan === 'enterprise' ? t('finalCta.cta') : t('pricingPreview.cta')}
      </Link>
    </div>
  );
}

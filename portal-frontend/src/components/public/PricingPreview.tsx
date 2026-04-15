import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const plans = [
  { key: 'starter', price: '$49', sms: '50,000', highlighted: false },
  { key: 'business', price: '$199', sms: '500,000', highlighted: true },
  { key: 'enterprise', price: null, sms: 'Unlimited', highlighted: false },
] as const;

export function PricingPreview() {
  const { t } = useTranslation();
  const location = useLocation();
  const prefix = location.pathname.startsWith('/en') ? '/en' : '';

  return (
    <section className="bg-gray-50 py-20">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        <h2 className="text-center text-3xl font-bold text-gray-900 sm:text-4xl">
          {t('pricingPreview.title')}
        </h2>

        <div className="mt-14 grid gap-8 md:grid-cols-3">
          {plans.map((plan) => (
            <div
              key={plan.key}
              className={`relative rounded-2xl bg-white p-8 shadow-sm transition-shadow hover:shadow-lg ${
                plan.highlighted ? 'ring-2 ring-indigo-600' : ''
              }`}
            >
              {plan.highlighted && (
                <span className="absolute -top-3 left-1/2 -translate-x-1/2 rounded-full bg-gradient-to-r from-indigo-600 to-purple-600 px-4 py-1 text-xs font-semibold text-white">
                  {t('pricingPreview.popular')}
                </span>
              )}
              <h3 className="text-xl font-bold text-gray-900">
                {t(`pricing.${plan.key}.name`)}
              </h3>
              <p className="mt-4">
                <span className="text-4xl font-extrabold text-gray-900">
                  {plan.price ?? t('pricingPreview.custom')}
                </span>
                {plan.price && (
                  <span className="text-gray-500">{t('pricingPreview.perMonth')}</span>
                )}
              </p>
              <p className="mt-2 text-sm text-gray-500">
                {plan.sms} {t('pricingPreview.smsIncluded')}
              </p>
              <Link
                to={`${prefix}/register`}
                className={`mt-8 block rounded-xl py-3 text-center text-sm font-semibold transition-opacity hover:opacity-90 ${
                  plan.highlighted
                    ? 'bg-gradient-to-r from-indigo-600 to-purple-600 text-white'
                    : 'bg-gray-100 text-gray-700 hover:bg-gray-200'
                }`}
              >
                {t('pricingPreview.cta')}
              </Link>
            </div>
          ))}
        </div>

        <div className="mt-10 text-center">
          <Link
            to={`${prefix}/pricing`}
            className="text-sm font-medium text-indigo-600 hover:text-indigo-700"
          >
            {t('pricingPreview.moreDetails')}
          </Link>
        </div>
      </div>
    </section>
  );
}

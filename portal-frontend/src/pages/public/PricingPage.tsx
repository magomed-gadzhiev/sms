import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { PricingCard } from '../../components/public/PricingCard';
import { PricingTable } from '../../components/public/PricingTable';
import { PricingCalculator } from '../../components/public/PricingCalculator';
import { PricingFaq } from '../../components/public/PricingFaq';
import { FinalCta } from '../../components/public/FinalCta';

const plans = ['starter', 'business', 'enterprise'] as const;

export function PricingPage() {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('pricing.title')} | SMS Gateway</title>
        <meta name="description" content={t('pricing.subtitle')} />
      </Helmet>

      {/* Hero */}
      <section className="bg-gradient-to-br from-indigo-900 via-indigo-700 to-purple-600 py-20 text-center text-white">
        <div className="mx-auto max-w-3xl px-4 sm:px-6 lg:px-8">
          <h1 className="text-4xl font-bold sm:text-5xl">{t('pricing.title')}</h1>
          <p className="mt-4 text-lg text-indigo-100">{t('pricing.subtitle')}</p>
        </div>
      </section>

      {/* Pricing Cards */}
      <section className="py-20">
        <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
          <div className="grid gap-8 md:grid-cols-3">
            {plans.map((plan) => (
              <PricingCard key={plan} plan={plan} />
            ))}
          </div>
        </div>
      </section>

      <PricingTable />
      <PricingCalculator />
      <PricingFaq />
      <FinalCta />
    </>
  );
}

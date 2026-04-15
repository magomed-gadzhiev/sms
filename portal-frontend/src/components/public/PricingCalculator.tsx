import { useState } from 'react';
import { useTranslation } from 'react-i18next';

export function PricingCalculator() {
  const { t } = useTranslation();
  const rate = t('pricing.overageRate', { returnObjects: true }) as unknown as number;
  const [volume, setVolume] = useState(100_000);

  const cost = (volume * rate).toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0,
  });

  const volumeLabel =
    volume >= 1_000_000
      ? `${(volume / 1_000_000).toFixed(1)}M`
      : `${(volume / 1_000).toFixed(0)}K`;

  return (
    <section className="py-20">
      <div className="mx-auto max-w-2xl px-4 sm:px-6 lg:px-8">
        <h2 className="text-center text-3xl font-bold text-gray-900 sm:text-4xl">
          {t('pricing.calculatorTitle')}
        </h2>

        <div className="mt-12 rounded-2xl border border-gray-200 bg-white p-8 shadow-sm">
          <label className="block text-sm font-medium text-gray-700">
            {t('pricing.calculatorLabel')}
          </label>

          <input
            type="range"
            min={10_000}
            max={2_000_000}
            step={10_000}
            value={volume}
            onChange={(e) => setVolume(Number(e.target.value))}
            className="mt-4 w-full accent-indigo-600"
          />

          <div className="mt-4 flex items-center justify-between text-sm text-gray-500">
            <span>10K</span>
            <span className="text-lg font-bold text-gray-900">{volumeLabel}</span>
            <span>2M</span>
          </div>

          <div className="mt-6 rounded-xl bg-indigo-50 p-4 text-center">
            <p className="text-sm text-gray-600">{t('pricing.calculatorResult')}</p>
            <p className="mt-1 text-3xl font-extrabold text-indigo-700">{cost}</p>
          </div>
        </div>
      </div>
    </section>
  );
}

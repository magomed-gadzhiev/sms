import { useTranslation } from 'react-i18next';

const steps = ['step1', 'step2', 'step3'] as const;

export function HowItWorks() {
  const { t } = useTranslation();

  return (
    <section className="bg-gray-50 py-20">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        <h2 className="text-center text-3xl font-bold text-gray-900 sm:text-4xl">
          {t('howItWorks.title')}
        </h2>

        <div className="mt-16 grid gap-10 md:grid-cols-3">
          {steps.map((step, i) => (
            <div key={step} className="text-center">
              <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-full bg-gradient-to-br from-indigo-600 to-purple-600 text-xl font-bold text-white shadow-lg">
                {i + 1}
              </div>
              <h3 className="mt-6 text-lg font-semibold text-gray-900">
                {t(`howItWorks.${step}.title`)}
              </h3>
              <p className="mt-2 text-gray-500">
                {t(`howItWorks.${step}.desc`)}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

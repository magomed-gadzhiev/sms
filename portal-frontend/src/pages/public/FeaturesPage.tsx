import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Plug, Users, GitBranch, Layers, Megaphone, Code, BarChart3, CreditCard, Tag, Search } from 'lucide-react';
import { FinalCta } from '../../components/public/FinalCta';

const ICONS = [Plug, Users, GitBranch, Layers, Megaphone, Code, BarChart3, CreditCard, Tag, Search];

export function FeaturesPage() {
  const { t } = useTranslation();
  const items = t('featuresPage.items', { returnObjects: true }) as { title: string; desc: string }[];

  return (
    <>
      <Helmet>
        <title>{t('seo.features.title')}</title>
        <meta name="description" content={t('seo.features.desc')} />
      </Helmet>
      <section className="pt-16 pb-12 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-4xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-3">{t('featuresPage.title')}</h1>
          <p className="text-gray-500">{t('featuresPage.subtitle')}</p>
        </div>
      </section>
      <section className="py-16">
        <div className="max-w-5xl mx-auto px-4 space-y-20">
          {items.map((item, i) => {
            const Icon = ICONS[i];
            const reversed = i % 2 === 1;
            return (
              <div key={i} className={`flex flex-col md:flex-row items-center gap-10 ${reversed ? 'md:flex-row-reverse' : ''}`}>
                <div className="flex-1">
                  <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-500 flex items-center justify-center mb-4">
                    <Icon size={24} className="text-white" />
                  </div>
                  <h3 className="text-2xl font-bold text-gray-900 mb-3">{item.title}</h3>
                  <p className="text-gray-600 leading-relaxed">{item.desc}</p>
                </div>
                <div className="flex-1 w-full">
                  <div className="bg-gradient-to-br from-gray-100 to-gray-200 rounded-2xl p-12 text-gray-400 text-center">
                    [ Screenshot ]
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </section>
      <FinalCta />
    </>
  );
}

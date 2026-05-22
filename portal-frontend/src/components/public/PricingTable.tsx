import { useTranslation } from 'react-i18next';
import { Check, X } from 'lucide-react';

interface Row {
  label: string;
  starter: boolean;
  business: boolean;
  enterprise: boolean;
}

const rows: Row[] = [
  { label: 'API + Web Panel',       starter: true,  business: true,  enterprise: true },
  { label: 'Email Support',         starter: true,  business: true,  enterprise: true },
  { label: 'Rule-based Routing',    starter: false, business: true,  enterprise: true },
  { label: 'Cascade Delivery',      starter: false, business: true,  enterprise: true },
  { label: 'Campaigns',             starter: false, business: true,  enterprise: true },
  { label: 'Analytics + Export',    starter: false, business: true,  enterprise: true },
  { label: 'Priority Support',      starter: false, business: true,  enterprise: true },
  { label: 'SMPP Access',           starter: false, business: false, enterprise: true },
  { label: 'HLR Lookup',            starter: false, business: false, enterprise: true },
  { label: 'Dedicated Manager',     starter: false, business: false, enterprise: true },
  { label: 'Custom SLA',            starter: false, business: false, enterprise: true },
];

function BoolIcon({ value }: { value: boolean }) {
  return value ? (
    <Check className="mx-auto h-5 w-5 text-indigo-600" />
  ) : (
    <X className="mx-auto h-5 w-5 text-gray-300" />
  );
}

export function PricingTable() {
  const { t } = useTranslation();

  return (
    <section className="py-20">
      <div className="mx-auto max-w-5xl px-4 sm:px-6 lg:px-8">
        <h2 className="text-center text-3xl font-bold text-gray-900 sm:text-4xl">
          {t('pricing.compareTitle')}
        </h2>

        <div className="mt-12 overflow-x-auto">
          <table className="w-full min-w-[600px] text-left text-sm">
            <thead>
              <tr className="border-b border-gray-200">
                <th className="py-3 pr-4 font-semibold text-gray-600">Feature</th>
                <th className="px-4 py-3 text-center font-semibold text-gray-900">Starter</th>
                <th className="px-4 py-3 text-center font-semibold text-indigo-600">Business</th>
                <th className="px-4 py-3 text-center font-semibold text-gray-900">Enterprise</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => (
                <tr key={row.label} className="border-b border-gray-100">
                  <td className="py-3 pr-4 text-gray-700">{row.label}</td>
                  <td className="px-4 py-3 text-center"><BoolIcon value={row.starter} /></td>
                  <td className="px-4 py-3 text-center"><BoolIcon value={row.business} /></td>
                  <td className="px-4 py-3 text-center"><BoolIcon value={row.enterprise} /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}

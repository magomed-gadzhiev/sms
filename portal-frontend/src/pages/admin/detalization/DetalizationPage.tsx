import { PageHeader } from '../../../components/layout/PageHeader';

export function DetalizationPage() {
  return (
    <>
      <PageHeader
        title="Детализация"
        breadcrumbs={[{ label: 'Админ', href: '/admin/dashboard' }, { label: 'Детализация' }]}
      />
      <div className="bg-white border border-gray-200 rounded-lg p-12 text-center">
        <div className="text-4xl mb-4">🔍</div>
        <h2 className="text-lg font-semibold text-gray-700 mb-2">Детализация сообщений</h2>
        <p className="text-sm text-gray-400">Раздел находится в разработке</p>
      </div>
    </>
  );
}

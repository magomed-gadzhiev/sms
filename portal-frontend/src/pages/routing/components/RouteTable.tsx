import { StatusBadge } from '../../../components/ui/Badge';
import { Button } from '../../../components/ui/Button';
import type { RouteListItem } from '../types';
import type { Provider } from '../../../api/client';

interface RouteTableProps {
  routes: RouteListItem[];
  providers: Map<string, Provider>;
  loading: boolean;
  onEdit: (route: RouteListItem) => void;
  onDelete: (route: RouteListItem) => void;
}

export function RouteTable({ routes, providers, loading, onEdit, onDelete }: RouteTableProps) {
  if (loading) {
    return (
      <div className="bg-white rounded-lg border p-12 text-center text-gray-500">
        Загрузка маршрутов...
      </div>
    );
  }

  if (routes.length === 0) {
    return (
      <div className="bg-white rounded-lg border p-12 text-center text-gray-500">
        Нет маршрутов. Создайте первый маршрут.
      </div>
    );
  }

  return (
    <div className="bg-white rounded-lg border overflow-hidden">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b bg-gray-50">
            <th className="text-left px-4 py-3 font-medium text-gray-700">Название / Условия</th>
            <th className="text-left px-4 py-3 font-medium text-gray-700">Провайдер</th>
            <th className="text-center px-4 py-3 font-medium text-gray-700">Приоритет</th>
            <th className="text-center px-4 py-3 font-medium text-gray-700">Доля</th>
            <th className="text-center px-4 py-3 font-medium text-gray-700">Статус</th>
            <th className="text-right px-4 py-3 font-medium text-gray-700">Действия</th>
          </tr>
        </thead>
        <tbody>
          {routes.map((route) => {
            const provider = providers.get(route.provider_id);
            return (
              <tr key={route.id} className="border-b last:border-0 hover:bg-gray-50">
                <td className="px-4 py-3">
                  <div className="font-medium text-gray-900">{route.name}</div>
                  <div className="flex flex-wrap gap-1 mt-1">
                    {route.condition_tags.map((tag, i) => (
                      <span
                        key={i}
                        className="inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium bg-blue-50 text-blue-600"
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                  {route.schedule_summary && (
                    <div className="text-xs text-gray-500 mt-1">{route.schedule_summary}</div>
                  )}
                </td>
                <td className="px-4 py-3 text-gray-700">
                  {provider ? provider.name : route.provider_id.slice(0, 8)}
                </td>
                <td className="px-4 py-3 text-center">
                  <span className="font-bold text-gray-900">{route.priority}</span>
                </td>
                <td className="px-4 py-3 text-center text-gray-700">{route.share}%</td>
                <td className="px-4 py-3 text-center">
                  <StatusBadge status={route.status} />
                </td>
                <td className="px-4 py-3">
                  <div className="flex justify-end gap-2">
                    <Button variant="ghost" size="sm" onClick={() => onEdit(route)}>
                      Изменить
                    </Button>
                    <Button variant="danger" size="sm" onClick={() => onDelete(route)}>
                      Удалить
                    </Button>
                  </div>
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

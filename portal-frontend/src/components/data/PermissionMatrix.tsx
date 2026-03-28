import { useCallback, useMemo } from 'react';
import type { PermissionInfo } from '../../api/admin';

interface PermissionMatrixProps {
  allPermissions: PermissionInfo[];
  selectedIds: Set<string>;
  onChange: (ids: Set<string>) => void;
  disabled?: boolean;
}

const RESOURCES = [
  'messages',
  'clients',
  'providers',
  'routes',
  'analytics',
  'billing',
  'tarification',
  'templates',
  'users',
  'webhooks',
  'countries',
  'hlr',
  'audit',
];

const ACTIONS = ['read', 'write', 'delete'];

export function PermissionMatrix({ allPermissions, selectedIds, onChange, disabled = false }: PermissionMatrixProps) {
  const permMap = useMemo(() => {
    const m = new Map<string, string>();
    for (const p of allPermissions) {
      m.set(`${p.resource}:${p.action}`, p.id);
    }
    return m;
  }, [allPermissions]);

  const toggle = useCallback(
    (id: string) => {
      const next = new Set(selectedIds);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      onChange(next);
    },
    [selectedIds, onChange],
  );

  const toggleRow = useCallback(
    (resource: string) => {
      const rowIds = ACTIONS.map((a) => permMap.get(`${resource}:${a}`)).filter((id): id is string => !!id);
      const allSelected = rowIds.every((id) => selectedIds.has(id));
      const next = new Set(selectedIds);
      for (const id of rowIds) {
        if (allSelected) {
          next.delete(id);
        } else {
          next.add(id);
        }
      }
      onChange(next);
    },
    [permMap, selectedIds, onChange],
  );

  const toggleColumn = useCallback(
    (action: string) => {
      const colIds = RESOURCES.map((r) => permMap.get(`${r}:${action}`)).filter((id): id is string => !!id);
      const allSelected = colIds.every((id) => selectedIds.has(id));
      const next = new Set(selectedIds);
      for (const id of colIds) {
        if (allSelected) {
          next.delete(id);
        } else {
          next.add(id);
        }
      }
      onChange(next);
    },
    [permMap, selectedIds, onChange],
  );

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm border-collapse">
        <thead>
          <tr className="border-b border-gray-200">
            <th className="text-left py-2 px-3 font-medium text-gray-700">Resource</th>
            {ACTIONS.map((action) => {
              const colIds = RESOURCES.map((r) => permMap.get(`${r}:${action}`)).filter(
                (id): id is string => !!id,
              );
              const allSelected = colIds.length > 0 && colIds.every((id) => selectedIds.has(id));
              return (
                <th key={action} className="text-center py-2 px-3 font-medium text-gray-700">
                  <label className="flex items-center justify-center gap-1 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      disabled={disabled}
                      onChange={() => toggleColumn(action)}
                      className="rounded border-gray-300"
                    />
                    <span className="capitalize">{action}</span>
                  </label>
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          {RESOURCES.map((resource) => {
            const rowIds = ACTIONS.map((a) => permMap.get(`${resource}:${a}`)).filter(
              (id): id is string => !!id,
            );
            const allSelected = rowIds.length > 0 && rowIds.every((id) => selectedIds.has(id));
            return (
              <tr key={resource} className="border-b border-gray-100 hover:bg-gray-50">
                <td className="py-2 px-3">
                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={allSelected}
                      disabled={disabled}
                      onChange={() => toggleRow(resource)}
                      className="rounded border-gray-300"
                    />
                    <span className="capitalize font-medium text-gray-800">{resource}</span>
                  </label>
                </td>
                {ACTIONS.map((action) => {
                  const permId = permMap.get(`${resource}:${action}`);
                  if (!permId) {
                    return <td key={action} className="text-center py-2 px-3 text-gray-300">&mdash;</td>;
                  }
                  return (
                    <td key={action} className="text-center py-2 px-3">
                      <input
                        type="checkbox"
                        checked={selectedIds.has(permId)}
                        disabled={disabled}
                        onChange={() => toggle(permId)}
                        className="rounded border-gray-300"
                      />
                    </td>
                  );
                })}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

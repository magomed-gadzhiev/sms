import { useState, useEffect, useCallback } from 'react';
import { Button } from '../../../components/ui/Button';
import { Input } from '../../../components/ui/Input';
import { Modal } from '../../../components/ui/Modal';
import { Badge } from '../../../components/ui/Badge';
import { useToast } from '../../../components/ui/Toast';
import {
  resellerTariffApi,
  type ResellerTemplate,
  type ResellerTariffPlan,
  ApiError,
} from '../../../api/client';
import { TariffPlanEditor } from './TariffPlanEditor';
import { TemplateAssignModal } from './TemplateAssignModal';

export function TariffTemplatesTab() {
  const toast = useToast();
  const [templates, setTemplates] = useState<ResellerTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [plans, setPlans] = useState<ResellerTariffPlan[]>([]);
  const [plansLoading, setPlansLoading] = useState(false);

  // Create template modal
  const [showCreate, setShowCreate] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [creating, setCreating] = useState(false);

  // Assign modal
  const [assignTemplateId, setAssignTemplateId] = useState<string | null>(null);
  const [assignTemplateName, setAssignTemplateName] = useState('');

  const loadTemplates = useCallback(() => {
    setLoading(true);
    resellerTariffApi
      .listTemplates()
      .then((r) => setTemplates(r.templates || []))
      .catch(() => toast.error('Ошибка загрузки шаблонов'))
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    loadTemplates();
  }, [loadTemplates]);

  const loadPlans = useCallback((templateId: string) => {
    setPlansLoading(true);
    resellerTariffApi
      .listPlans({ template_id: templateId })
      .then((r) => setPlans(r.plans || []))
      .catch(() => toast.error('Ошибка загрузки планов'))
      .finally(() => setPlansLoading(false));
  }, []);

  function handleExpand(templateId: string) {
    if (expandedId === templateId) {
      setExpandedId(null);
      setPlans([]);
    } else {
      setExpandedId(templateId);
      loadPlans(templateId);
    }
  }

  async function handleCreate() {
    if (!newName.trim()) {
      toast.error('Введите название шаблона');
      return;
    }
    setCreating(true);
    try {
      await resellerTariffApi.createTemplate({
        name: newName.trim(),
        description: newDesc.trim() || undefined,
      });
      toast.success('Шаблон создан');
      setShowCreate(false);
      setNewName('');
      setNewDesc('');
      loadTemplates();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка создания шаблона');
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(id: string) {
    try {
      await resellerTariffApi.deleteTemplate(id);
      toast.success('Шаблон удалён');
      if (expandedId === id) {
        setExpandedId(null);
        setPlans([]);
      }
      loadTemplates();
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка удаления шаблона');
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="text-sm text-gray-500">
          {templates.length}{' '}
          {templates.length === 1 ? 'шаблон' : templates.length < 5 ? 'шаблона' : 'шаблонов'}
        </div>
        <Button size="sm" onClick={() => setShowCreate(true)}>
          Создать шаблон
        </Button>
      </div>

      {/* Create modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title="Новый шаблон тарифов"
      >
        <div className="flex flex-col gap-4">
          <Input
            label="Название"
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder="Например: Базовый тариф"
            required
          />
          <Input
            label="Описание"
            value={newDesc}
            onChange={(e) => setNewDesc(e.target.value)}
            placeholder="Необязательное описание"
          />
          <div className="flex gap-2">
            <Button onClick={handleCreate} disabled={creating}>
              {creating ? 'Создание...' : 'Создать'}
            </Button>
            <Button variant="secondary" onClick={() => setShowCreate(false)}>
              Отмена
            </Button>
          </div>
        </div>
      </Modal>

      {/* Assign modal */}
      {assignTemplateId && (
        <TemplateAssignModal
          open={!!assignTemplateId}
          onClose={() => setAssignTemplateId(null)}
          templateId={assignTemplateId}
          templateName={assignTemplateName}
          onAssigned={() => {
            loadTemplates();
            if (expandedId) loadPlans(expandedId);
          }}
        />
      )}

      {/* Content */}
      {loading ? (
        <div className="space-y-3">
          <div className="h-16 bg-gray-100 rounded animate-pulse" />
          <div className="h-16 bg-gray-100 rounded animate-pulse" />
        </div>
      ) : templates.length === 0 ? (
        <div className="py-16 text-center border border-gray-200 rounded-lg bg-white">
          <div className="text-gray-400">Нет шаблонов тарифов</div>
          <div className="text-sm text-gray-400 mt-1">Создайте первый шаблон</div>
        </div>
      ) : (
        <div className="space-y-2">
          {templates.map((tmpl) => {
            const isExpanded = expandedId === tmpl.id;
            return (
              <div
                key={tmpl.id}
                className="border border-gray-200 rounded-lg bg-white shadow-sm"
              >
                <div
                  className="flex items-center justify-between px-4 py-3 cursor-pointer hover:bg-gray-50"
                  onClick={() => handleExpand(tmpl.id)}
                >
                  <div className="flex items-center gap-3">
                    <span className="font-medium text-gray-900">{tmpl.name}</span>
                    {tmpl.description && (
                      <span className="text-sm text-gray-400">{tmpl.description}</span>
                    )}
                    <Badge variant="info">
                      {tmpl.assigned_count}{' '}
                      {tmpl.assigned_count === 1
                        ? 'субаккаунт'
                        : tmpl.assigned_count < 5
                          ? 'субаккаунта'
                          : 'субаккаунтов'}
                    </Badge>
                  </div>
                  <div className="flex items-center gap-3">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        setAssignTemplateId(tmpl.id);
                        setAssignTemplateName(tmpl.name);
                      }}
                      className="text-xs text-primary hover:underline"
                    >
                      Привязать
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDelete(tmpl.id);
                      }}
                      className="text-xs text-red-400 hover:text-red-600"
                    >
                      Удалить
                    </button>
                    <span className="text-gray-400 text-xs">{isExpanded ? '▼' : '▶'}</span>
                  </div>
                </div>
                {isExpanded && (
                  <div className="px-4 pb-4 border-t border-gray-100">
                    {plansLoading ? (
                      <div className="h-24 bg-gray-100 rounded animate-pulse mt-3" />
                    ) : (
                      <div className="mt-3">
                        <TariffPlanEditor
                          plans={plans}
                          ownerId={tmpl.id}
                          ownerType="template"
                          onRefresh={() => loadPlans(tmpl.id)}
                        />
                      </div>
                    )}
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

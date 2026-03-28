import { useState, useEffect, useCallback } from 'react';
import { Modal } from '../../../components/ui/Modal';
import { Button } from '../../../components/ui/Button';
import { TimelineEvent } from '../../../components/data/TimelineEvent';
import { useToast } from '../../../components/ui/Toast';
import { templatesApi, type TemplateInfo, type AuditEntry } from '../../../api/admin';

interface TemplateReviewModalProps {
  template: TemplateInfo | null;
  open: boolean;
  onClose: () => void;
  onUpdate: () => void;
}

function highlightVariables(body: string) {
  const parts = body.split(/(\{[^}]+\})/g);
  return parts.map((part, i) =>
    /^\{[^}]+\}$/.test(part) ? (
      <span key={i} className="text-primary font-semibold bg-primary/10 px-1 rounded">
        {part}
      </span>
    ) : (
      part
    ),
  );
}

const auditActionMap: Record<string, { icon: string; color: 'blue' | 'green' | 'red' | 'yellow'; title: string }> = {
  created: { icon: '✚', color: 'blue', title: 'Создан' },
  approved: { icon: '✓', color: 'green', title: 'Одобрен' },
  rejected: { icon: '✗', color: 'red', title: 'Отклонён' },
  assign_reviewer: { icon: '👤', color: 'blue', title: 'Назначен ревьюер' },
  request_revision: { icon: '↩', color: 'yellow', title: 'Запрошена доработка' },
};

export function TemplateReviewModal({ template, open, onClose, onUpdate }: TemplateReviewModalProps) {
  const toast = useToast();
  const [auditEntries, setAuditEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [revisionMode, setRevisionMode] = useState(false);
  const [rejectMode, setRejectMode] = useState(false);
  const [comment, setComment] = useState('');

  const fetchAudit = useCallback(async () => {
    if (!template) return;
    setLoading(true);
    try {
      const res = await templatesApi.audit(template.template_id);
      setAuditEntries(res.entries || []);
    } catch {
      toast.error('Failed to load audit trail');
    } finally {
      setLoading(false);
    }
  }, [template, toast]);

  useEffect(() => {
    if (open && template) {
      fetchAudit();
      setRevisionMode(false);
      setRejectMode(false);
      setComment('');
    }
  }, [open, template, fetchAudit]);

  const handleAssign = async () => {
    if (!template) return;
    setSaving(true);
    try {
      await templatesApi.assign(template.template_id);
      toast.success('Шаблон взят на ревью');
      onUpdate();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  const handleApprove = async () => {
    if (!template) return;
    setSaving(true);
    try {
      await templatesApi.approve(template.template_id);
      toast.success('Шаблон одобрен');
      onUpdate();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  const handleRequestRevision = async () => {
    if (!template || !comment.trim()) return;
    setSaving(true);
    try {
      await templatesApi.requestRevision(template.template_id, { comment: comment.trim() });
      toast.success('Запрос на доработку отправлен');
      setRevisionMode(false);
      setComment('');
      onUpdate();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  const handleReject = async () => {
    if (!template || !comment.trim()) return;
    setSaving(true);
    try {
      await templatesApi.reject(template.template_id, { reason: comment.trim() });
      toast.success('Шаблон отклонён');
      setRejectMode(false);
      setComment('');
      onUpdate();
    } catch (e) {
      toast.error(e instanceof Error ? e.message : 'Failed');
    } finally {
      setSaving(false);
    }
  };

  const getAuditDescription = (entry: AuditEntry): string | undefined => {
    if (entry.action === 'rejected' || entry.action === 'request_revision') {
      return (entry.details?.reason as string) || (entry.details?.comment as string) || undefined;
    }
    return undefined;
  };

  return (
    <Modal open={open} onClose={onClose} title="Ревью шаблона" wide>
      {template && (
        <div className="space-y-5">
          {/* Header info */}
          <div className="flex gap-4 text-sm text-gray-600">
            <div>
              <span className="font-medium text-gray-900">Шаблон:</span> {template.name}
            </div>
            <div>
              <span className="font-medium text-gray-900">Клиент:</span> {template.client_id.slice(0, 8)}...
            </div>
          </div>

          {/* Template body */}
          <div className="rounded border border-gray-200 bg-gray-50 p-4">
            <div className="text-xs font-medium text-gray-500 mb-2 uppercase tracking-wide">Текст шаблона</div>
            <pre className="whitespace-pre-wrap font-mono text-sm text-gray-800 leading-relaxed">
              {highlightVariables(template.body)}
            </pre>
          </div>

          {/* Action buttons */}
          <div className="space-y-3">
            {template.status === 'pending' && (
              <div className="flex gap-2">
                <Button onClick={handleAssign} disabled={saving}>
                  {saving ? 'Назначение...' : 'Взять на ревью'}
                </Button>
              </div>
            )}

            {template.status === 'review' && !revisionMode && !rejectMode && (
              <div className="flex gap-2">
                <Button variant="primary" onClick={handleApprove} disabled={saving}>
                  {saving ? 'Одобрение...' : 'Одобрить'}
                </Button>
                <Button variant="secondary" onClick={() => { setRevisionMode(true); setRejectMode(false); setComment(''); }}>
                  Запросить доработку
                </Button>
                <Button variant="danger" onClick={() => { setRejectMode(true); setRevisionMode(false); setComment(''); }}>
                  Отклонить
                </Button>
              </div>
            )}

            {revisionMode && (
              <div className="space-y-3 rounded border border-yellow-200 bg-yellow-50 p-3">
                <label className="block text-sm font-medium text-gray-700">
                  Комментарий для доработки <span className="text-danger">*</span>
                </label>
                <textarea
                  className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                  rows={3}
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Укажите, что нужно доработать..."
                />
                <div className="flex gap-2 justify-end">
                  <Button variant="secondary" onClick={() => { setRevisionMode(false); setComment(''); }}>
                    Отмена
                  </Button>
                  <Button onClick={handleRequestRevision} disabled={saving || !comment.trim()}>
                    {saving ? 'Отправка...' : 'Отправить'}
                  </Button>
                </div>
              </div>
            )}

            {rejectMode && (
              <div className="space-y-3 rounded border border-red-200 bg-red-50 p-3">
                <label className="block text-sm font-medium text-gray-700">
                  Причина отклонения <span className="text-danger">*</span>
                </label>
                <textarea
                  className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-primary/50"
                  rows={3}
                  value={comment}
                  onChange={(e) => setComment(e.target.value)}
                  placeholder="Укажите причину отклонения..."
                />
                <div className="flex gap-2 justify-end">
                  <Button variant="secondary" onClick={() => { setRejectMode(false); setComment(''); }}>
                    Отмена
                  </Button>
                  <Button variant="danger" onClick={handleReject} disabled={saving || !comment.trim()}>
                    {saving ? 'Отклонение...' : 'Отклонить'}
                  </Button>
                </div>
              </div>
            )}
          </div>

          {/* Audit timeline */}
          <div>
            <div className="text-sm font-semibold text-gray-900 mb-3">История</div>
            {loading ? (
              <div className="text-sm text-gray-500">Загрузка...</div>
            ) : auditEntries.length === 0 ? (
              <div className="text-sm text-gray-500">Нет записей</div>
            ) : (
              <div>
                {auditEntries.map((entry) => {
                  const mapping = auditActionMap[entry.action] || { icon: '?', color: 'gray' as const, title: entry.action };
                  return (
                    <TimelineEvent
                      key={entry.id}
                      icon={mapping.icon}
                      color={mapping.color}
                      title={mapping.title}
                      description={getAuditDescription(entry)}
                      author={entry.user_id}
                      date={entry.created_at}
                    />
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}

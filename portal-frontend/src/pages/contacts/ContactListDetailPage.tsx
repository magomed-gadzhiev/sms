import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import {
  contactListsApi,
  type ContactList,
  type Contact,
  type ContactAttribute,
} from '../../api/contacts';
import { ApiError } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { ConfirmDialog } from '../../components/ui/ConfirmDialog';
import { useToast } from '../../components/ui/Toast';

export function ContactListDetailPage() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const toast = useToast();

  const [list, setList] = useState<ContactList | null>(null);
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [attributes, setAttributes] = useState<ContactAttribute[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [search, setSearch] = useState('');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  // Add contact modal
  const [showAddContact, setShowAddContact] = useState(false);
  const [newPhone, setNewPhone] = useState('');
  const [newAttrs, setNewAttrs] = useState<Record<string, string>>({});
  const [newTags, setNewTags] = useState('');
  const [adding, setAdding] = useState(false);
  const [addContactError, setAddContactError] = useState('');

  // Attributes modal
  const [showAttrsModal, setShowAttrsModal] = useState(false);
  const [editAttrs, setEditAttrs] = useState<ContactAttribute[]>([]);
  const [savingAttrs, setSavingAttrs] = useState(false);

  // Delete contact
  const [deleteContactId, setDeleteContactId] = useState<string | null>(null);
  const [deletingContact, setDeletingContact] = useState(false);

  const perPage = 20;

  const loadContacts = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError('');
    try {
      const resp = await contactListsApi.listContacts(id, page, perPage, search);
      setContacts(resp.contacts ?? []);
      setTotal(resp.total ?? 0);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Не удалось загрузить контакты');
    } finally {
      setLoading(false);
    }
  }, [id, page, search]);

  useEffect(() => {
    if (!id) return;
    // Load list info and attributes
    contactListsApi.get(id).then(setList).catch(() => {
      setError('Не удалось загрузить информацию о списке');
    });
    contactListsApi
      .getAttributes(id)
      .then((resp) => setAttributes(resp.attributes ?? []))
      .catch(() => {});
  }, [id]);

  useEffect(() => {
    loadContacts();
  }, [loadContacts]);

  async function handleAddContact() {
    if (!id || !newPhone.trim()) return;
    setAdding(true);
    try {
      const attrValues: Record<string, unknown> = {};
      for (const [k, v] of Object.entries(newAttrs)) {
        if (v) attrValues[k] = v;
      }
      const tags = newTags
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean);
      await contactListsApi.createContact(id, {
        phone: newPhone.trim(),
        attributes: Object.keys(attrValues).length > 0 ? attrValues : undefined,
        tags: tags.length > 0 ? tags : undefined,
      });
      setShowAddContact(false);
      setNewPhone('');
      setNewAttrs({});
      setNewTags('');
      loadContacts();
    } catch (err) {
      setAddContactError(err instanceof ApiError ? err.message : 'Ошибка при добавлении контакта');
    } finally {
      setAdding(false);
    }
  }

  async function handleDeleteContact() {
    if (!id || !deleteContactId) return;
    setDeletingContact(true);
    try {
      await contactListsApi.deleteContact(id, deleteContactId);
      setContacts((prev) => prev.filter((c) => c.id !== deleteContactId));
      setTotal((prev) => prev - 1);
      setDeleteContactId(null);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при удалении');
    } finally {
      setDeletingContact(false);
    }
  }

  async function handleSaveAttributes() {
    if (!id) return;
    // Проверка что все атрибуты имеют системное имя
    if (editAttrs.some(a => !a.name.trim())) {
      alert('Все атрибуты должны иметь системное имя');
      return;
    }
    // Проверка дублей системных имён
    const names = editAttrs.map(a => a.name.trim()).filter(Boolean);
    const uniqueNames = new Set(names);
    if (uniqueNames.size !== names.length) {
      alert('Системные имена атрибутов должны быть уникальными');
      return;
    }
    setSavingAttrs(true);
    try {
      const resp = await contactListsApi.setAttributes(id, editAttrs);
      setAttributes(resp.attributes ?? []);
      setShowAttrsModal(false);
    } catch (err) {
      toast.error(err instanceof ApiError ? err.message : 'Ошибка при сохранении атрибутов');
    } finally {
      setSavingAttrs(false);
    }
  }

  function openAttrsModal() {
    setEditAttrs(attributes.length > 0 ? [...attributes] : []);
    setShowAttrsModal(true);
  }

  function addAttribute() {
    setEditAttrs((prev) => [
      ...prev,
      {
        name: '',
        display_name: '',
        type: 'string' as const,
        required: false,
        position: prev.length,
      },
    ]);
  }

  function updateAttribute(index: number, field: string, value: unknown) {
    setEditAttrs((prev) =>
      prev.map((a, i) => (i === index ? { ...a, [field]: value } : a)),
    );
  }

  function removeAttribute(index: number) {
    setEditAttrs((prev) => prev.filter((_, i) => i !== index));
  }

  const totalPages = Math.max(1, Math.ceil(total / perPage));

  function handleSearchSubmit(e: React.FormEvent) {
    e.preventDefault();
    setPage(1);
    loadContacts();
  }

  return (
    <div>
      <PageHeader
        title={list?.name ?? 'Контактная база'}
        subtitle={list?.description || undefined}
        breadcrumbs={[
          { label: 'Контактные базы', href: '/contact-lists' },
          { label: list?.name ?? '...' },
        ]}
        actions={
          <div className="flex gap-2">
            <Button variant="secondary" onClick={openAttrsModal}>
              Настройки атрибутов
            </Button>
            <Button
              variant="secondary"
              onClick={() => navigate(`/contact-lists/${id}/import`)}
            >
              Импорт
            </Button>
            <Button onClick={() => { setAddContactError(''); setShowAddContact(true); }}>
              + Добавить контакт
            </Button>
          </div>
        }
      />

      {error && <p className="text-red-600 mb-4">{error}</p>}

      {/* Search bar */}
      <form onSubmit={handleSearchSubmit} className="mb-4 flex gap-2">
        <label htmlFor="contact-search" className="sr-only">Поиск по номеру телефона</label>
        <input
          id="contact-search"
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder="Поиск по номеру телефона..."
          className="flex-1 max-w-md border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
        <Button variant="secondary" type="submit">
          Найти
        </Button>
      </form>

      {/* Contacts table */}
      <div className="border border-gray-200 rounded-lg overflow-hidden">
        {loading ? (
          <div className="animate-pulse p-8 text-center text-gray-500">
            Загрузка...
          </div>
        ) : (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <caption className="sr-only">Таблица контактов списка</caption>
                <thead className="bg-gray-50 border-b border-gray-200">
                  <tr>
                    <th className="px-4 py-3 text-left font-medium text-gray-700">
                      Телефон
                    </th>
                    {attributes.map((attr) => (
                      <th
                        key={attr.name}
                        className="px-4 py-3 text-left font-medium text-gray-700 hidden sm:table-cell"
                      >
                        {attr.display_name || attr.name}
                      </th>
                    ))}
                    <th className="px-4 py-3 text-left font-medium text-gray-700">
                      Теги
                    </th>
                    <th className="px-4 py-3 w-24"></th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-gray-100">
                  {contacts.length === 0 ? (
                    <tr>
                      <td
                        colSpan={3 + attributes.length}
                        className="px-4 py-8 text-center text-gray-500"
                      >
                        {search ? (
                          'По запросу ничего не найдено'
                        ) : (
                          <div>
                            <p className="mb-3">Контакты не добавлены</p>
                            <div className="flex justify-center gap-2">
                              <button
                                onClick={() => setShowAddContact(true)}
                                className="text-sm text-blue-600 hover:underline"
                              >
                                Добавить контакт
                              </button>
                              <span className="text-gray-300">|</span>
                              <button
                                onClick={() => navigate(`/contact-lists/${id}/import`)}
                                className="text-sm text-blue-600 hover:underline"
                              >
                                Импортировать из файла
                              </button>
                            </div>
                          </div>
                        )}
                      </td>
                    </tr>
                  ) : (
                    contacts.map((contact) => (
                      <tr key={contact.id} className="hover:bg-gray-50">
                        <td className="px-4 py-3 font-mono text-gray-800">
                          {contact.phone}
                        </td>
                        {attributes.map((attr) => (
                          <td
                            key={attr.name}
                            className="px-4 py-3 text-gray-600 hidden sm:table-cell"
                          >
                            {String(contact.attributes?.[attr.name] ?? '—')}
                          </td>
                        ))}
                        <td className="px-4 py-3">
                          <div className="flex gap-1 flex-wrap">
                            {(contact.tags ?? []).map((tag) => (
                              <span
                                key={tag}
                                className="inline-flex items-center rounded-full bg-blue-50 text-blue-700 px-2 py-0.5 text-xs"
                              >
                                {tag}
                              </span>
                            ))}
                          </div>
                        </td>
                        <td className="px-4 py-3">
                          <Button
                            variant="danger"
                            size="sm"
                            onClick={() => setDeleteContactId(contact.id)}
                          >
                            Удалить
                          </Button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {total > perPage && (
              <div className="flex items-center justify-between px-4 py-3 border-t border-gray-200 bg-gray-50">
                <span className="text-sm text-gray-700">
                  {(page - 1) * perPage + 1}–{Math.min(page * perPage, total)}{' '}
                  из {total}
                </span>
                <div className="flex gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={page <= 1}
                    onClick={() => setPage(page - 1)}
                  >
                    Назад
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={page >= totalPages}
                    onClick={() => setPage(page + 1)}
                  >
                    Вперёд
                  </Button>
                </div>
              </div>
            )}
          </>
        )}
      </div>

      {/* Add contact modal */}
      <Modal
        open={showAddContact}
        onClose={() => { setShowAddContact(false); setAddContactError(''); }}
        title="Добавить контакт"
      >
        <div className="space-y-4">
          {addContactError && (
            <div role="alert" className="rounded bg-red-50 border border-red-200 px-3 py-2 text-sm text-red-700">
              {addContactError}
            </div>
          )}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Телефон *
            </label>
            <input
              type="text"
              value={newPhone}
              onChange={(e) => setNewPhone(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="+79001234567"
              autoFocus
            />
          </div>
          {attributes.map((attr) => (
            <div key={attr.name}>
              <label className="block text-sm font-medium text-gray-700 mb-1">
                {attr.display_name || attr.name}
                {attr.required && ' *'}
              </label>
              <input
                type={attr.type === 'number' ? 'number' : 'text'}
                value={newAttrs[attr.name] ?? ''}
                onChange={(e) =>
                  setNewAttrs((prev) => ({
                    ...prev,
                    [attr.name]: e.target.value,
                  }))
                }
                className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              />
            </div>
          ))}
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">
              Теги (через запятую)
            </label>
            <input
              type="text"
              value={newTags}
              onChange={(e) => setNewTags(e.target.value)}
              className="w-full border border-gray-300 rounded px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
              placeholder="vip, москва"
            />
          </div>
          <div className="flex justify-end gap-3 pt-2">
            <Button
              variant="secondary"
              onClick={() => setShowAddContact(false)}
              disabled={adding}
            >
              Отмена
            </Button>
            <Button
              onClick={handleAddContact}
              disabled={adding || !newPhone.trim()}
            >
              {adding ? 'Добавление...' : 'Добавить'}
            </Button>
          </div>
        </div>
      </Modal>

      {/* Attributes modal */}
      <Modal
        open={showAttrsModal}
        onClose={() => setShowAttrsModal(false)}
        title="Настройки атрибутов"
        wide
      >
        <div className="space-y-3">
          {editAttrs.map((attr, idx) => (
            <div
              key={idx}
              className="flex gap-2 items-start border border-gray-200 rounded p-3"
            >
              <div className="flex-1 grid grid-cols-2 gap-2">
                <input
                  type="text"
                  value={attr.name}
                  onChange={(e) => updateAttribute(idx, 'name', e.target.value)}
                  placeholder="Системное имя"
                  className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <input
                  type="text"
                  value={attr.display_name}
                  onChange={(e) =>
                    updateAttribute(idx, 'display_name', e.target.value)
                  }
                  placeholder="Отображаемое имя"
                  className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                />
                <select
                  value={attr.type}
                  onChange={(e) => updateAttribute(idx, 'type', e.target.value)}
                  className="border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                >
                  <option value="string">Строка</option>
                  <option value="number">Число</option>
                  <option value="date">Дата</option>
                  <option value="boolean">Логический</option>
                </select>
                <label className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    checked={attr.required}
                    onChange={(e) =>
                      updateAttribute(idx, 'required', e.target.checked)
                    }
                    className="rounded"
                  />
                  Обязательный
                </label>
              </div>
              <Button
                variant="danger"
                size="sm"
                onClick={() => removeAttribute(idx)}
                aria-label="Удалить атрибут"
              >
                ✕
              </Button>
            </div>
          ))}
          <Button variant="ghost" size="sm" onClick={addAttribute}>
            + Добавить атрибут
          </Button>
        </div>
        <div className="flex justify-end gap-3 pt-4 mt-4 border-t border-gray-200">
          <Button
            variant="secondary"
            onClick={() => setShowAttrsModal(false)}
            disabled={savingAttrs}
          >
            Отмена
          </Button>
          <Button onClick={handleSaveAttributes} disabled={savingAttrs}>
            {savingAttrs ? 'Сохранение...' : 'Сохранить'}
          </Button>
        </div>
      </Modal>

      {/* Delete contact confirm */}
      <ConfirmDialog
        open={deleteContactId !== null}
        onConfirm={handleDeleteContact}
        onCancel={() => setDeleteContactId(null)}
        title="Удалить контакт"
        description="Вы уверены, что хотите удалить этот контакт из базы?"
        confirmLabel="Удалить"
        variant="danger"
        loading={deletingContact}
      />
    </div>
  );
}

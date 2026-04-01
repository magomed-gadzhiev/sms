# Quickstart: Portal UX Improvements

**Feature**: 019-portal-ux-improvements

## Что будет реализовано

| Фича | Приоритет | Сложность |
|------|-----------|-----------|
| Визуальный дашборд с графиками | P1 | Medium |
| Массовые действия в таблицах | P1 | Medium |
| Улучшенный мастер рассылки (free step nav) | P2 | Low |
| Экспорт данных в CSV | P2 | Medium |
| Быстрый поиск — Command Palette | P2 | Medium |
| Inline-валидация форм | P3 | Low |
| Центр уведомлений | P3 | High |

---

## Сопоставление изменений с файлами

### Backend (Go) — portal-gateway

| Изменение | Файл |
|-----------|------|
| Новые handlers для уведомлений | `internal/gateway/portal/handlers/notifications.go` |
| Новый handler глобального поиска | `internal/gateway/portal/handlers/search.go` |
| Новые handlers для async экспорта | `internal/gateway/portal/handlers/export.go` |
| Расширение GetDashboard с charts | `internal/gateway/portal/handlers/dashboard.go` |
| Расширение GetAnalytics (compare, cost) | `internal/gateway/portal/handlers/analytics.go` |
| Регистрация новых маршрутов | `internal/gateway/portal/router/router.go` |
| Фоновый планировщик уведомлений | `internal/gateway/portal/notifications/scheduler.go` |
| Инициализация scheduler в main | `cmd/portal-gateway/main.go` |
| Миграция таблицы notifications | `migrations/000072_notifications.up.sql` |
| Откат миграции | `migrations/000072_notifications.down.sql` |

### Frontend (React) — portal-frontend

| Изменение | Файл |
|-----------|------|
| Dashboard с Recharts | `src/pages/dashboard/DashboardPage.tsx` |
| Analytics comparison + cost tab | `src/pages/analytics/AnalyticsPage.tsx` |
| DataTable с bulk selection | `src/components/data/DataTable.tsx` |
| Панель массовых действий | `src/components/data/BulkActionBar.tsx` |
| Wizard с free navigation | `src/pages/campaigns/CampaignWizardPage.tsx` |
| Step indicator component | `src/components/campaigns/StepIndicator.tsx` |
| Command Palette component | `src/components/ui/CommandPalette.tsx` |
| Notification Bell | `src/components/ui/NotificationBell.tsx` |
| Notification Dropdown | `src/components/ui/NotificationPanel.tsx` |
| Хук для уведомлений (polling) | `src/hooks/useNotifications.ts` |
| Хук Command Palette | `src/hooks/useCommandPalette.ts` |
| Inline validation hook | `src/hooks/useFormValidation.ts` |
| Character counter component | `src/components/ui/CharacterCounter.tsx` |
| API client — notifications | `src/api/client.ts` (extend) |
| API client — search | `src/api/client.ts` (extend) |
| API client — async export | `src/api/client.ts` (extend) |
| UserLayout с колокольчиком + Ctrl+K | `src/components/layout/UserLayout.tsx` |
| Inline validation в Template form | `src/pages/templates/*.tsx` |
| Inline validation в SenderName form | `src/pages/sender-names/*.tsx` |
| Export кнопка на Messages | `src/pages/messages/MessagesPage.tsx` |
| Export кнопка на Analytics | `src/pages/analytics/AnalyticsPage.tsx` |

---

## Запуск и проверка

### Backend

```bash
# Применить миграцию
scripts/server.sh migrate

# Пересобрать portal-gateway
scripts/server.sh deploy portal-gateway

# Проверить новые эндпоинты
curl -s http://localhost:8082/portal/v1/notifications \
  -H "Cookie: session=..." | jq .

curl -s "http://localhost:8082/portal/v1/search?q=шаблон" \
  -H "Cookie: session=..." | jq .
```

### Frontend

```bash
cd portal-frontend
npm run dev
```

Открыть `http://localhost:5173/dashboard` — должны отображаться графики.  
Нажать `Ctrl+K` — должна открыться палитра команд.  
Нажать на колокольчик в хедере — должна открыться панель уведомлений.

---

## Порядок реализации (рекомендуемый)

1. **Миграция + backend уведомлений** — foundation для полинга
2. **Backend: dashboard charts + analytics compare** — data for frontend
3. **Frontend: Dashboard charts** — самый заметный результат (P1)
4. **Frontend: DataTable bulk actions** — P1, независим от backend
5. **Frontend: CSV export кнопки** — переиспользует существующий ExportCSV
6. **Backend: async export** — для случаев >10K записей
7. **Frontend: Command Palette** — независимый компонент
8. **Frontend: Notification Bell + Panel** — зависит от backend (п.1)
9. **Frontend: Wizard step navigation** — изолированное изменение
10. **Frontend: Inline validation** — последним, наименьший риск


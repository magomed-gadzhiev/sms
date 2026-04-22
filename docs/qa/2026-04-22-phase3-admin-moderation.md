# QA Checklist — Phase 3 Admin Moderation Inbox

Branch: feature/sender-names-phase1
Date: 2026-04-22

## Sidebar (AdminSidebar)

- [ ] Load any admin page. Sidebar shows three new entries under "Основное":
  - Модерация имён (with badge if pending count > 0)
  - Справочник имён
  - Модерация биндингов (with badge if pending count > 0)
- [ ] Initial load fetches `/admin/v1/moderation/counts`. Verify in Network tab.
- [ ] Counts poll every 30s automatically.
- [ ] After approving/rejecting a sender name or binding, sidebar badge refreshes within ~1s (via `moderation:changed` CustomEvent).
- [ ] Badge hidden when count is 0.
- [ ] Badge shows `99+` when count > 99.

## /admin/moderation/sender-names (Inbox)

- [ ] Only `pending` sender names. Approved/rejected/deactivated absent.
- [ ] Columns: Имя (моно) · Клиент (email) · Канал (chip) · Подано · Действия.
- [ ] Approve one-click. Row disappears. Sidebar badge decreases by 1.
- [ ] Reject requires non-empty reason. Empty → "Укажите причину" error.
- [ ] Filters: channel (select), name search (text).
- [ ] Empty state: «Очередь модерации пуста».

## /admin/moderation/sender-names-directory

- [ ] Default filter: Статус = Одобрено. Only approved rows visible initially.
- [ ] Switch filter to Деактивировано — only deactivated rows.
- [ ] Filters: статус, канал, имя.
- [ ] Row action «Деактивировать» only for approved rows. Opens modal with optional reason.
- [ ] Header CTA «Создать имя» opens create modal: Client ID (UUID input), Канал (select), Имя (text).
- [ ] Name validation: reject spaces; reject > 11 chars for alphanumeric; accept `.`, `_`, `-`.
- [ ] Submit → row appears in the directory with status=Одобрено (auto-approve via admin).
- [ ] After create or deactivate: sidebar badge doesn't change (these don't affect pending counts).

## /admin/moderation/bindings

- [ ] Pending bindings grouped by operator. Each group: operator name header + inner table.
- [ ] Row columns: Шаблон (name + truncated body tooltip) · Имя отправителя (моно) · Канал (chip) · Клиент (email) · Подано · Действия.
- [ ] Approve one-click. Row disappears from the group. Sidebar binding-badge decreases by 1.
- [ ] Reject requires non-empty reason.
- [ ] Operator filter (select). Select a specific operator — only their pending bindings shown.
- [ ] Empty state: «Нет биндингов на модерации».

## Scope isolation (admin vs aggregator_moderator)

- [ ] Login as **admin/superadmin**:
  - Sidebar badge for sender names reflects pending count of DIRECT clients only.
  - Inbox list hides aggregator-owned sender names.
  - Binding inbox hides aggregator-owned bindings.
  - `GET /admin/v1/moderation/counts` returns counts scoped to `parent_client_id IS NULL`.

- [ ] Login as **aggregator_moderator**:
  - Sidebar shows counts for THEIR sub-accounts only (join `parent_client_id = <their-client-id>`).
  - Inbox shows only their sub-account sender names.
  - Binding inbox shows only their sub-account bindings.
  - Direct-client rows NOT visible.
  - Cross-tenant approve attempt: `POST /admin/v1/moderation/bindings/<foreign-id>/approve` → 403.

## Redirects

- [ ] Navigate to `/admin/sender-names` → redirected to `/admin/moderation/sender-names` (URL changes).
- [ ] Navigate to `/admin/operator-templates` → redirected to `/admin/moderation/bindings`.
- [ ] `/admin/sender-names/:id` still works for admin sender-name detail (no redirect).

## Regressions

- [ ] Client portal `/sender-names` list unaffected — channel tabs work, counts ok.
- [ ] Client portal `/sender-names/:id?tab=...` unaffected — all tabs render.
- [ ] Client portal `/templates` still read-only, no write actions.
- [ ] No console errors in browser devtools on any admin moderation page.
- [ ] No server 5xx in admin gateway logs under normal inbox flow.

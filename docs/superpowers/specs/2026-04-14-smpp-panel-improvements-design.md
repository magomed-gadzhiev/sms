# SMPP Panel Improvements — Design Spec

**Date:** 2026-04-14  
**Scope:** Client portal — SMPP provider management  
**Status:** Approved

---

## Overview

Three improvements to the SMPP provider management section of the client self-service portal:

1. Remove the "Маршрутизация" step from the add-provider wizard
2. Translate non-technical field labels to Russian across all wizard steps
3. Add an "Edit" button per provider in the provider list, opening a flat modal

---

## Task 1 — Remove Routing Step from Wizard

### What changes

The current wizard has 6 steps:
1. Основное
2. Подключение
3. Параметры
4. Тест
5. **Маршрутизация** ← removed
6. Итоги → renumbered to 5

After the change the wizard has 5 steps: Основное → Подключение → Параметры → Тест → Итоги.

### Files affected

- `portal-frontend/src/pages/providers/ProviderWizardPage.tsx` — remove step 5 from steps array, update total step count
- `portal-frontend/src/pages/providers/steps/Step5Routing.tsx` — delete file
- `portal-frontend/src/pages/providers/steps/Step6Summary.tsx` — rename to `Step5Summary.tsx`, remove routing_rules section from summary table
- `portal-frontend/src/pages/providers/components/WizardProgress.tsx` — update step labels array

### Backend / API

The `routing_rules` field remains in the API request/response and the database model — no backend changes. The field will simply be omitted (empty array) when creating a provider through the portal. This preserves backward compatibility for any future routing section.

---

## Task 2 — Localize Field Labels

All non-technical user-facing labels are translated to Russian. Technical identifiers and industry-standard abbreviations are left in English.

| Field (before) | Label (after) | Notes |
|---|---|---|
| System ID | Системный ID | |
| Bind Type | Режим привязки | |
| Window Size | Размер окна | |
| Max Connections | Макс. соединений | |
| TPS Limit | Лимит TPS | "TPS" kept — industry standard |
| Tags | Метки | |
| Host | Host | kept — technical |
| Port | Port | kept — technical |
| Password | Пароль | already translated in most places; verify all |
| TRX / TX / RX | TRX / TX / RX | kept — protocol abbreviations |

### Files affected

- `portal-frontend/src/pages/providers/steps/Step1BasicInfo.tsx`
- `portal-frontend/src/pages/providers/steps/Step2Connection.tsx`
- `portal-frontend/src/pages/providers/steps/Step3Params.tsx`
- `portal-frontend/src/pages/providers/steps/Step6Summary.tsx` (becomes Step5Summary)

---

## Task 3 — Edit Provider Modal

### Trigger

An "Редактировать" button is added to each row in the provider list table (`ProvidersPage.tsx`). Clicking it opens a modal pre-filled with the provider's current data. The SMPP connection remains active while the modal is open.

### Modal structure

The modal is a flat scrollable dialog (no wizard steps) divided into three labeled sections:

**Основное**
- Название (required)
- Описание (optional)
- Метки (comma-separated)

**Подключение**
- Header badge: ⚠ "Изменение этих полей перезапустит соединение" (always visible, amber)
- Host + Port (grid: 2fr + 1fr)
- Системный ID + Пароль (grid: 1fr + 1fr)
- Режим привязки + "▶ Протестировать" button (inline)

**Параметры**
- Макс. соединений / Размер окна / Лимит TPS (grid: 1fr 1fr 1fr)

Footer: "Отмена" (secondary) + "Сохранить" (primary).

### Password field behavior

The password field displays a cosmetic mask `••••••••` (the backend never returns the real password value). On click/focus the field clears and allows entering a new password. If the user does not interact with the password field (mask remains), the password is **not sent** in the update request — the backend keeps the existing value.

Implementation: track a `passwordChanged: boolean` flag in component state. Only include `password` in the `UpdateProviderRequest` payload when `passwordChanged === true`.

### Test connection button

Located in the Подключение section. Behavior identical to Step 4 of the wizard: calls `POST /portal/v1/providers/test-connection` with the current form values (not the saved provider). Shows success/failure + latency inline below the button. The provider does not need to be saved first.

### Save behavior

On "Сохранить" click:

1. Determine if any connection field changed: `host`, `port`, `system_id`, `password` (when `passwordChanged`), `bind_type`.
2. **If connection fields changed:** show a confirmation dialog — *"Изменение параметров подключения приведёт к кратковременному разрыву соединения. Продолжить?"* — with "Отмена" and "Продолжить" buttons.
3. **If only soft fields changed** (`name`, `description`, `tags`, `max_connections`, `window_size`, `tps_limit`): save silently without confirmation.
4. On confirmed save: call `PUT /portal/v1/providers/{id}`. On success: close modal, refresh provider list row.

### Files to create / modify

- `portal-frontend/src/pages/providers/ProvidersPage.tsx` — add "Редактировать" button per row, wire up modal state
- `portal-frontend/src/pages/providers/components/EditProviderModal.tsx` — new component
- `portal-frontend/src/pages/providers/components/EditProviderModal.test.tsx` — unit tests (password flag, connection-field detection)

---

## Out of Scope

- Routing configuration UI (separate feature)
- Bulk editing
- Provider activation/deactivation toggle (existing separate control)
- Changes to the backend API contract

---

## Open Questions

None — all design decisions resolved.

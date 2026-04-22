# QA Checklist — Phase 2 UI (Sender Names & Templates Redesign)

Branch: feature/sender-names-phase1
Date: 2026-04-22

## /sender-names list (ChannelTabs)

- [ ] Open `/sender-names`. Default tab = SMS, URL has no `?channel=` (or `?channel=sms`).
- [ ] Click Voice. URL gains `?channel=voice`. Table re-filters (client-side) to Voice items only.
- [ ] Click Viber. Same. Counts next to each tab reflect current page (may be approximate).
- [ ] Register new name from empty state on Voice tab. Modal opens with channel=Voice pre-selected. Can still switch to SMS/Viber in the modal select.
- [ ] Create "TestVoice" on Voice tab. Item appears immediately in the Voice tab.
- [ ] Reload the page with `?channel=viber`. Viber tab is active on mount.
- [ ] Changing channel resets pagination to page 1.

## /sender-names/:id (tabs)

- [ ] Open a sender name. Default tab = Overview. URL has no `?tab=`.
- [ ] Status badge, channel chip, created/reviewed dates, rejection reason (if present) show in Overview tab.
- [ ] Click Templates. URL gains `?tab=templates`. Templates list loads, scoped to this sender name.
- [ ] If sender name status != approved: "Создать шаблон" is disabled with tooltip.
- [ ] If approved, click "Создать шаблон". Modal opens; sender-name select is pre-filled AND disabled. Submit creates a template. Row appears in the tab.
- [ ] Preview, Submit-for-review, Edit, Delete actions all work.
- [ ] Click Operators. URL gains `?tab=operators`. Per-operator registrations render.
- [ ] Register at an operator. List refreshes in place (no navigation).
- [ ] Click Billing. URL gains `?tab=billing`. Billing history renders.
- [ ] Click History. URL gains `?tab=history`. Status transitions timeline shows (latest first).
- [ ] Arrow-left / Arrow-right keys switch tabs when focus is on a tab button.
- [ ] Deep-link `/sender-names/<id>?tab=billing` — billing tab active on mount.
- [ ] Unknown tab in URL (e.g. `?tab=whatever`) — defaults to overview.

## /templates (read-only)

- [ ] No "Создать шаблон" button anywhere on the page.
- [ ] Subtitle reads: "Справочник всех ваших шаблонов. Создание и редактирование — в карточке имени отправителя."
- [ ] No bulk-action checkboxes.
- [ ] Row actions: only "Превью" and "Перейти к имени".
- [ ] Clicking "Превью" opens preview modal with variables + render (as before).
- [ ] Clicking "Перейти к имени" navigates to `/sender-names/<sn.id>?tab=templates`.
- [ ] Templates with no sender_name_id show "без имени" label instead of the link.
- [ ] Empty state links to `/sender-names`.

## Legacy routes (deleted)

- [ ] `/sender-names/<id>/operators` — 404 (or React Router fallback).
- [ ] `/sender-registrations/<id>/billing` — 404.
- [ ] No in-app navigation button takes you to those URLs.

## Sidebar

- [ ] "Шаблоны" entry hover title says "Справочник всех шаблонов" (or similar).
- [ ] Clicking the entry lands on `/templates` (read-only directory).

## Regressions to check

- [ ] Existing sender names unaffected (status transitions still work).
- [ ] Existing templates unaffected (status badges correct).
- [ ] No console errors in dev tools on any tab switch or page load.

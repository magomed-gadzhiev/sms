# Viber Business Messages — Implementation Plan (STUB)

> **STATUS: BLOCKED — требуется бизнес-решение перед реализацией**

**Goal:** Добавить Viber Business Messages как канал в cascade-стратегии. Viber — второй по популярности мессенджер в СНГ/Восточной Европе.

**Architecture:** Аналогично WhatsApp — новый provider adapter, интеграция в cascade. Два пути: прямой Viber Partner или через агрегатора (BSG, SMS.ru).

---

## BLOCKERS — решить до начала разработки

### Блокер: Партнёрская модель
**Выбрать одну из двух опций:**

**Option A — Через агрегатора (рекомендуется для быстрого старта)**
- BSG, SMS.ru или Devino уже являются Viber Partners и предоставляют REST API
- Быстрый старт: 1-2 недели на интеграцию
- Маржа ниже: платим агрегатору + добавляем маржу
- Риск: зависимость от третьей стороны

**Option B — Прямой Viber Partner**
- Прямой доступ к Viber API
- Требует: подача заявки на Viber Partner, верификация бизнеса (4-8 недель)
- Маржа выше: прямые цены Viber
- Рекомендуется при объёмах >5M сообщений/месяц

**→ ACTION: Bizdev проводит discovery до 2026-04-30 и выбирает опцию**

---

## Высокоуровневые задачи (детализируются после разблокировки)

1. **Viber Provider Adapter** — имплементировать интерфейс Provider
2. **API Client** — HTTP client для Viber Business API (или агрегатора API)
3. **DLR Webhook** — Viber присылает delivery receipts
4. **DB migration** — добавить Viber в delivery_channels
5. **Frontend** — добавить Viber в UI стратегий

## Viber API (справочник)

После разблокировки использовать Viber REST API:

```
POST https://chatapi.viber.com/pa/send_message
X-Viber-Auth-Token: {AUTH_TOKEN}
```

```json
{
  "receiver": "79991234567",
  "type": "text",
  "text": "Ваш код: 1234",
  "sender": { "name": "MyCompany" }
}
```

## Как вернуться к этому плану

После выбора партнёрской модели и получения credentials:
1. Запустить детализацию через `superpowers:writing-plans`
2. Передать API endpoint, auth метод и sandbox credentials

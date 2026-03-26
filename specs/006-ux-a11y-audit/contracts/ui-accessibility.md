# UI Accessibility Contracts

**Branch**: `006-ux-a11y-audit` | **Date**: 2026-03-21

Контракты описывают ожидаемое поведение UI с точки зрения assistive technology — что именно должен "видеть" и "слышать" скринридер.

---

## Contract 1: Страница входа (LoginPage)

### Обычный вход

| Событие | AT-объявление (ожидается) |
|---------|--------------------------|
| Страница открыта | "Login, heading level 2" |
| Поле Email сфокусировано | "Email, required, edit text" |
| Поле Password сфокусировано | "Password, required, password edit text" |
| Форма отправлена с ошибкой | "Invalid credentials" (alert, немедленно) |
| Успешный вход | Переход на Dashboard, "Dashboard, heading level 2" |

### 2FA вход

| Событие | AT-объявление |
|---------|---------------|
| Форма 2FA показана | "Two-Factor Authentication, heading level 2" (автофокус) |
| TOTP поле сфокусировано | "TOTP Code, required, 6-digit numeric edit text" |
| Ошибка 2FA | "2FA verification failed" (alert) |

---

## Contract 2: API Keys Page

| Событие | AT-объявление |
|---------|---------------|
| Страница открыта | "API Keys, heading level 2" |
| Таблица встречена | "API Keys, table, N rows" |
| Кнопка Revoke сфокусирована | "Revoke Production Key, button" |
| Нажата Revoke | "Sure? Yes, revoke. Cancel." (inline confirmation) |
| После создания ключа | "API Key created successfully. Copy it now, it will not be shown again." (alert) |
| Форма создания открыта | Фокус → "Name, required, edit text" |
| Escape в форме | Форма закрыта, фокус → "Create API Key, button" |

---

## Contract 3: Навигация

| Событие | AT-объявление |
|---------|---------------|
| Навигация встречена | "Main navigation, navigation landmark" |
| Активная страница | "Dashboard, link, current page" |
| Неактивная страница | "Messages, link" |
| Кнопка Logout | "Logout, button" |
| Skip-link при Tab | "Skip to main content, link" (первое нажатие Tab на любой странице) |

---

## Contract 4: Таблица сообщений (MessagesPage)

| Элемент | AT-объявление |
|---------|---------------|
| Таблица | "Messages list, table, N rows, 7 columns" |
| Ячейка с обрезанным текстом | Полный текст через `title` или `aria-label` |
| Пустое состояние | "No messages found" (не скрыто от AT) |
| Кнопка Prev/Next | "Previous page, button, dimmed" / "Next page, button" |
| Информация о странице | "Page 1 of 5, total 87 messages" |

---

## Contract 5: Статусные индикаторы

Применяется к: API Keys, Webhooks, Sub-accounts.

| Визуал | AT-объявление |
|--------|---------------|
| Зелёный "Active" | "Status: Active" |
| Красный "Revoked" | "Status: Revoked" |
| Красный "Inactive" | "Status: Inactive" |

---

## Contract 6: Profile / 2FA Setup

| Событие | AT-объявление |
|---------|---------------|
| Секрет TOTP показан | "Or enter the secret manually: [секрет]. Copy secret, button." |
| 2FA включён | "2FA enabled successfully" (status, polite) |
| 2FA отключён | "2FA disabled" (status, polite) |
| Профиль сохранён | "Profile updated" (status, polite) |

---

## Contract 7: Загрузочные состояния

Применяется ко всем страницам.

| Элемент | AT-объявление |
|---------|---------------|
| Загрузка страницы | "Loading dashboard..." (status) |
| Загрузка данных | "Loading..." (status) |
| Загрузка завершена | AT переключается на контент страницы |

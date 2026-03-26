# Quickstart: Тестирование доступности SMS Portal

**Branch**: `006-ux-a11y-audit` | **Date**: 2026-03-21

---

## Запуск фронтенда

```bash
cd portal-frontend
npm install
npm run dev
# → http://localhost:5173
```

---

## Автоматическое тестирование (axe DevTools)

### Chrome Extension (рекомендуется)

1. Установить [axe DevTools](https://www.deque.com/axe/devtools/) (бесплатная версия)
2. Открыть DevTools → вкладка "axe DevTools"
3. Нажать "Analyze" на каждой из 10 страниц
4. Цель: **0 critical, 0 serious** нарушений

### Базовый запуск через консоль (без расширения)

```javascript
// В консоли DevTools:
const { axe } = await import('https://cdnjs.cloudflare.com/ajax/libs/axe-core/4.9.1/axe.min.js');
axe.run().then(results => console.log(results.violations));
```

---

## Ручное тестирование

### Клавиатурная навигация

1. Открыть страницу, убрать руки от мыши
2. Нажать `Tab` — первый элемент должен быть Skip-link "Skip to main content"
3. Нажать `Tab` ещё раз — фокус должен перейти в навигацию
4. Пройти Tab по всем 8 пунктам навигации — каждый должен иметь видимый outline
5. Нажать `Enter` на пункте — должен произойти переход
6. На странице API Keys: нажать "Create API Key" → Tab внутри формы → фокус не должен выходить наружу
7. Нажать `Escape` → форма закрыта, фокус вернулся на кнопку "Create API Key"

### Тестирование со скринридером

**macOS (VoiceOver)**:
```
⌘+F5 — включить/выключить VoiceOver
VO+Right (^⌥→) — следующий элемент
VO+U — открыть ротор (Web Rotor) для навигации по landmarks
```

**Windows (NVDA, бесплатный)**:
```
NVDA+N — открыть меню NVDA
Tab — навигация по интерактивным элементам
H — следующий заголовок
T — следующая таблица
R — следующая строка таблицы
```

**Ключевые сценарии для проверки**:
1. Войти в систему — ошибка должна быть озвучена без ручного перемещения фокуса
2. Создать API Key — банер должен быть озвучен сразу при появлении
3. Перейти по навигации — активный пункт должен объявляться как "current page"

---

## Проверка контрастности

```
Chrome DevTools → Elements → Styles →
Кликнуть на цветной кружок → "Contrast ratio"
```

Целевые значения:
- Обычный текст: ≥ 4.5:1
- Крупный текст (18pt+ или 14pt bold): ≥ 3:1
- UI-компоненты (border input, иконки): ≥ 3:1

---

## Чеклист финальной проверки

- [ ] axe DevTools: 0 critical/serious на всех 10 страницах
- [ ] Skip-link присутствует и работает (Tab → Enter → фокус в `#main-content`)
- [ ] Активный пункт навигации: видимый outline + `aria-current="page"` в DevTools
- [ ] Форма создания API Key: Tab-фокус замкнут внутри формы; Escape закрывает
- [ ] Банер API Key: объявляется скринридером без ручного перемещения фокуса
- [ ] Все таблицы: в DevTools видны `<caption>` элементы
- [ ] Кнопки "Revoke"/"Delete": `aria-label` содержит имя ресурса
- [ ] Статус "Active/Revoked": не только цвет, но и `aria-label`
- [ ] Текст сообщений в Messages: `title` атрибут с полным текстом
- [ ] Загрузочные состояния: `role="status"` виден в DOM

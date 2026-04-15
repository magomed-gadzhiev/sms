# Landing Page + Pricing + SEO Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a public-facing website (landing, pricing, features, docs, blog, about, contact) to the existing portal-frontend SPA, with i18n (RU/EN) and SEO prerendering.

**Architecture:** Public pages share the portal-frontend codebase but use a separate `PublicLayout` (header + footer, no sidebar, no auth). React Router handles both public and authenticated routes. `react-i18next` provides RU/EN language switching via URL prefix (`/en/*`). `react-helmet-async` manages SEO meta tags. Markdown content (blog, docs) is loaded at build time via Vite glob imports.

**Tech Stack:** React 19, React Router 7.1, Vite 6, Tailwind CSS 4.2, Lucide React (icons), react-i18next (i18n), react-helmet-async (SEO), unified/remark/rehype (markdown), gray-matter (frontmatter)

**Spec:** `docs/superpowers/specs/2026-04-15-landing-page-design.md`

---

## Phase 1: Foundation (Dependencies, i18n, Layout, Routing)

### Task 1: Install Dependencies

**Files:**
- Modify: `portal-frontend/package.json`

- [ ] **Step 1: Install production dependencies**

```bash
cd portal-frontend
npm install lucide-react react-i18next i18next react-helmet-async unified remark-parse remark-gfm remark-rehype rehype-stringify rehype-highlight gray-matter
```

- [ ] **Step 2: Verify installation**

```bash
cd portal-frontend && npm ls lucide-react react-i18next react-helmet-async unified gray-matter --depth=0
```

Expected: All 5 packages listed without errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/package.json portal-frontend/package-lock.json
git commit -m "chore: add landing page dependencies (lucide, i18next, helmet, markdown)"
```

---

### Task 2: Setup i18n

**Files:**
- Create: `portal-frontend/src/i18n/ru.json`
- Create: `portal-frontend/src/i18n/en.json`
- Create: `portal-frontend/src/i18n/index.ts`

- [ ] **Step 1: Create Russian translations**

Create `portal-frontend/src/i18n/ru.json`:

```json
{
  "nav": {
    "features": "Возможности",
    "pricing": "Тарифы",
    "docs": "Документация",
    "blog": "Блог",
    "about": "О нас",
    "contact": "Контакты",
    "login": "Войти",
    "getStarted": "Начать"
  },
  "hero": {
    "title": "SMS-платформа для вашего бизнеса",
    "subtitle": "Подключайте своих провайдеров, управляйте клиентами, масштабируйте доставку. Всё в одной системе.",
    "cta": "Начать бесплатно",
    "docs": "Документация API"
  },
  "stats": {
    "sms": "SMS в месяц",
    "uptime": "Uptime",
    "providers": "Провайдеров",
    "resellers": "Реселлеров"
  },
  "features": {
    "title": "Ключевые возможности",
    "subtitle": "Всё, что нужно для SMS-бизнеса",
    "providers": {
      "title": "Свои провайдеры",
      "desc": "SMPP и HTTP — подключайте любых операторов мира"
    },
    "multiTenant": {
      "title": "Multi-tenant",
      "desc": "Sub-accounts, индивидуальные тарифы для каждого клиента"
    },
    "routing": {
      "title": "Маршрутизация",
      "desc": "Правила по стране, оператору и стоимости"
    },
    "analytics": {
      "title": "Аналитика",
      "desc": "Детализация, статистика, экспорт отчётов"
    },
    "billing": {
      "title": "Биллинг",
      "desc": "Тарификация клиентов, баланс, транзакции"
    },
    "api": {
      "title": "API & Webhooks",
      "desc": "REST API, колбеки доставки, полная интеграция"
    }
  },
  "howItWorks": {
    "title": "Как это работает",
    "step1": { "title": "Регистрация", "desc": "Создайте аккаунт и настройте компанию" },
    "step2": { "title": "Подключите провайдеров", "desc": "SMPP или HTTP — добавьте своих операторов" },
    "step3": { "title": "Отправляйте", "desc": "API, веб-панель или SMPP — выбирайте удобный способ" }
  },
  "screenshot": {
    "title": "Мощная панель управления",
    "alt": "Скриншот дашборда SMS-платформы"
  },
  "pricingPreview": {
    "title": "Тарифные планы",
    "popular": "Популярный",
    "perMonth": "/мес",
    "custom": "По запросу",
    "moreDetails": "Подробнее о тарифах →",
    "smsIncluded": "SMS включено",
    "cta": "Начать"
  },
  "finalCta": {
    "title": "Запустите свой SMS-бизнес за 10 минут",
    "subtitle": "Бесплатная регистрация. Без обязательств.",
    "cta": "Создать аккаунт бесплатно"
  },
  "footer": {
    "product": "Продукт",
    "company": "Компания",
    "legal": "Правовое",
    "support": "Поддержка",
    "terms": "Условия использования",
    "privacy": "Конфиденциальность",
    "telegram": "Telegram",
    "email": "Email",
    "faq": "FAQ",
    "copyright": "© 2026 SMS Platform. Все права защищены."
  },
  "pricing": {
    "title": "Тарифные планы",
    "subtitle": "Выберите план, подходящий для вашего бизнеса",
    "starter": {
      "name": "Starter",
      "desc": "Для начинающих реселлеров",
      "price": "$XX",
      "smsLimit": "50,000",
      "subAccounts": "1 sub-account",
      "providerLimit": "3 провайдера",
      "features": ["API + веб-панель", "Email-поддержка", "Базовая аналитика"]
    },
    "business": {
      "name": "Business",
      "desc": "Для растущего бизнеса",
      "price": "$XX",
      "smsLimit": "500,000",
      "subAccounts": "20 sub-accounts",
      "providerLimit": "10 провайдеров",
      "features": ["Маршрутизация по правилам", "Каскадная доставка", "Кампании и шаблоны", "Аналитика + экспорт", "Приоритетная поддержка"]
    },
    "enterprise": {
      "name": "Enterprise",
      "desc": "Для крупных агрегаторов",
      "price": "Custom",
      "smsLimit": "Безлимит",
      "subAccounts": "Безлимит sub-accounts",
      "providerLimit": "Безлимит провайдеров",
      "features": ["SMPP-доступ", "HLR-lookup", "SLA + выделенный менеджер", "Кастомная интеграция"]
    },
    "compareTitle": "Сравнение планов",
    "faqTitle": "Часто задаваемые вопросы",
    "faq": [
      { "q": "Что если я превышу лимит?", "a": "Сверх лимита SMS тарифицируются по ставке вашего плана. Вы всегда видите текущее потребление в панели управления." },
      { "q": "Как перейти на другой план?", "a": "Перейти на более высокий план можно в любой момент из настроек. Разница в стоимости пересчитывается пропорционально." },
      { "q": "Есть ли пробный период?", "a": "Да, план Starter доступен бесплатно на 14 дней с полным функционалом." },
      { "q": "Как происходит оплата?", "a": "Ежемесячная или годовая подписка. Принимаем банковские переводы и карты." }
    ],
    "calculatorTitle": "Калькулятор стоимости",
    "calculatorLabel": "Объём SMS в месяц",
    "calculatorResult": "Примерная стоимость",
    "overageRate": 0.005
  },
  "featuresPage": {
    "title": "Всё для SMS-бизнеса",
    "subtitle": "Полный набор инструментов для запуска и масштабирования вашего SMS-сервиса",
    "items": [
      { "title": "Подключение провайдеров", "desc": "Поддержка SMPP и HTTP протоколов. Подключайте любых операторов мира. Мониторинг статуса соединений в реальном времени. Автоматическое переподключение при обрывах." },
      { "title": "Multi-tenant / Sub-accounts", "desc": "Создавайте клиентские аккаунты с индивидуальными тарифами. Контролируйте балансы и лимиты каждого клиента. Полная изоляция данных между аккаунтами." },
      { "title": "Маршрутизация", "desc": "Гибкие правила маршрутизации по стране, оператору и стоимости. Приоритеты и fallback-маршруты. Балансировка нагрузки между провайдерами." },
      { "title": "Каскадная доставка", "desc": "Автоматический fallback при недоставке. SMS → Viber → другие каналы. Настраиваемые стратегии доставки с таймаутами." },
      { "title": "Кампании и рассылки", "desc": "Массовая отправка по спискам контактов. Шаблоны с переменными. Расписание отправки. Сегментация аудитории." },
      { "title": "API и интеграции", "desc": "REST API для полной автоматизации. Webhooks для получения статусов доставки. SMPP-интерфейс для высоконагруженных интеграций." },
      { "title": "Аналитика и детализация", "desc": "Дашборд с ключевыми метриками. Статистика по провайдерам и клиентам. Экспорт отчётов в CSV. Мониторинг в реальном времени." },
      { "title": "Биллинг и тарификация", "desc": "Тарифы по направлениям. Автоматическое списание с баланса. Полная история транзакций. Детализация по каждому сообщению." },
      { "title": "Sender Names", "desc": "Управление именами отправителей. Отслеживание статусов регистрации у операторов. Автоматические уведомления о смене статуса." },
      { "title": "HLR-Lookup", "desc": "Проверка номеров перед отправкой. Определение оператора и статуса номера. Экономия на невалидных номерах до 15%." }
    ]
  },
  "about": {
    "title": "О платформе",
    "mission": "Мы создаём инфраструктуру, которая позволяет SMS-агрегаторам и реселлерам запускать и масштабировать свой бизнес без необходимости разрабатывать собственную платформу.",
    "mission2": "Наша платформа обрабатывает миллионы сообщений ежедневно, обеспечивая надёжную доставку через любых операторов мира.",
    "statsTitle": "Платформа в цифрах"
  },
  "contact": {
    "title": "Свяжитесь с нами",
    "subtitle": "Есть вопросы? Мы будем рады помочь.",
    "name": "Имя",
    "email": "Email",
    "company": "Компания",
    "message": "Сообщение",
    "send": "Отправить",
    "sending": "Отправка...",
    "success": "Сообщение отправлено! Мы свяжемся с вами в ближайшее время.",
    "error": "Ошибка отправки. Попробуйте позже.",
    "directTitle": "Или напишите напрямую"
  },
  "seo": {
    "landing": { "title": "SMS Platform — SMS-платформа для реселлеров", "desc": "PaaS для SMS-бизнеса. Подключайте своих провайдеров, управляйте клиентами, масштабируйте доставку." },
    "pricing": { "title": "Тарифы — SMS Platform", "desc": "Тарифные планы SMS-платформы: Starter, Business, Enterprise. Гибкая модель оплаты." },
    "features": { "title": "Возможности — SMS Platform", "desc": "Провайдеры, маршрутизация, каскадная доставка, API, аналитика, биллинг — всё для SMS-бизнеса." },
    "docs": { "title": "Документация API — SMS Platform", "desc": "API-документация SMS-платформы: отправка, статусы, контакты, кампании." },
    "blog": { "title": "Блог — SMS Platform", "desc": "Обновления платформы, гайды для реселлеров, новости индустрии." },
    "about": { "title": "О нас — SMS Platform", "desc": "SMS-платформа для агрегаторов и реселлеров. Инфраструктура для вашего SMS-бизнеса." },
    "contact": { "title": "Контакты — SMS Platform", "desc": "Свяжитесь с нами: форма обратной связи, email, Telegram." }
  }
}
```

- [ ] **Step 2: Create English translations**

Create `portal-frontend/src/i18n/en.json`:

```json
{
  "nav": {
    "features": "Features",
    "pricing": "Pricing",
    "docs": "Docs",
    "blog": "Blog",
    "about": "About",
    "contact": "Contact",
    "login": "Log in",
    "getStarted": "Get Started"
  },
  "hero": {
    "title": "SMS Platform for Your Business",
    "subtitle": "Connect your providers, manage clients, scale delivery. All in one system.",
    "cta": "Start for Free",
    "docs": "API Documentation"
  },
  "stats": {
    "sms": "SMS per month",
    "uptime": "Uptime",
    "providers": "Providers",
    "resellers": "Resellers"
  },
  "features": {
    "title": "Key Features",
    "subtitle": "Everything you need for SMS business",
    "providers": { "title": "Your Providers", "desc": "SMPP and HTTP — connect any operator worldwide" },
    "multiTenant": { "title": "Multi-tenant", "desc": "Sub-accounts with individual tariffs for each client" },
    "routing": { "title": "Routing", "desc": "Rules by country, operator, and cost" },
    "analytics": { "title": "Analytics", "desc": "Detailed reports, statistics, CSV export" },
    "billing": { "title": "Billing", "desc": "Client tarification, balance, transactions" },
    "api": { "title": "API & Webhooks", "desc": "REST API, delivery callbacks, full integration" }
  },
  "howItWorks": {
    "title": "How It Works",
    "step1": { "title": "Register", "desc": "Create an account and set up your company" },
    "step2": { "title": "Connect Providers", "desc": "SMPP or HTTP — add your operators" },
    "step3": { "title": "Start Sending", "desc": "API, web panel, or SMPP — choose your way" }
  },
  "screenshot": {
    "title": "Powerful Control Panel",
    "alt": "SMS platform dashboard screenshot"
  },
  "pricingPreview": {
    "title": "Pricing Plans",
    "popular": "Popular",
    "perMonth": "/mo",
    "custom": "Custom",
    "moreDetails": "View all plans →",
    "smsIncluded": "SMS included",
    "cta": "Get Started"
  },
  "finalCta": {
    "title": "Launch Your SMS Business in 10 Minutes",
    "subtitle": "Free registration. No commitments.",
    "cta": "Create Free Account"
  },
  "footer": {
    "product": "Product",
    "company": "Company",
    "legal": "Legal",
    "support": "Support",
    "terms": "Terms of Service",
    "privacy": "Privacy Policy",
    "telegram": "Telegram",
    "email": "Email",
    "faq": "FAQ",
    "copyright": "© 2026 SMS Platform. All rights reserved."
  },
  "pricing": {
    "title": "Pricing Plans",
    "subtitle": "Choose the plan that fits your business",
    "starter": {
      "name": "Starter",
      "desc": "For new resellers",
      "price": "$XX",
      "smsLimit": "50,000",
      "subAccounts": "1 sub-account",
      "providerLimit": "3 providers",
      "features": ["API + web panel", "Email support", "Basic analytics"]
    },
    "business": {
      "name": "Business",
      "desc": "For growing business",
      "price": "$XX",
      "smsLimit": "500,000",
      "subAccounts": "20 sub-accounts",
      "providerLimit": "10 providers",
      "features": ["Rule-based routing", "Cascade delivery", "Campaigns & templates", "Analytics + export", "Priority support"]
    },
    "enterprise": {
      "name": "Enterprise",
      "desc": "For large aggregators",
      "price": "Custom",
      "smsLimit": "Unlimited",
      "subAccounts": "Unlimited sub-accounts",
      "providerLimit": "Unlimited providers",
      "features": ["SMPP access", "HLR lookup", "SLA + dedicated manager", "Custom integration"]
    },
    "compareTitle": "Plan Comparison",
    "faqTitle": "Frequently Asked Questions",
    "faq": [
      { "q": "What if I exceed the limit?", "a": "Overage SMS are billed at your plan's per-SMS rate. You can always see your current usage in the dashboard." },
      { "q": "How do I upgrade my plan?", "a": "You can upgrade at any time from settings. The cost difference is prorated." },
      { "q": "Is there a trial period?", "a": "Yes, the Starter plan is free for 14 days with full functionality." },
      { "q": "How does billing work?", "a": "Monthly or annual subscription. We accept bank transfers and cards." }
    ],
    "calculatorTitle": "Cost Calculator",
    "calculatorLabel": "SMS volume per month",
    "calculatorResult": "Estimated cost",
    "overageRate": 0.005
  },
  "featuresPage": {
    "title": "Everything for SMS Business",
    "subtitle": "Complete toolkit for launching and scaling your SMS service",
    "items": [
      { "title": "Provider Integration", "desc": "SMPP and HTTP protocol support. Connect any operator worldwide. Real-time connection monitoring. Automatic reconnection on failures." },
      { "title": "Multi-tenant / Sub-accounts", "desc": "Create client accounts with individual tariffs. Control balances and limits. Complete data isolation between accounts." },
      { "title": "Routing", "desc": "Flexible routing rules by country, operator, and cost. Priority and fallback routes. Load balancing across providers." },
      { "title": "Cascade Delivery", "desc": "Automatic fallback on delivery failure. SMS → Viber → other channels. Configurable delivery strategies with timeouts." },
      { "title": "Campaigns & Broadcasts", "desc": "Mass sending to contact lists. Templates with variables. Scheduled delivery. Audience segmentation." },
      { "title": "API & Integrations", "desc": "REST API for full automation. Webhooks for delivery status. SMPP interface for high-throughput integrations." },
      { "title": "Analytics & Reporting", "desc": "Dashboard with key metrics. Statistics by provider and client. CSV report export. Real-time monitoring." },
      { "title": "Billing & Tarification", "desc": "Destination-based tariffs. Automatic balance deduction. Full transaction history. Per-message breakdown." },
      { "title": "Sender Names", "desc": "Sender name management. Registration status tracking with operators. Automatic status change notifications." },
      { "title": "HLR Lookup", "desc": "Number validation before sending. Operator and status detection. Save up to 15% on invalid numbers." }
    ]
  },
  "about": {
    "title": "About the Platform",
    "mission": "We build infrastructure that enables SMS aggregators and resellers to launch and scale their business without developing their own platform.",
    "mission2": "Our platform processes millions of messages daily, ensuring reliable delivery through any operator worldwide.",
    "statsTitle": "Platform in Numbers"
  },
  "contact": {
    "title": "Contact Us",
    "subtitle": "Have questions? We'd love to help.",
    "name": "Name",
    "email": "Email",
    "company": "Company",
    "message": "Message",
    "send": "Send",
    "sending": "Sending...",
    "success": "Message sent! We'll get back to you shortly.",
    "error": "Failed to send. Please try again later.",
    "directTitle": "Or reach us directly"
  },
  "seo": {
    "landing": { "title": "SMS Platform — SMS Platform for Resellers", "desc": "PaaS for SMS business. Connect your providers, manage clients, scale delivery." },
    "pricing": { "title": "Pricing — SMS Platform", "desc": "SMS platform pricing: Starter, Business, Enterprise. Flexible billing model." },
    "features": { "title": "Features — SMS Platform", "desc": "Providers, routing, cascade delivery, API, analytics, billing — everything for SMS business." },
    "docs": { "title": "API Documentation — SMS Platform", "desc": "SMS platform API docs: sending, statuses, contacts, campaigns." },
    "blog": { "title": "Blog — SMS Platform", "desc": "Platform updates, reseller guides, industry news." },
    "about": { "title": "About — SMS Platform", "desc": "SMS platform for aggregators and resellers. Infrastructure for your SMS business." },
    "contact": { "title": "Contact — SMS Platform", "desc": "Contact us: feedback form, email, Telegram." }
  }
}
```

- [ ] **Step 3: Create i18n configuration**

Create `portal-frontend/src/i18n/index.ts`:

```typescript
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import ru from './ru.json';
import en from './en.json';

i18n.use(initReactI18next).init({
  resources: { ru: { translation: ru }, en: { translation: en } },
  lng: localStorage.getItem('lang') || 'ru',
  fallbackLng: 'ru',
  interpolation: { escapeValue: false },
});

export default i18n;
```

- [ ] **Step 4: Import i18n in main.tsx**

In `portal-frontend/src/main.tsx`, add `import './i18n';` before the `import './index.css';` line (first line of imports).

- [ ] **Step 5: Verify dev server starts**

```bash
cd portal-frontend && npm run dev
```

Expected: Compiles without errors, dev server starts on port 3001.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/i18n/
git add portal-frontend/src/main.tsx
git commit -m "feat: add i18n setup with RU/EN translations"
```

---

### Task 3: Create PublicLayout (Header + Footer)

**Files:**
- Create: `portal-frontend/src/components/public/Header.tsx`
- Create: `portal-frontend/src/components/public/Footer.tsx`
- Create: `portal-frontend/src/components/layout/PublicLayout.tsx`

- [ ] **Step 1: Create Header component**

Create `portal-frontend/src/components/public/Header.tsx`:

```tsx
import { useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Menu, X } from 'lucide-react';

export function Header() {
  const { t, i18n } = useTranslation();
  const location = useLocation();
  const [mobileOpen, setMobileOpen] = useState(false);

  const isEn = location.pathname.startsWith('/en');
  const prefix = isEn ? '/en' : '';

  const links = [
    { to: `${prefix}/features`, label: t('nav.features') },
    { to: `${prefix}/pricing`, label: t('nav.pricing') },
    { to: `${prefix}/docs`, label: t('nav.docs') },
    { to: `${prefix}/blog`, label: t('nav.blog') },
    { to: `${prefix}/about`, label: t('nav.about') },
  ];

  function toggleLang() {
    const newLang = isEn ? 'ru' : 'en';
    i18n.changeLanguage(newLang);
    localStorage.setItem('lang', newLang);

    const path = location.pathname.replace(/^\/en/, '') || '/';
    if (newLang === 'en') {
      window.location.href = `/en${path}`;
    } else {
      window.location.href = path;
    }
  }

  return (
    <header className="fixed top-0 left-0 right-0 z-50 bg-white/80 backdrop-blur-md border-b border-gray-100">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 flex items-center justify-between h-16">
        <Link to={prefix || '/'} className="text-xl font-bold bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent">
          SMS Platform
        </Link>

        {/* Desktop nav */}
        <nav className="hidden md:flex items-center gap-6">
          {links.map((l) => (
            <Link key={l.to} to={l.to} className="text-sm text-gray-600 hover:text-indigo-600 transition-colors">
              {l.label}
            </Link>
          ))}
        </nav>

        <div className="hidden md:flex items-center gap-3">
          <button onClick={toggleLang} className="text-sm text-gray-500 hover:text-gray-700 px-2 py-1 rounded">
            {isEn ? 'RU' : 'EN'}
          </button>
          <Link to="/login" className="text-sm text-gray-600 hover:text-indigo-600">
            {t('nav.login')}
          </Link>
          <Link to="/register" className="text-sm bg-gradient-to-r from-indigo-600 to-purple-600 text-white px-4 py-2 rounded-xl hover:opacity-90 transition-opacity">
            {t('nav.getStarted')}
          </Link>
        </div>

        {/* Mobile hamburger */}
        <button onClick={() => setMobileOpen(!mobileOpen)} className="md:hidden text-gray-600" aria-label="Menu">
          {mobileOpen ? <X size={24} /> : <Menu size={24} />}
        </button>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="md:hidden bg-white border-t border-gray-100 px-4 pb-4">
          {links.map((l) => (
            <Link key={l.to} to={l.to} onClick={() => setMobileOpen(false)} className="block py-2 text-gray-600 hover:text-indigo-600">
              {l.label}
            </Link>
          ))}
          <div className="flex items-center gap-3 pt-3 border-t border-gray-100 mt-2">
            <button onClick={toggleLang} className="text-sm text-gray-500">{isEn ? 'RU' : 'EN'}</button>
            <Link to="/login" className="text-sm text-gray-600">{t('nav.login')}</Link>
            <Link to="/register" className="text-sm bg-gradient-to-r from-indigo-600 to-purple-600 text-white px-4 py-2 rounded-xl">
              {t('nav.getStarted')}
            </Link>
          </div>
        </div>
      )}
    </header>
  );
}
```

- [ ] **Step 2: Create Footer component**

Create `portal-frontend/src/components/public/Footer.tsx`:

```tsx
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function Footer() {
  const { t } = useTranslation();
  const location = useLocation();
  const prefix = location.pathname.startsWith('/en') ? '/en' : '';

  const columns = [
    {
      title: t('footer.product'),
      links: [
        { to: `${prefix}/features`, label: t('nav.features') },
        { to: `${prefix}/pricing`, label: t('nav.pricing') },
        { to: `${prefix}/docs`, label: t('nav.docs') },
      ],
    },
    {
      title: t('footer.company'),
      links: [
        { to: `${prefix}/about`, label: t('nav.about') },
        { to: `${prefix}/blog`, label: t('nav.blog') },
        { to: `${prefix}/contact`, label: t('nav.contact') },
      ],
    },
    {
      title: t('footer.legal'),
      links: [
        { to: '#', label: t('footer.terms') },
        { to: '#', label: t('footer.privacy') },
      ],
    },
  ];

  return (
    <footer className="bg-gray-900 text-gray-300">
      <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-12">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8">
          {columns.map((col) => (
            <div key={col.title}>
              <h4 className="text-sm font-semibold text-white mb-3">{col.title}</h4>
              <ul className="space-y-2">
                {col.links.map((l) => (
                  <li key={l.to + l.label}>
                    <Link to={l.to} className="text-sm text-gray-400 hover:text-white transition-colors">
                      {l.label}
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
          <div>
            <h4 className="text-sm font-semibold text-white mb-3">{t('footer.support')}</h4>
            <ul className="space-y-2">
              <li>
                <a href="https://t.me/sms_support" target="_blank" rel="noopener noreferrer" className="text-sm text-gray-400 hover:text-white transition-colors">
                  {t('footer.telegram')}
                </a>
              </li>
              <li>
                <a href="mailto:support@smsplatform.com" className="text-sm text-gray-400 hover:text-white transition-colors">
                  {t('footer.email')}
                </a>
              </li>
            </ul>
          </div>
        </div>
        <div className="border-t border-gray-800 mt-10 pt-6 text-center text-sm text-gray-500">
          {t('footer.copyright')}
        </div>
      </div>
    </footer>
  );
}
```

- [ ] **Step 3: Create PublicLayout**

Create `portal-frontend/src/components/layout/PublicLayout.tsx`:

```tsx
import { Outlet, useLocation } from 'react-router-dom';
import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Header } from '../public/Header';
import { Footer } from '../public/Footer';

export function PublicLayout() {
  const { i18n } = useTranslation();
  const location = useLocation();

  useEffect(() => {
    const isEn = location.pathname.startsWith('/en');
    const lang = isEn ? 'en' : 'ru';
    if (i18n.language !== lang) {
      i18n.changeLanguage(lang);
      localStorage.setItem('lang', lang);
    }
    document.documentElement.lang = lang;
  }, [location.pathname, i18n]);

  return (
    <div className="min-h-screen flex flex-col">
      <Header />
      <main className="flex-1 pt-16">
        <Outlet />
      </main>
      <Footer />
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add portal-frontend/src/components/public/ portal-frontend/src/components/layout/PublicLayout.tsx
git commit -m "feat: add PublicLayout with Header and Footer components"
```

---

### Task 4: Add Public Routes to App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Create placeholder pages**

Create `portal-frontend/src/pages/public/LandingPage.tsx`:

```tsx
export function LandingPage() {
  return <div className="p-8 text-center text-gray-500">Landing Page — coming soon</div>;
}
```

Create the same minimal placeholder for each page:
- `portal-frontend/src/pages/public/PricingPage.tsx` (export `PricingPage`)
- `portal-frontend/src/pages/public/FeaturesPage.tsx` (export `FeaturesPage`)
- `portal-frontend/src/pages/public/DocsPage.tsx` (export `DocsPage`)
- `portal-frontend/src/pages/public/DocArticlePage.tsx` (export `DocArticlePage`)
- `portal-frontend/src/pages/public/BlogPage.tsx` (export `BlogPage`)
- `portal-frontend/src/pages/public/BlogPostPage.tsx` (export `BlogPostPage`)
- `portal-frontend/src/pages/public/AboutPage.tsx` (export `AboutPage`)
- `portal-frontend/src/pages/public/ContactPage.tsx` (export `ContactPage`)

- [ ] **Step 2: Add public routes to App.tsx**

Add these imports at the top of `portal-frontend/src/App.tsx`:

```tsx
import { PublicLayout } from './components/layout/PublicLayout';
import { LandingPage } from './pages/public/LandingPage';
import { PricingPage } from './pages/public/PricingPage';
import { FeaturesPage } from './pages/public/FeaturesPage';
import { DocsPage } from './pages/public/DocsPage';
import { DocArticlePage } from './pages/public/DocArticlePage';
import { BlogPage } from './pages/public/BlogPage';
import { BlogPostPage } from './pages/public/BlogPostPage';
import { AboutPage } from './pages/public/AboutPage';
import { ContactPage } from './pages/public/ContactPage';
```

Add a public route group **before** the auth routes in the `<Routes>`:

```tsx
{/* Public pages */}
<Route element={<PublicLayout />}>
  <Route path="/" element={<LandingPage />} />
  <Route path="/pricing" element={<PricingPage />} />
  <Route path="/features" element={<FeaturesPage />} />
  <Route path="/docs" element={<DocsPage />} />
  <Route path="/docs/:slug" element={<DocArticlePage />} />
  <Route path="/blog" element={<BlogPage />} />
  <Route path="/blog/:slug" element={<BlogPostPage />} />
  <Route path="/about" element={<AboutPage />} />
  <Route path="/contact" element={<ContactPage />} />

  {/* EN versions */}
  <Route path="/en" element={<LandingPage />} />
  <Route path="/en/pricing" element={<PricingPage />} />
  <Route path="/en/features" element={<FeaturesPage />} />
  <Route path="/en/docs" element={<DocsPage />} />
  <Route path="/en/docs/:slug" element={<DocArticlePage />} />
  <Route path="/en/blog" element={<BlogPage />} />
  <Route path="/en/blog/:slug" element={<BlogPostPage />} />
  <Route path="/en/about" element={<AboutPage />} />
  <Route path="/en/contact" element={<ContactPage />} />
</Route>
```

- [ ] **Step 3: Wrap app with HelmetProvider**

In `portal-frontend/src/main.tsx`, add `import { HelmetProvider } from 'react-helmet-async';` and wrap the entire app:

```tsx
<StrictMode>
  <ErrorBoundary>
    <HelmetProvider>
      <BrowserRouter>
        <AuthProvider>
          <ToastProvider>
            <App />
          </ToastProvider>
        </AuthProvider>
      </BrowserRouter>
    </HelmetProvider>
  </ErrorBoundary>
</StrictMode>
```

- [ ] **Step 4: Verify routes work**

```bash
cd portal-frontend && npm run dev
```

Open `http://localhost:3001/` — should show placeholder landing page with Header and Footer.
Open `http://localhost:3001/pricing` — should show placeholder pricing page.
Open `http://localhost:3001/login` — should still show the login page (existing flow).

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/public/ portal-frontend/src/App.tsx portal-frontend/src/main.tsx
git commit -m "feat: add public routes with PublicLayout and placeholder pages"
```

---

## Phase 2: Landing Page

### Task 5: Build Landing Page — Hero + Stats

**Files:**
- Create: `portal-frontend/src/components/public/Hero.tsx`
- Create: `portal-frontend/src/components/public/Stats.tsx`
- Modify: `portal-frontend/src/pages/public/LandingPage.tsx`

- [ ] **Step 1: Create Hero component**

Create `portal-frontend/src/components/public/Hero.tsx`:

```tsx
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Helmet } from 'react-helmet-async';

export function Hero() {
  const { t } = useTranslation();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <>
      <Helmet>
        <title>{t('seo.landing.title')}</title>
        <meta name="description" content={t('seo.landing.desc')} />
        <meta property="og:title" content={t('seo.landing.title')} />
        <meta property="og:description" content={t('seo.landing.desc')} />
      </Helmet>
      <section className="relative overflow-hidden bg-gradient-to-br from-indigo-900 via-indigo-700 to-purple-600 text-white py-24 sm:py-32">
        <div className="absolute inset-0 opacity-20">
          <div className="absolute inset-0" style={{
            backgroundImage: 'radial-gradient(circle at 25% 25%, rgba(255,255,255,0.15) 0%, transparent 50%), radial-gradient(circle at 75% 75%, rgba(255,255,255,0.1) 0%, transparent 50%)'
          }} />
        </div>
        <div className="relative max-w-4xl mx-auto px-4 sm:px-6 text-center">
          <h1 className="text-4xl sm:text-5xl lg:text-6xl font-bold tracking-tight mb-6">
            {t('hero.title')}
          </h1>
          <p className="text-lg sm:text-xl text-indigo-100 max-w-2xl mx-auto mb-10">
            {t('hero.subtitle')}
          </p>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
            <Link to="/register" className="w-full sm:w-auto bg-white text-indigo-700 font-semibold px-8 py-3 rounded-xl hover:bg-indigo-50 transition-colors text-center">
              {t('hero.cta')}
            </Link>
            <Link to={`${prefix}/docs`} className="w-full sm:w-auto border border-white/30 text-white font-semibold px-8 py-3 rounded-xl hover:bg-white/10 transition-colors text-center">
              {t('hero.docs')}
            </Link>
          </div>
        </div>
      </section>
    </>
  );
}
```

- [ ] **Step 2: Create Stats component**

Create `portal-frontend/src/components/public/Stats.tsx`:

```tsx
import { useTranslation } from 'react-i18next';

const STATS = [
  { value: '500M+', key: 'sms' },
  { value: '99.9%', key: 'uptime' },
  { value: '50+', key: 'providers' },
  { value: '200+', key: 'resellers' },
];

export function Stats() {
  const { t } = useTranslation();

  return (
    <section className="bg-gray-50 py-10 border-b border-gray-100">
      <div className="max-w-5xl mx-auto px-4 grid grid-cols-2 md:grid-cols-4 gap-8 text-center">
        {STATS.map((s) => (
          <div key={s.key}>
            <div className="text-3xl font-bold text-gray-900">{s.value}</div>
            <div className="text-sm text-gray-500 mt-1">{t(`stats.${s.key}`)}</div>
          </div>
        ))}
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Update LandingPage**

Replace `portal-frontend/src/pages/public/LandingPage.tsx`:

```tsx
import { Hero } from '../../components/public/Hero';
import { Stats } from '../../components/public/Stats';

export function LandingPage() {
  return (
    <>
      <Hero />
      <Stats />
    </>
  );
}
```

- [ ] **Step 4: Verify in browser**

Open `http://localhost:3001/` — Hero section with gradient background and Stats bar should render.

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/components/public/Hero.tsx portal-frontend/src/components/public/Stats.tsx portal-frontend/src/pages/public/LandingPage.tsx
git commit -m "feat: add Hero and Stats sections to landing page"
```

---

### Task 6: Build Landing Page — Features Grid + How It Works

**Files:**
- Create: `portal-frontend/src/components/public/FeaturesGrid.tsx`
- Create: `portal-frontend/src/components/public/HowItWorks.tsx`
- Modify: `portal-frontend/src/pages/public/LandingPage.tsx`

- [ ] **Step 1: Create FeaturesGrid component**

Create `portal-frontend/src/components/public/FeaturesGrid.tsx`:

```tsx
import { useTranslation } from 'react-i18next';
import { Plug, Users, GitBranch, BarChart3, CreditCard, Webhook } from 'lucide-react';

const FEATURES = [
  { icon: Plug, key: 'providers' },
  { icon: Users, key: 'multiTenant' },
  { icon: GitBranch, key: 'routing' },
  { icon: BarChart3, key: 'analytics' },
  { icon: CreditCard, key: 'billing' },
  { icon: Webhook, key: 'api' },
];

export function FeaturesGrid() {
  const { t } = useTranslation();

  return (
    <section className="py-20">
      <div className="max-w-6xl mx-auto px-4 sm:px-6">
        <div className="text-center mb-14">
          <h2 className="text-3xl font-bold text-gray-900">{t('features.title')}</h2>
          <p className="text-gray-500 mt-3">{t('features.subtitle')}</p>
        </div>
        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-6">
          {FEATURES.map(({ icon: Icon, key }) => (
            <div key={key} className="bg-gray-50 rounded-2xl p-6 hover:shadow-md transition-shadow">
              <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-500 flex items-center justify-center mb-4">
                <Icon size={24} className="text-white" />
              </div>
              <h3 className="text-lg font-semibold text-gray-900 mb-2">{t(`features.${key}.title`)}</h3>
              <p className="text-sm text-gray-500">{t(`features.${key}.desc`)}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 2: Create HowItWorks component**

Create `portal-frontend/src/components/public/HowItWorks.tsx`:

```tsx
import { useTranslation } from 'react-i18next';

const STEPS = ['step1', 'step2', 'step3'] as const;

export function HowItWorks() {
  const { t } = useTranslation();

  return (
    <section className="bg-gray-50 py-20">
      <div className="max-w-5xl mx-auto px-4 sm:px-6">
        <h2 className="text-3xl font-bold text-gray-900 text-center mb-14">{t('howItWorks.title')}</h2>
        <div className="grid md:grid-cols-3 gap-10">
          {STEPS.map((key, i) => (
            <div key={key} className="text-center">
              <div className="w-14 h-14 rounded-full bg-gradient-to-br from-indigo-600 to-purple-600 text-white flex items-center justify-center text-xl font-bold mx-auto mb-4">
                {i + 1}
              </div>
              <h3 className="text-lg font-semibold text-gray-900 mb-2">{t(`howItWorks.${key}.title`)}</h3>
              <p className="text-sm text-gray-500">{t(`howItWorks.${key}.desc`)}</p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Add to LandingPage**

Update `portal-frontend/src/pages/public/LandingPage.tsx`:

```tsx
import { Hero } from '../../components/public/Hero';
import { Stats } from '../../components/public/Stats';
import { FeaturesGrid } from '../../components/public/FeaturesGrid';
import { HowItWorks } from '../../components/public/HowItWorks';

export function LandingPage() {
  return (
    <>
      <Hero />
      <Stats />
      <FeaturesGrid />
      <HowItWorks />
    </>
  );
}
```

- [ ] **Step 4: Verify in browser**

Open `http://localhost:3001/` — scroll down to see Features grid (6 cards) and How It Works (3 steps).

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/components/public/FeaturesGrid.tsx portal-frontend/src/components/public/HowItWorks.tsx portal-frontend/src/pages/public/LandingPage.tsx
git commit -m "feat: add FeaturesGrid and HowItWorks to landing page"
```

---

### Task 7: Build Landing Page — Screenshot + Pricing Preview + Final CTA

**Files:**
- Create: `portal-frontend/src/components/public/Screenshot.tsx`
- Create: `portal-frontend/src/components/public/PricingPreview.tsx`
- Create: `portal-frontend/src/components/public/FinalCta.tsx`
- Modify: `portal-frontend/src/pages/public/LandingPage.tsx`

- [ ] **Step 1: Create Screenshot component**

Create `portal-frontend/src/components/public/Screenshot.tsx`:

```tsx
import { useTranslation } from 'react-i18next';

export function Screenshot() {
  const { t } = useTranslation();

  return (
    <section className="py-20">
      <div className="max-w-5xl mx-auto px-4 sm:px-6 text-center">
        <h2 className="text-3xl font-bold text-gray-900 mb-10">{t('screenshot.title')}</h2>
        <div className="bg-gradient-to-br from-gray-100 to-gray-200 rounded-2xl p-16 text-gray-400 text-lg">
          [ {t('screenshot.alt')} ]
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 2: Create PricingPreview component**

Create `portal-frontend/src/components/public/PricingPreview.tsx`:

```tsx
import { Link, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

const PLANS = ['starter', 'business', 'enterprise'] as const;

export function PricingPreview() {
  const { t } = useTranslation();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <section className="bg-gray-50 py-20">
      <div className="max-w-5xl mx-auto px-4 sm:px-6">
        <h2 className="text-3xl font-bold text-gray-900 text-center mb-12">{t('pricingPreview.title')}</h2>
        <div className="grid md:grid-cols-3 gap-6">
          {PLANS.map((plan) => {
            const isBusiness = plan === 'business';
            return (
              <div
                key={plan}
                className={`relative bg-white rounded-2xl p-6 ${isBusiness ? 'border-2 border-indigo-500 shadow-lg' : 'border border-gray-200'}`}
              >
                {isBusiness && (
                  <div className="absolute -top-3 left-1/2 -translate-x-1/2 bg-gradient-to-r from-indigo-600 to-purple-600 text-white text-xs font-semibold px-3 py-1 rounded-full">
                    {t('pricingPreview.popular')}
                  </div>
                )}
                <h3 className="text-lg font-semibold text-gray-900">{t(`pricing.${plan}.name`)}</h3>
                <p className="text-sm text-gray-500 mt-1">{t(`pricing.${plan}.desc`)}</p>
                <div className="mt-4">
                  <span className="text-3xl font-bold text-gray-900">{t(`pricing.${plan}.price`)}</span>
                  {plan !== 'enterprise' && <span className="text-gray-500 text-sm">{t('pricingPreview.perMonth')}</span>}
                </div>
                <p className="text-sm text-gray-500 mt-2">
                  {t(`pricing.${plan}.smsLimit`)} {t('pricingPreview.smsIncluded')}
                </p>
                <Link
                  to="/register"
                  className={`block text-center mt-6 py-2 rounded-xl text-sm font-semibold transition-colors ${
                    isBusiness
                      ? 'bg-gradient-to-r from-indigo-600 to-purple-600 text-white hover:opacity-90'
                      : 'border border-gray-300 text-gray-700 hover:bg-gray-50'
                  }`}
                >
                  {t('pricingPreview.cta')}
                </Link>
              </div>
            );
          })}
        </div>
        <div className="text-center mt-8">
          <Link to={`${prefix}/pricing`} className="text-indigo-600 hover:text-indigo-700 font-medium">
            {t('pricingPreview.moreDetails')}
          </Link>
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Create FinalCta component**

Create `portal-frontend/src/components/public/FinalCta.tsx`:

```tsx
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';

export function FinalCta() {
  const { t } = useTranslation();

  return (
    <section className="bg-gradient-to-br from-indigo-900 via-indigo-700 to-purple-600 text-white py-20">
      <div className="max-w-3xl mx-auto px-4 text-center">
        <h2 className="text-3xl sm:text-4xl font-bold mb-4">{t('finalCta.title')}</h2>
        <p className="text-indigo-100 mb-8">{t('finalCta.subtitle')}</p>
        <Link to="/register" className="inline-block bg-white text-indigo-700 font-semibold px-8 py-3 rounded-xl hover:bg-indigo-50 transition-colors">
          {t('finalCta.cta')}
        </Link>
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Assemble complete LandingPage**

Update `portal-frontend/src/pages/public/LandingPage.tsx`:

```tsx
import { Hero } from '../../components/public/Hero';
import { Stats } from '../../components/public/Stats';
import { FeaturesGrid } from '../../components/public/FeaturesGrid';
import { HowItWorks } from '../../components/public/HowItWorks';
import { Screenshot } from '../../components/public/Screenshot';
import { PricingPreview } from '../../components/public/PricingPreview';
import { FinalCta } from '../../components/public/FinalCta';

export function LandingPage() {
  return (
    <>
      <Hero />
      <Stats />
      <FeaturesGrid />
      <HowItWorks />
      <Screenshot />
      <PricingPreview />
      <FinalCta />
    </>
  );
}
```

- [ ] **Step 5: Verify in browser**

Open `http://localhost:3001/` — full landing page with all 7 sections should render. Test mobile view (resize to 375px width).

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/components/public/Screenshot.tsx portal-frontend/src/components/public/PricingPreview.tsx portal-frontend/src/components/public/FinalCta.tsx portal-frontend/src/pages/public/LandingPage.tsx
git commit -m "feat: complete landing page with all 7 sections"
```

---

## Phase 3: Pricing Page

### Task 8: Build Pricing Page

**Files:**
- Create: `portal-frontend/src/components/public/PricingCard.tsx`
- Create: `portal-frontend/src/components/public/PricingTable.tsx`
- Create: `portal-frontend/src/components/public/PricingFaq.tsx`
- Create: `portal-frontend/src/components/public/PricingCalculator.tsx`
- Modify: `portal-frontend/src/pages/public/PricingPage.tsx`

- [ ] **Step 1: Create PricingCard component**

Create `portal-frontend/src/components/public/PricingCard.tsx`:

```tsx
import { Link } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { Check } from 'lucide-react';

type Plan = 'starter' | 'business' | 'enterprise';

export function PricingCard({ plan }: { plan: Plan }) {
  const { t } = useTranslation();
  const isBusiness = plan === 'business';
  const features = t(`pricing.${plan}.features`, { returnObjects: true }) as string[];

  return (
    <div className={`relative bg-white rounded-2xl p-8 flex flex-col ${isBusiness ? 'border-2 border-indigo-500 shadow-xl' : 'border border-gray-200'}`}>
      {isBusiness && (
        <div className="absolute -top-3 left-1/2 -translate-x-1/2 bg-gradient-to-r from-indigo-600 to-purple-600 text-white text-xs font-semibold px-4 py-1 rounded-full">
          {t('pricingPreview.popular')}
        </div>
      )}
      <h3 className="text-xl font-bold text-gray-900">{t(`pricing.${plan}.name`)}</h3>
      <p className="text-sm text-gray-500 mt-1">{t(`pricing.${plan}.desc`)}</p>
      <div className="mt-6">
        <span className="text-4xl font-bold text-gray-900">{t(`pricing.${plan}.price`)}</span>
        {plan !== 'enterprise' && <span className="text-gray-500">{t('pricingPreview.perMonth')}</span>}
      </div>
      <div className="text-sm text-gray-500 mt-3 space-y-1">
        <p>{t(`pricing.${plan}.smsLimit`)} {t('pricingPreview.smsIncluded')}</p>
        <p>{t(`pricing.${plan}.subAccounts`)}</p>
        <p>{t(`pricing.${plan}.providerLimit`)}</p>
      </div>
      <ul className="mt-6 space-y-3 flex-1">
        {features.map((f) => (
          <li key={f} className="flex items-start gap-2 text-sm text-gray-600">
            <Check size={16} className="text-indigo-500 mt-0.5 shrink-0" />
            {f}
          </li>
        ))}
      </ul>
      <Link
        to="/register"
        className={`block text-center mt-8 py-3 rounded-xl font-semibold transition-colors ${
          isBusiness
            ? 'bg-gradient-to-r from-indigo-600 to-purple-600 text-white hover:opacity-90'
            : 'border border-gray-300 text-gray-700 hover:bg-gray-50'
        }`}
      >
        {t('pricingPreview.cta')}
      </Link>
    </div>
  );
}
```

- [ ] **Step 2: Create PricingTable component**

Create `portal-frontend/src/components/public/PricingTable.tsx`:

```tsx
import { useTranslation } from 'react-i18next';
import { Check, X } from 'lucide-react';

const ROWS = [
  { label: 'API + Web Panel', starter: true, business: true, enterprise: true },
  { label: 'Email Support', starter: true, business: true, enterprise: true },
  { label: 'Rule-based Routing', starter: false, business: true, enterprise: true },
  { label: 'Cascade Delivery', starter: false, business: true, enterprise: true },
  { label: 'Campaigns & Templates', starter: false, business: true, enterprise: true },
  { label: 'Analytics + Export', starter: false, business: true, enterprise: true },
  { label: 'Priority Support', starter: false, business: true, enterprise: true },
  { label: 'SMPP Access', starter: false, business: false, enterprise: true },
  { label: 'HLR Lookup', starter: false, business: false, enterprise: true },
  { label: 'Dedicated Manager', starter: false, business: false, enterprise: true },
  { label: 'Custom SLA', starter: false, business: false, enterprise: true },
];

export function PricingTable() {
  const { t } = useTranslation();
  const plans = ['starter', 'business', 'enterprise'] as const;

  return (
    <section className="py-16">
      <div className="max-w-4xl mx-auto px-4">
        <h2 className="text-2xl font-bold text-gray-900 text-center mb-8">{t('pricing.compareTitle')}</h2>
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-gray-200">
                <th className="text-left py-3 pr-4 text-gray-500 font-medium"></th>
                {plans.map((p) => (
                  <th key={p} className="text-center py-3 px-4 font-semibold text-gray-900">
                    {t(`pricing.${p}.name`)}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {ROWS.map((row) => (
                <tr key={row.label} className="border-b border-gray-100">
                  <td className="py-3 pr-4 text-gray-600">{row.label}</td>
                  {[row.starter, row.business, row.enterprise].map((val, i) => (
                    <td key={i} className="text-center py-3 px-4">
                      {val ? <Check size={18} className="text-indigo-500 mx-auto" /> : <X size={18} className="text-gray-300 mx-auto" />}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 3: Create PricingFaq component**

Create `portal-frontend/src/components/public/PricingFaq.tsx`:

```tsx
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { ChevronDown } from 'lucide-react';

export function PricingFaq() {
  const { t } = useTranslation();
  const faq = t('pricing.faq', { returnObjects: true }) as { q: string; a: string }[];
  const [open, setOpen] = useState<number | null>(null);

  return (
    <section className="bg-gray-50 py-16">
      <div className="max-w-3xl mx-auto px-4">
        <h2 className="text-2xl font-bold text-gray-900 text-center mb-8">{t('pricing.faqTitle')}</h2>
        <div className="space-y-3">
          {faq.map((item, i) => (
            <div key={i} className="bg-white rounded-xl border border-gray-200">
              <button
                onClick={() => setOpen(open === i ? null : i)}
                className="w-full flex items-center justify-between p-4 text-left"
              >
                <span className="font-medium text-gray-900">{item.q}</span>
                <ChevronDown size={18} className={`text-gray-400 transition-transform ${open === i ? 'rotate-180' : ''}`} />
              </button>
              {open === i && (
                <div className="px-4 pb-4 text-sm text-gray-600">{item.a}</div>
              )}
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
```

- [ ] **Step 4: Create PricingCalculator component**

Create `portal-frontend/src/components/public/PricingCalculator.tsx`:

```tsx
import { useState } from 'react';
import { useTranslation } from 'react-i18next';

export function PricingCalculator() {
  const { t } = useTranslation();
  const [volume, setVolume] = useState(100000);
  const rate = t('pricing.overageRate') as unknown as number;
  const cost = Math.round(volume * rate);

  return (
    <section className="py-16">
      <div className="max-w-xl mx-auto px-4 text-center">
        <h2 className="text-2xl font-bold text-gray-900 mb-8">{t('pricing.calculatorTitle')}</h2>
        <label className="block text-sm text-gray-600 mb-2">{t('pricing.calculatorLabel')}</label>
        <input
          type="range"
          min={10000}
          max={2000000}
          step={10000}
          value={volume}
          onChange={(e) => setVolume(Number(e.target.value))}
          className="w-full accent-indigo-600"
        />
        <div className="text-lg font-semibold text-gray-900 mt-2">{volume.toLocaleString()} SMS</div>
        <div className="mt-4 text-sm text-gray-500">{t('pricing.calculatorResult')}</div>
        <div className="text-3xl font-bold text-indigo-600">${cost.toLocaleString()}</div>
      </div>
    </section>
  );
}
```

- [ ] **Step 5: Assemble PricingPage**

Replace `portal-frontend/src/pages/public/PricingPage.tsx`:

```tsx
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { PricingCard } from '../../components/public/PricingCard';
import { PricingTable } from '../../components/public/PricingTable';
import { PricingFaq } from '../../components/public/PricingFaq';
import { PricingCalculator } from '../../components/public/PricingCalculator';
import { FinalCta } from '../../components/public/FinalCta';

export function PricingPage() {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('seo.pricing.title')}</title>
        <meta name="description" content={t('seo.pricing.desc')} />
      </Helmet>
      <section className="pt-16 pb-12 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-5xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-3">{t('pricing.title')}</h1>
          <p className="text-gray-500">{t('pricing.subtitle')}</p>
        </div>
      </section>
      <section className="py-12">
        <div className="max-w-5xl mx-auto px-4 grid md:grid-cols-3 gap-6">
          <PricingCard plan="starter" />
          <PricingCard plan="business" />
          <PricingCard plan="enterprise" />
        </div>
      </section>
      <PricingTable />
      <PricingCalculator />
      <PricingFaq />
      <FinalCta />
    </>
  );
}
```

- [ ] **Step 6: Verify in browser**

Open `http://localhost:3001/pricing` — should show 3 plan cards, comparison table, calculator slider, FAQ accordion, and CTA.

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/src/components/public/PricingCard.tsx portal-frontend/src/components/public/PricingTable.tsx portal-frontend/src/components/public/PricingFaq.tsx portal-frontend/src/components/public/PricingCalculator.tsx portal-frontend/src/pages/public/PricingPage.tsx
git commit -m "feat: add Pricing page with plans, comparison table, calculator, FAQ"
```

---

## Phase 4: Features Page

### Task 9: Build Features Page

**Files:**
- Modify: `portal-frontend/src/pages/public/FeaturesPage.tsx`

- [ ] **Step 1: Implement FeaturesPage**

Replace `portal-frontend/src/pages/public/FeaturesPage.tsx`:

```tsx
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Plug, Users, GitBranch, Layers, Megaphone, Code, BarChart3, CreditCard, Tag, Search } from 'lucide-react';
import { FinalCta } from '../../components/public/FinalCta';

const ICONS = [Plug, Users, GitBranch, Layers, Megaphone, Code, BarChart3, CreditCard, Tag, Search];

export function FeaturesPage() {
  const { t } = useTranslation();
  const items = t('featuresPage.items', { returnObjects: true }) as { title: string; desc: string }[];

  return (
    <>
      <Helmet>
        <title>{t('seo.features.title')}</title>
        <meta name="description" content={t('seo.features.desc')} />
      </Helmet>

      <section className="pt-16 pb-12 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-4xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-3">{t('featuresPage.title')}</h1>
          <p className="text-gray-500">{t('featuresPage.subtitle')}</p>
        </div>
      </section>

      <section className="py-16">
        <div className="max-w-5xl mx-auto px-4 space-y-20">
          {items.map((item, i) => {
            const Icon = ICONS[i];
            const reversed = i % 2 === 1;
            return (
              <div key={i} className={`flex flex-col md:flex-row items-center gap-10 ${reversed ? 'md:flex-row-reverse' : ''}`}>
                <div className="flex-1">
                  <div className="w-12 h-12 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-500 flex items-center justify-center mb-4">
                    <Icon size={24} className="text-white" />
                  </div>
                  <h3 className="text-2xl font-bold text-gray-900 mb-3">{item.title}</h3>
                  <p className="text-gray-600 leading-relaxed">{item.desc}</p>
                </div>
                <div className="flex-1 w-full">
                  <div className="bg-gradient-to-br from-gray-100 to-gray-200 rounded-2xl p-12 text-gray-400 text-center">
                    [ Screenshot ]
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </section>

      <FinalCta />
    </>
  );
}
```

- [ ] **Step 2: Verify in browser**

Open `http://localhost:3001/features` — 10 feature blocks alternating layout should render.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/public/FeaturesPage.tsx
git commit -m "feat: add Features page with 10 alternating feature blocks"
```

---

## Phase 5: Docs Pages (Markdown)

### Task 10: Build Docs System with Markdown Rendering

**Files:**
- Create: `portal-frontend/src/content/docs/getting-started.md`
- Create: `portal-frontend/src/content/docs/authentication.md`
- Create: `portal-frontend/src/content/docs/send-sms.md`
- Create: `portal-frontend/src/utils/markdown.ts`
- Create: `portal-frontend/src/components/public/DocsSidebar.tsx`
- Modify: `portal-frontend/src/pages/public/DocsPage.tsx`
- Modify: `portal-frontend/src/pages/public/DocArticlePage.tsx`

- [ ] **Step 1: Create markdown utility**

Create `portal-frontend/src/utils/markdown.ts`:

```typescript
import { unified } from 'unified';
import remarkParse from 'remark-parse';
import remarkGfm from 'remark-gfm';
import remarkRehype from 'remark-rehype';
import rehypeStringify from 'rehype-stringify';
import rehypeHighlight from 'rehype-highlight';

export async function renderMarkdown(content: string): Promise<string> {
  const result = await unified()
    .use(remarkParse)
    .use(remarkGfm)
    .use(remarkRehype)
    .use(rehypeHighlight)
    .use(rehypeStringify)
    .process(content);
  return String(result);
}

export function parseFrontmatter(raw: string): { data: Record<string, unknown>; content: string } {
  const match = raw.match(/^---\n([\s\S]*?)\n---\n([\s\S]*)$/);
  if (!match) return { data: {}, content: raw };

  const data: Record<string, unknown> = {};
  for (const line of match[1].split('\n')) {
    const idx = line.indexOf(':');
    if (idx > 0) {
      const key = line.slice(0, idx).trim();
      let val: unknown = line.slice(idx + 1).trim();
      if (typeof val === 'string' && val.startsWith('"') && val.endsWith('"')) val = val.slice(1, -1);
      if (typeof val === 'string' && val.startsWith('[')) {
        try { val = JSON.parse(val); } catch { /* keep string */ }
      }
      data[key] = val;
    }
  }
  return { data, content: match[2] };
}
```

- [ ] **Step 2: Create sample doc files**

Create `portal-frontend/src/content/docs/getting-started.md`:

```markdown
---
title: "Getting Started"
order: 1
---

# Getting Started

Welcome to the SMS Platform API. This guide will help you send your first SMS in minutes.

## Prerequisites

- An active account on SMS Platform
- An API key (generate in Settings → API Keys)

## Quick Start

### 1. Get your API key

After registering, navigate to **Settings → API Keys** and create a new key.

### 2. Send your first SMS

```bash
curl -X POST https://api.smsplatform.com/v1/messages \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "to": "+1234567890",
    "text": "Hello from SMS Platform!",
    "sender": "MyCompany"
  }'
```

### 3. Check delivery status

```bash
curl https://api.smsplatform.com/v1/messages/{message_id} \
  -H "Authorization: Bearer YOUR_API_KEY"
```
```

Create `portal-frontend/src/content/docs/authentication.md`:

```markdown
---
title: "Authentication"
order: 2
---

# Authentication

All API requests require an API key passed in the `Authorization` header.

## API Keys

Generate keys in the portal: **Settings → API Keys**.

```bash
Authorization: Bearer YOUR_API_KEY
```

## Rate Limits

- Starter: 10 requests/second
- Business: 100 requests/second
- Enterprise: Custom
```

Create `portal-frontend/src/content/docs/send-sms.md`:

```markdown
---
title: "Send SMS"
order: 3
---

# Send SMS

## Single message

```bash
POST /v1/messages
Content-Type: application/json

{
  "to": "+1234567890",
  "text": "Your verification code: 1234",
  "sender": "MyApp"
}
```

## Batch send

```bash
POST /v1/messages/batch
Content-Type: application/json

{
  "messages": [
    { "to": "+1234567890", "text": "Hello" },
    { "to": "+0987654321", "text": "World" }
  ],
  "sender": "MyApp"
}
```

## Response

```json
{
  "id": "msg_abc123",
  "status": "queued",
  "created_at": "2026-04-15T10:00:00Z"
}
```
```

- [ ] **Step 3: Create DocsSidebar component**

Create `portal-frontend/src/components/public/DocsSidebar.tsx`:

```tsx
import { Link, useParams, useLocation } from 'react-router-dom';

interface DocEntry {
  slug: string;
  title: string;
}

export function DocsSidebar({ docs }: { docs: DocEntry[] }) {
  const { slug } = useParams();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <nav className="w-full md:w-56 shrink-0">
      <ul className="space-y-1">
        {docs.map((doc) => (
          <li key={doc.slug}>
            <Link
              to={`${prefix}/docs/${doc.slug}`}
              className={`block px-3 py-2 rounded-lg text-sm transition-colors ${
                slug === doc.slug ? 'bg-indigo-50 text-indigo-700 font-medium' : 'text-gray-600 hover:bg-gray-50'
              }`}
            >
              {doc.title}
            </Link>
          </li>
        ))}
      </ul>
    </nav>
  );
}
```

- [ ] **Step 4: Implement DocsPage and DocArticlePage**

Replace `portal-frontend/src/pages/public/DocsPage.tsx`:

```tsx
import { Navigate, useLocation } from 'react-router-dom';

export function DocsPage() {
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';
  return <Navigate to={`${prefix}/docs/getting-started`} replace />;
}
```

Replace `portal-frontend/src/pages/public/DocArticlePage.tsx`:

```tsx
import { useParams } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { renderMarkdown, parseFrontmatter } from '../../utils/markdown';
import { DocsSidebar } from '../../components/public/DocsSidebar';

const docFiles = import.meta.glob('../../content/docs/*.md', { query: '?raw', import: 'default' });

interface DocMeta { slug: string; title: string; order: number; raw: string }

export function DocArticlePage() {
  const { slug } = useParams();
  const { t } = useTranslation();
  const [docs, setDocs] = useState<DocMeta[]>([]);
  const [html, setHtml] = useState('');
  const [title, setTitle] = useState('');

  useEffect(() => {
    async function load() {
      const entries: DocMeta[] = [];
      for (const [path, loader] of Object.entries(docFiles)) {
        const raw = (await loader()) as string;
        const { data } = parseFrontmatter(raw);
        const s = path.split('/').pop()!.replace('.md', '');
        entries.push({ slug: s, title: (data.title as string) || s, order: (data.order as number) || 99, raw });
      }
      entries.sort((a, b) => a.order - b.order);
      setDocs(entries);
    }
    load();
  }, []);

  useEffect(() => {
    const doc = docs.find((d) => d.slug === slug);
    if (!doc) return;
    setTitle(doc.title);
    const { content } = parseFrontmatter(doc.raw);
    renderMarkdown(content).then(setHtml);
  }, [slug, docs]);

  return (
    <>
      <Helmet>
        <title>{title ? `${title} — ${t('seo.docs.title')}` : t('seo.docs.title')}</title>
      </Helmet>
      <div className="max-w-6xl mx-auto px-4 py-12 flex flex-col md:flex-row gap-8">
        <DocsSidebar docs={docs} />
        <article
          className="flex-1 prose prose-indigo max-w-none"
          dangerouslySetInnerHTML={{ __html: html }}
        />
      </div>
    </>
  );
}
```

- [ ] **Step 5: Add highlight.js CSS**

In `portal-frontend/src/index.css`, add at the top (after the tailwindcss import):

```css
@import "highlight.js/styles/github.css";
```

- [ ] **Step 6: Verify in browser**

Open `http://localhost:3001/docs` — should redirect to `/docs/getting-started`. Sidebar with 3 docs, content with syntax-highlighted code blocks.

- [ ] **Step 7: Commit**

```bash
git add portal-frontend/src/content/docs/ portal-frontend/src/utils/markdown.ts portal-frontend/src/components/public/DocsSidebar.tsx portal-frontend/src/pages/public/DocsPage.tsx portal-frontend/src/pages/public/DocArticlePage.tsx portal-frontend/src/index.css
git commit -m "feat: add Docs pages with markdown rendering and syntax highlighting"
```

---

## Phase 6: Blog

### Task 11: Build Blog Pages

**Files:**
- Create: `portal-frontend/src/content/blog/2026-04-15-platform-launch.md`
- Create: `portal-frontend/src/components/public/BlogCard.tsx`
- Modify: `portal-frontend/src/pages/public/BlogPage.tsx`
- Modify: `portal-frontend/src/pages/public/BlogPostPage.tsx`

- [ ] **Step 1: Create sample blog post**

Create `portal-frontend/src/content/blog/2026-04-15-platform-launch.md`:

```markdown
---
title: "Запуск SMS Platform — ваша инфраструктура для SMS-бизнеса"
date: "2026-04-15"
author: "Команда SMS Platform"
tags: ["обновления", "релиз"]
description: "Мы запускаем SMS Platform — PaaS-решение для SMS-реселлеров. Подключайте провайдеров, управляйте клиентами, масштабируйте доставку."
---

# Запуск SMS Platform

Мы рады представить SMS Platform — полнофункциональную PaaS-платформу для SMS-бизнеса.

## Что мы предлагаем

- **Подключение своих провайдеров** — SMPP и HTTP
- **Multi-tenant архитектура** — создавайте sub-accounts для ваших клиентов
- **Гибкая маршрутизация** — правила по стране, оператору, стоимости
- **Полный API** — REST API, Webhooks, SMPP-интерфейс

## Начните бесплатно

Зарегистрируйтесь и получите 14 дней бесплатного доступа ко всем функциям.
```

- [ ] **Step 2: Create BlogCard component**

Create `portal-frontend/src/components/public/BlogCard.tsx`:

```tsx
import { Link, useLocation } from 'react-router-dom';

interface Props {
  slug: string;
  title: string;
  date: string;
  description: string;
  tags: string[];
}

export function BlogCard({ slug, title, date, description, tags }: Props) {
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';

  return (
    <Link to={`${prefix}/blog/${slug}`} className="block bg-white border border-gray-200 rounded-2xl p-6 hover:shadow-md transition-shadow">
      <div className="flex items-center gap-2 mb-3">
        <span className="text-sm text-gray-400">{new Date(date).toLocaleDateString('ru-RU')}</span>
        {tags.map((tag) => (
          <span key={tag} className="text-xs bg-indigo-50 text-indigo-600 px-2 py-0.5 rounded-full">{tag}</span>
        ))}
      </div>
      <h3 className="text-lg font-semibold text-gray-900 mb-2">{title}</h3>
      <p className="text-sm text-gray-500">{description}</p>
    </Link>
  );
}
```

- [ ] **Step 3: Implement BlogPage**

Replace `portal-frontend/src/pages/public/BlogPage.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { parseFrontmatter } from '../../utils/markdown';
import { BlogCard } from '../../components/public/BlogCard';

const blogFiles = import.meta.glob('../../content/blog/*.md', { query: '?raw', import: 'default' });

interface BlogMeta { slug: string; title: string; date: string; description: string; tags: string[]; author: string }

export function BlogPage() {
  const { t } = useTranslation();
  const [posts, setPosts] = useState<BlogMeta[]>([]);

  useEffect(() => {
    async function load() {
      const entries: BlogMeta[] = [];
      for (const [path, loader] of Object.entries(blogFiles)) {
        const raw = (await loader()) as string;
        const { data } = parseFrontmatter(raw);
        const slug = path.split('/').pop()!.replace('.md', '');
        entries.push({
          slug,
          title: (data.title as string) || slug,
          date: (data.date as string) || '',
          description: (data.description as string) || '',
          tags: (data.tags as string[]) || [],
          author: (data.author as string) || '',
        });
      }
      entries.sort((a, b) => b.date.localeCompare(a.date));
      setPosts(entries);
    }
    load();
  }, []);

  return (
    <>
      <Helmet>
        <title>{t('seo.blog.title')}</title>
        <meta name="description" content={t('seo.blog.desc')} />
      </Helmet>
      <section className="pt-16 pb-8 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-4xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900">{t('nav.blog')}</h1>
        </div>
      </section>
      <section className="py-12">
        <div className="max-w-4xl mx-auto px-4 space-y-4">
          {posts.map((post) => (
            <BlogCard key={post.slug} {...post} />
          ))}
        </div>
      </section>
    </>
  );
}
```

- [ ] **Step 4: Implement BlogPostPage**

Replace `portal-frontend/src/pages/public/BlogPostPage.tsx`:

```tsx
import { useParams, Link, useLocation } from 'react-router-dom';
import { useEffect, useState } from 'react';
import { Helmet } from 'react-helmet-async';
import { ArrowLeft } from 'lucide-react';
import { renderMarkdown, parseFrontmatter } from '../../utils/markdown';

const blogFiles = import.meta.glob('../../content/blog/*.md', { query: '?raw', import: 'default' });

export function BlogPostPage() {
  const { slug } = useParams();
  const prefix = useLocation().pathname.startsWith('/en') ? '/en' : '';
  const [html, setHtml] = useState('');
  const [title, setTitle] = useState('');
  const [date, setDate] = useState('');

  useEffect(() => {
    async function load() {
      for (const [path, loader] of Object.entries(blogFiles)) {
        if (path.endsWith(`${slug}.md`)) {
          const raw = (await loader()) as string;
          const { data, content } = parseFrontmatter(raw);
          setTitle((data.title as string) || '');
          setDate((data.date as string) || '');
          const rendered = await renderMarkdown(content);
          setHtml(rendered);
          break;
        }
      }
    }
    load();
  }, [slug]);

  return (
    <>
      <Helmet><title>{title}</title></Helmet>
      <div className="max-w-3xl mx-auto px-4 py-12">
        <Link to={`${prefix}/blog`} className="inline-flex items-center gap-1 text-sm text-indigo-600 hover:text-indigo-700 mb-6">
          <ArrowLeft size={16} /> Back
        </Link>
        {date && <p className="text-sm text-gray-400 mb-2">{new Date(date).toLocaleDateString('ru-RU')}</p>}
        <article className="prose prose-indigo max-w-none" dangerouslySetInnerHTML={{ __html: html }} />
      </div>
    </>
  );
}
```

- [ ] **Step 5: Verify in browser**

Open `http://localhost:3001/blog` — should list the sample post. Click to open full article.

- [ ] **Step 6: Commit**

```bash
git add portal-frontend/src/content/blog/ portal-frontend/src/components/public/BlogCard.tsx portal-frontend/src/pages/public/BlogPage.tsx portal-frontend/src/pages/public/BlogPostPage.tsx
git commit -m "feat: add Blog pages with markdown rendering"
```

---

## Phase 7: About + Contact

### Task 12: Build About Page

**Files:**
- Modify: `portal-frontend/src/pages/public/AboutPage.tsx`

- [ ] **Step 1: Implement AboutPage**

Replace `portal-frontend/src/pages/public/AboutPage.tsx`:

```tsx
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { FinalCta } from '../../components/public/FinalCta';

export function AboutPage() {
  const { t } = useTranslation();

  const stats = [
    { value: '500M+', label: t('stats.sms') },
    { value: '99.9%', label: t('stats.uptime') },
    { value: '50+', label: t('stats.providers') },
    { value: '200+', label: t('stats.resellers') },
  ];

  return (
    <>
      <Helmet>
        <title>{t('seo.about.title')}</title>
        <meta name="description" content={t('seo.about.desc')} />
      </Helmet>
      <section className="pt-16 pb-12 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-3xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-6">{t('about.title')}</h1>
          <p className="text-lg text-gray-600 mb-4">{t('about.mission')}</p>
          <p className="text-lg text-gray-600">{t('about.mission2')}</p>
        </div>
      </section>
      <section className="py-16">
        <div className="max-w-5xl mx-auto px-4">
          <h2 className="text-2xl font-bold text-gray-900 text-center mb-10">{t('about.statsTitle')}</h2>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-8 text-center">
            {stats.map((s) => (
              <div key={s.label}>
                <div className="text-4xl font-bold bg-gradient-to-r from-indigo-600 to-purple-600 bg-clip-text text-transparent">{s.value}</div>
                <div className="text-sm text-gray-500 mt-2">{s.label}</div>
              </div>
            ))}
          </div>
        </div>
      </section>
      <FinalCta />
    </>
  );
}
```

- [ ] **Step 2: Verify and commit**

```bash
git add portal-frontend/src/pages/public/AboutPage.tsx
git commit -m "feat: add About page"
```

---

### Task 13: Build Contact Page

**Files:**
- Create: `portal-frontend/src/components/public/ContactForm.tsx`
- Modify: `portal-frontend/src/pages/public/ContactPage.tsx`

- [ ] **Step 1: Create ContactForm component**

Create `portal-frontend/src/components/public/ContactForm.tsx`:

```tsx
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';

export function ContactForm() {
  const { t } = useTranslation();
  const [status, setStatus] = useState<'idle' | 'sending' | 'success' | 'error'>('idle');

  async function handleSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    setStatus('sending');
    const form = new FormData(e.currentTarget);
    try {
      const res = await fetch('/portal/v1/contact', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          name: form.get('name'),
          email: form.get('email'),
          company: form.get('company'),
          message: form.get('message'),
        }),
      });
      setStatus(res.ok ? 'success' : 'error');
    } catch {
      setStatus('error');
    }
  }

  if (status === 'success') {
    return <p className="text-center text-green-600 py-8">{t('contact.success')}</p>;
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">{t('contact.name')}</label>
        <input name="name" required className="w-full border border-gray-300 rounded-xl px-4 py-2 focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">{t('contact.email')}</label>
        <input name="email" type="email" required className="w-full border border-gray-300 rounded-xl px-4 py-2 focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">{t('contact.company')}</label>
        <input name="company" className="w-full border border-gray-300 rounded-xl px-4 py-2 focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none" />
      </div>
      <div>
        <label className="block text-sm font-medium text-gray-700 mb-1">{t('contact.message')}</label>
        <textarea name="message" rows={5} required className="w-full border border-gray-300 rounded-xl px-4 py-2 focus:ring-2 focus:ring-indigo-500 focus:border-transparent outline-none resize-none" />
      </div>
      {status === 'error' && <p className="text-sm text-red-600">{t('contact.error')}</p>}
      <button
        type="submit"
        disabled={status === 'sending'}
        className="w-full bg-gradient-to-r from-indigo-600 to-purple-600 text-white font-semibold py-3 rounded-xl hover:opacity-90 transition-opacity disabled:opacity-50"
      >
        {status === 'sending' ? t('contact.sending') : t('contact.send')}
      </button>
    </form>
  );
}
```

- [ ] **Step 2: Implement ContactPage**

Replace `portal-frontend/src/pages/public/ContactPage.tsx`:

```tsx
import { Helmet } from 'react-helmet-async';
import { useTranslation } from 'react-i18next';
import { Mail, MessageCircle } from 'lucide-react';
import { ContactForm } from '../../components/public/ContactForm';

export function ContactPage() {
  const { t } = useTranslation();

  return (
    <>
      <Helmet>
        <title>{t('seo.contact.title')}</title>
        <meta name="description" content={t('seo.contact.desc')} />
      </Helmet>
      <section className="pt-16 pb-8 bg-gradient-to-b from-indigo-50 to-white">
        <div className="max-w-3xl mx-auto px-4 text-center">
          <h1 className="text-4xl font-bold text-gray-900 mb-3">{t('contact.title')}</h1>
          <p className="text-gray-500">{t('contact.subtitle')}</p>
        </div>
      </section>
      <section className="py-12">
        <div className="max-w-4xl mx-auto px-4 grid md:grid-cols-2 gap-12">
          <ContactForm />
          <div>
            <h3 className="text-lg font-semibold text-gray-900 mb-4">{t('contact.directTitle')}</h3>
            <div className="space-y-4">
              <a href="mailto:support@smsplatform.com" className="flex items-center gap-3 text-gray-600 hover:text-indigo-600">
                <Mail size={20} /> support@smsplatform.com
              </a>
              <a href="https://t.me/sms_support" target="_blank" rel="noopener noreferrer" className="flex items-center gap-3 text-gray-600 hover:text-indigo-600">
                <MessageCircle size={20} /> Telegram
              </a>
            </div>
          </div>
        </div>
      </section>
    </>
  );
}
```

- [ ] **Step 3: Verify and commit**

```bash
git add portal-frontend/src/components/public/ContactForm.tsx portal-frontend/src/pages/public/ContactPage.tsx
git commit -m "feat: add Contact page with feedback form"
```

---

## Phase 8: SEO (robots.txt, sitemap, prerender)

### Task 14: Add robots.txt and sitemap.xml

**Files:**
- Create: `portal-frontend/public/robots.txt`
- Create: `portal-frontend/public/sitemap.xml`

- [ ] **Step 1: Create robots.txt**

Create `portal-frontend/public/robots.txt`:

```
User-agent: *
Allow: /
Allow: /pricing
Allow: /features
Allow: /docs
Allow: /blog
Allow: /about
Allow: /contact
Allow: /en/

Disallow: /login
Disallow: /register
Disallow: /reset-password
Disallow: /command-center
Disallow: /dashboard
Disallow: /messages
Disallow: /campaigns
Disallow: /quick-send
Disallow: /templates
Disallow: /sender-names
Disallow: /companies
Disallow: /contact-lists
Disallow: /segments
Disallow: /opt-out
Disallow: /analytics
Disallow: /billing
Disallow: /tariffs
Disallow: /api-keys
Disallow: /webhooks
Disallow: /providers
Disallow: /routing
Disallow: /lookup
Disallow: /profile
Disallow: /sub-accounts
Disallow: /audit-log
Disallow: /cascade
Disallow: /settings
Disallow: /admin
Disallow: /notifications

Sitemap: https://smsplatform.com/sitemap.xml
```

- [ ] **Step 2: Create sitemap.xml**

Create `portal-frontend/public/sitemap.xml`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"
        xmlns:xhtml="http://www.w3.org/1999/xhtml">
  <url>
    <loc>https://smsplatform.com/</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/"/>
    <changefreq>weekly</changefreq>
    <priority>1.0</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/pricing</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/pricing"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/pricing"/>
    <changefreq>monthly</changefreq>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/features</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/features"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/features"/>
    <changefreq>monthly</changefreq>
    <priority>0.9</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/docs</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/docs"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/docs"/>
    <changefreq>weekly</changefreq>
    <priority>0.8</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/blog</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/blog"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/blog"/>
    <changefreq>weekly</changefreq>
    <priority>0.7</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/about</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/about"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/about"/>
    <changefreq>monthly</changefreq>
    <priority>0.6</priority>
  </url>
  <url>
    <loc>https://smsplatform.com/contact</loc>
    <xhtml:link rel="alternate" hreflang="en" href="https://smsplatform.com/en/contact"/>
    <xhtml:link rel="alternate" hreflang="ru" href="https://smsplatform.com/contact"/>
    <changefreq>monthly</changefreq>
    <priority>0.6</priority>
  </url>
</urlset>
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/public/robots.txt portal-frontend/public/sitemap.xml
git commit -m "feat: add robots.txt and sitemap.xml for SEO"
```

---

### Task 15: Build Verification

**Files:** None (verification only)

- [ ] **Step 1: Run TypeScript check**

```bash
cd portal-frontend && npx tsc --noEmit
```

Expected: No type errors.

- [ ] **Step 2: Run build**

```bash
cd portal-frontend && npm run build
```

Expected: Build completes successfully.

- [ ] **Step 3: Test all routes in browser**

Start dev server and verify each route loads without errors:
- `/` — Landing page (7 sections)
- `/pricing` — Pricing (cards, table, calculator, FAQ)
- `/features` — Features (10 blocks)
- `/docs` → redirects to `/docs/getting-started`
- `/docs/getting-started` — Doc with sidebar and syntax highlighting
- `/blog` — Blog list
- `/blog/2026-04-15-platform-launch` — Blog post
- `/about` — About page
- `/contact` — Contact form
- `/en` — English landing
- `/en/pricing` — English pricing
- `/login` — Still works (existing auth flow)
- Language switcher toggles between RU and EN

- [ ] **Step 4: Final commit if any fixes needed**

```bash
git add -A && git commit -m "fix: address build/type issues"
```

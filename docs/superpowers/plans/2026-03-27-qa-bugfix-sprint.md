# QA Bugfix Sprint — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 11 bugs (2 critical, 5 major, 4 minor) and 4 UX issues from QA report, organized into 4 parallel blocks.

**Architecture:** 4 independent blocks by layer: (1) Backend security & validation, (2) Frontend auth & UX, (3) Mobile responsive, (4) Services & infra. Blocks do not share files and can be executed in parallel worktrees.

**Tech Stack:** Go 1.24.0 (gorilla/mux, gRPC, pgx/v5, zerolog), React 19 + TypeScript + Vite 6, Tailwind CSS v4, Radix UI

---

## File Structure

```
internal/gateway/client/middleware/
  auth.go                                # MODIFY — BUG-01: real API key auth
internal/gateway/portal/handlers/
  auth.go                                # MODIFY — BUG-02: CSRF token, BUG-04: email validation, BUG-05: length limits
  api_keys.go                            # MODIFY — BUG-05: API key name length limit
  webhooks.go                            # MODIFY — BUG-07: URL validation messages
  messages.go                            # MODIFY — U1: SendMessage handler
internal/gateway/portal/middleware/
  csrf.go                                # MODIFY — BUG-02: enable CSRF validation
internal/gateway/portal/router/
  router.go                              # MODIFY — U1: wire SendMessage route
internal/services/client/grpc/
  server.go                              # MODIFY — BUG-09: ListPlans RPC
internal/services/client/application/
  client_service.go                      # MODIFY — BUG-09: ListPlans service method
portal-frontend/src/
  contexts/AuthContext.tsx                # MODIFY — BUG-03: logout finally
  pages/dashboard/DashboardPage.tsx       # MODIFY — BUG-08: balance display, U3: empty state
  pages/messages/MessagesPage.tsx         # MODIFY — U1: Send SMS button + modal
  pages/profile/ProfilePage.tsx           # MODIFY — U4: refreshUser after save
  pages/providers/ProvidersPage.tsx       # MODIFY — U2: localized empty state
  components/ui/Modal.tsx                 # MODIFY — BUG-10: aria-describedby
  components/layout/Sidebar.tsx           # MODIFY — BUG-06: mobile responsive
  components/layout/UserLayout.tsx        # MODIFY — BUG-06: hamburger + backdrop
  components/data/DataTable.tsx           # MODIFY — BUG-06: responsive columns
```

---

## Блок 1: Backend Security & Validation

### Task 1: BUG-01 — ClientAuthMiddleware (critical)

**Files:**
- Modify: `internal/gateway/client/middleware/auth.go:34-52`

Middleware сейчас возвращает 401 для всех запросов при `LOAD_TEST_MODE != true`. Нужно извлекать API key из заголовка `X-API-Key` и вызывать `authClient.Authenticate()`.

- [ ] **Step 1: Write implementation**

Replace the `ClientAuthMiddleware` function body (lines 34–52) in `internal/gateway/client/middleware/auth.go`:

```go
func ClientAuthMiddleware(authClient authv1.AuthServiceClient) func(http.Handler) http.Handler {
	dummyID := uuid.MustParse("c0000000-0000-0000-0000-000000000001")
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isLoadTestMode() {
				ctx := r.Context()
				ctx = context.WithValue(ctx, UserIDKey, dummyID)
				ctx = context.WithValue(ctx, ClientIDKey, dummyID)
				ctx = context.WithValue(ctx, RoleKey, &authv1.Role{Name: "client"})
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "MISSING_API_KEY",
					Message:    "API key is required",
				})
				return
			}

			resp, err := authClient.Authenticate(r.Context(), &authv1.AuthenticateRequest{
				ApiKey: apiKey,
			})
			if err != nil {
				respondError(w, &shared.AppError{
					HTTPStatus: http.StatusUnauthorized,
					Code:       "INVALID_API_KEY",
					Message:    "Invalid or expired API key",
				})
				return
			}

			ctx := r.Context()
			if resp.User != nil {
				userID, parseErr := uuid.Parse(resp.User.Id)
				if parseErr == nil {
					ctx = context.WithValue(ctx, UserIDKey, userID)
					// GetClientID() falls back to UserIDKey if ClientIDKey is not set
				}
				ctx = context.WithValue(ctx, UserKey, resp.User)
				if resp.User.Role != nil {
					ctx = context.WithValue(ctx, RoleKey, resp.User.Role)
				}
			}

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

- [ ] **Step 2: Build and verify**

```bash
cd /home/magomed/projects/sms && go build ./internal/gateway/client/...
```

Expected: successful compilation.

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/client/middleware/auth.go
git commit -m "fix(auth): implement real API key authentication in ClientAuthMiddleware (BUG-01)"
```

---

### Task 2: BUG-02 — CSRF token = Session ID (critical/security)

**Files:**
- Modify: `internal/gateway/portal/handlers/auth.go:287-313` — generate separate CSRF token
- Modify: `internal/gateway/portal/middleware/csrf.go` — enable CSRF validation

- [ ] **Step 1: Add crypto/rand import and generate CSRF token**

In `internal/gateway/portal/handlers/auth.go`, add `"crypto/rand"`, `"encoding/hex"` to imports. Then replace the `setSessionCookies` function (lines 287–313):

```go
// setSessionCookies устанавливает cookies для сессии портала
func setSessionCookies(w http.ResponseWriter, sessionID string) {
	secure := isSecureCookie()
	sameSite := http.SameSiteStrictMode
	if !secure {
		sameSite = http.SameSiteLaxMode
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "portal_session",
		Value:    sessionID,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   secure,
		SameSite: sameSite,
	})

	// Generate a separate random CSRF token (not linked to session ID)
	csrfBytes := make([]byte, 32)
	if _, err := rand.Read(csrfBytes); err != nil {
		// Fallback: still don't use sessionID
		csrfBytes = make([]byte, 32)
	}
	csrfToken := hex.EncodeToString(csrfBytes)

	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    csrfToken,
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: false,
		Secure:   secure,
		SameSite: sameSite,
	})
}
```

- [ ] **Step 2: Enable CSRF validation middleware**

Replace the content of `internal/gateway/portal/middleware/csrf.go`:

```go
package middleware

import (
	"net/http"
	"strings"
)

// CSRFMiddleware создает middleware для защиты от CSRF-атак.
// Проверяет X-CSRF-Token header против значения csrf_token cookie.
func CSRFMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Safe methods не требуют CSRF проверки
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// Проверяем CSRF token
			cookie, err := r.Cookie("csrf_token")
			if err != nil || cookie.Value == "" {
				respondCSRFError(w)
				return
			}

			headerToken := r.Header.Get("X-CSRF-Token")
			if headerToken == "" || !strings.EqualFold(headerToken, cookie.Value) {
				respondCSRFError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// respondCSRFError отправляет ошибку CSRF-валидации
func respondCSRFError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte(`{"error":{"code":"CSRF_VALIDATION_FAILED","message":"Ошибка валидации CSRF-токена"}}`))
}
```

- [ ] **Step 3: Build and verify**

```bash
go build ./internal/gateway/portal/...
```

Expected: successful compilation.

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/auth.go internal/gateway/portal/middleware/csrf.go
git commit -m "fix(security): generate separate CSRF token and enable validation (BUG-02)"
```

---

### Task 3: BUG-04 — Email validation на backend

**Files:**
- Modify: `internal/gateway/portal/handlers/auth.go:247-250`

- [ ] **Step 1: Add `"net/mail"` import and email validation**

In `internal/gateway/portal/handlers/auth.go`, add `"net/mail"` to imports. In the `Register` function, after the check `if req.Email == ""` (line 247–250), add:

```go
	if req.Email == "" {
		respondError(w, shared.ErrInvalidInput("Поле email обязательно"))
		return
	}
	if _, err := mail.ParseAddress(req.Email); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат email"))
		return
	}
```

- [ ] **Step 2: Build and verify**

```bash
go build ./internal/gateway/portal/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/gateway/portal/handlers/auth.go
git commit -m "fix(validation): add email format validation on Register (BUG-04)"
```

---

### Task 4: BUG-05 — Long strings → INTERNAL_ERROR

**Files:**
- Modify: `internal/gateway/portal/handlers/auth.go:255-258` — CompanyName length
- Modify: `internal/gateway/portal/handlers/api_keys.go:95-98` — API key name length

- [ ] **Step 1: Add length validation to Register handler**

In `internal/gateway/portal/handlers/auth.go`, in the `Register` function, after the `CompanyName == ""` check (line 255–258), add:

```go
	if req.CompanyName == "" {
		respondError(w, shared.ErrInvalidInput("Поле company_name обязательно"))
		return
	}
	if len(req.CompanyName) > 500 {
		respondError(w, shared.ErrInvalidInput("Название компании не может превышать 500 символов"))
		return
	}
```

- [ ] **Step 2: Add length validation to CreateAPIKey handler**

In `internal/gateway/portal/handlers/api_keys.go`, in the `CreateAPIKey` function, after the `req.Name == ""` check (line 95–98), add:

```go
	if req.Name == "" {
		respondError(w, shared.ErrInvalidInput("Поле name обязательно"))
		return
	}
	if len(req.Name) > 255 {
		respondError(w, shared.ErrInvalidInput("Имя не может превышать 255 символов"))
		return
	}
```

- [ ] **Step 3: Build and verify**

```bash
go build ./internal/gateway/portal/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/auth.go internal/gateway/portal/handlers/api_keys.go
git commit -m "fix(validation): add string length limits for CompanyName and API key Name (BUG-05)"
```

---

### Task 5: BUG-07 — Webhook URL validation message

**Files:**
- Modify: `internal/gateway/portal/handlers/webhooks.go:75-82`

- [ ] **Step 1: Add URL validation to CreateWebhook handler**

In `internal/gateway/portal/handlers/webhooks.go`, add `"net/url"` to imports. Then replace the `req.URL == ""` check (lines 75–78):

```go
	if req.URL == "" {
		respondError(w, shared.ErrInvalidInput("url обязателен"))
		return
	}
	parsedURL, parseErr := url.Parse(req.URL)
	if parseErr != nil || parsedURL.Host == "" {
		respondError(w, shared.ErrInvalidInput("Неверный формат URL"))
		return
	}
	if parsedURL.Scheme != "https" {
		respondError(w, shared.ErrInvalidInput("URL должен использовать HTTPS"))
		return
	}
```

- [ ] **Step 2: Add same validation to UpdateWebhook handler**

Find the `UpdateWebhook` handler in the same file. If it accepts an optional URL field, add the same validation when URL is non-empty:

```go
	if req.URL != "" {
		parsedURL, parseErr := url.Parse(req.URL)
		if parseErr != nil || parsedURL.Host == "" {
			respondError(w, shared.ErrInvalidInput("Неверный формат URL"))
			return
		}
		if parsedURL.Scheme != "https" {
			respondError(w, shared.ErrInvalidInput("URL должен использовать HTTPS"))
			return
		}
	}
```

- [ ] **Step 3: Build and verify**

```bash
go build ./internal/gateway/portal/...
```

- [ ] **Step 4: Commit**

```bash
git add internal/gateway/portal/handlers/webhooks.go
git commit -m "fix(validation): add handler-level URL format and HTTPS validation for webhooks (BUG-07)"
```

---

## Блок 2: Frontend Auth & UX

### Task 6: BUG-03 — Logout не очищает UI

**Files:**
- Modify: `portal-frontend/src/contexts/AuthContext.tsx:60-67`

Сейчас `setUser(null)` на строке 66 — после catch. Если catch пробрасывает ошибку (`throw e`), `setUser(null)` не вызовется. Нужно перенести в `finally`.

- [ ] **Step 1: Move setUser(null) to finally**

Replace the `logout` callback (lines 60–67) in `portal-frontend/src/contexts/AuthContext.tsx`:

```typescript
  const logout = useCallback(async () => {
    try {
      await authApi.logout();
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 401)) throw e;
    } finally {
      setUser(null);
    }
  }, []);
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/contexts/AuthContext.tsx
git commit -m "fix(auth): move setUser(null) to finally in logout to always clear UI (BUG-03)"
```

---

### Task 7: BUG-08 — Balance пустой на Dashboard

**Files:**
- Modify: `portal-frontend/src/pages/dashboard/DashboardPage.tsx:50-51`

Когда `data.balance` равен `"0"` или `null`, нужно корректно отображать. Текущий код: `` `${data.balance} ${data.currency}` `` — если balance `null`, покажет "null RUB".

- [ ] **Step 1: Add safe balance display**

In `portal-frontend/src/pages/dashboard/DashboardPage.tsx`, replace line 51:

```typescript
    { label: 'Balance', value: `${data.balance ?? '0.00'} ${data.currency ?? ''}`.trim() },
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/dashboard/DashboardPage.tsx
git commit -m "fix(ui): handle null/zero balance display on Dashboard (BUG-08)"
```

---

### Task 8: BUG-10 — Missing aria-description on Modal

**Files:**
- Modify: `portal-frontend/src/components/ui/Modal.tsx`

Radix UI Dialog по умолчанию требует `Dialog.Description` для полной a11y. Добавим опциональный `description` prop.

- [ ] **Step 1: Add description prop**

Replace the full content of `portal-frontend/src/components/ui/Modal.tsx`:

```tsx
import { type ReactNode } from 'react';
import * as Dialog from '@radix-ui/react-dialog';

interface ModalProps {
  open: boolean;
  onClose: () => void;
  title: string;
  description?: string;
  children: ReactNode;
  wide?: boolean;
}

export function Modal({ open, onClose, title, description, children, wide }: ModalProps) {
  return (
    <Dialog.Root open={open} onOpenChange={(o) => !o && onClose()}>
      <Dialog.Portal>
        <Dialog.Overlay className="fixed inset-0 bg-black/40 z-40" />
        <Dialog.Content
          aria-describedby={description ? 'modal-desc' : undefined}
          className={`fixed top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2
            bg-white rounded-lg shadow-xl z-50 p-6 max-h-[85vh] overflow-y-auto
            ${wide ? 'w-[700px]' : 'w-[480px]'}`}
        >
          <Dialog.Title className="text-lg font-semibold mb-4">{title}</Dialog.Title>
          {description && (
            <Dialog.Description id="modal-desc" className="sr-only">
              {description}
            </Dialog.Description>
          )}
          {children}
          <Dialog.Close asChild>
            <button
              className="absolute top-4 right-4 text-gray-400 hover:text-gray-600"
              aria-label="Close"
            >
              ✕
            </button>
          </Dialog.Close>
        </Dialog.Content>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/ui/Modal.tsx
git commit -m "fix(a11y): add optional description prop to Modal for aria-describedby (BUG-10)"
```

---

### Task 9: U4 — Profile не обновляется в sidebar

**Files:**
- Modify: `portal-frontend/src/pages/profile/ProfilePage.tsx:1-2,30-45`

Сейчас ProfilePage использует только `profileApi` для получения и сохранения профиля, но не обновляет глобальный `AuthContext`. После сохранения email/имя в sidebar не обновляются.

- [ ] **Step 1: Import useAuth and call refreshUser**

In `portal-frontend/src/pages/profile/ProfilePage.tsx`, add `useAuth` import on line 2:

```typescript
import { profileApi, ApiError, type ProfileData } from '../../api/client';
import { useAuth } from '../../contexts/AuthContext';
```

Add `const { refreshUser } = useAuth();` inside the component (after line 8):

```typescript
export function ProfilePage() {
  const { refreshUser } = useAuth();
  const [profile, setProfile] = useState<ProfileData | null>(null);
```

In `handleSaveProfile`, after `setProfile(updated)` (line 37), add `await refreshUser()`:

```typescript
      const updated = await profileApi.update({ contact_person: contactPerson, phone });
      setProfile(updated);
      await refreshUser();
      setSaveMsg('Profile updated');
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/profile/ProfilePage.tsx
git commit -m "fix(ux): refresh sidebar user info after profile save (U4)"
```

---

## Блок 3: Mobile Responsive

### Task 10: BUG-06 — Sidebar responsive + mobile drawer

**Files:**
- Modify: `portal-frontend/src/components/layout/Sidebar.tsx`

Сейчас sidebar имеет фиксированную ширину `w-56` и всегда видим. На мобильных (<768px) нужен slide-in drawer с backdrop.

- [ ] **Step 1: Add mobile props and responsive classes**

Replace the full content of `portal-frontend/src/components/layout/Sidebar.tsx`:

```tsx
import { type ReactNode } from 'react';
import { Link, useLocation } from 'react-router-dom';

export interface NavItem {
  path: string;
  label: string;
  icon?: ReactNode;
}

interface SidebarProps {
  title: string;
  items: NavItem[];
  footer?: ReactNode;
  isOpen?: boolean;
  onClose?: () => void;
}

export function Sidebar({ title, items, footer, isOpen, onClose }: SidebarProps) {
  const location = useLocation();

  return (
    <>
      {/* Mobile backdrop */}
      {isOpen && (
        <div
          className="fixed inset-0 bg-black/40 z-40 md:hidden"
          onClick={onClose}
          aria-hidden="true"
        />
      )}

      <aside
        aria-label="Navigation sidebar"
        className={`
          w-56 border-r border-gray-200 bg-gray-50 flex flex-col min-h-screen
          fixed z-50 top-0 left-0 transition-transform duration-200 ease-in-out
          md:static md:translate-x-0 md:z-auto
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <div className="p-4 border-b border-gray-200 flex items-center justify-between">
          <h2 className="text-base font-semibold text-gray-900">{title}</h2>
          {onClose && (
            <button
              className="md:hidden text-gray-500 hover:text-gray-700"
              onClick={onClose}
              aria-label="Close menu"
            >
              ✕
            </button>
          )}
        </div>
        <nav aria-label="Main menu" className="flex-1 py-2 space-y-0.5 px-2">
          <ul>
          {items.map((item) => {
            const isActive = location.pathname.startsWith(item.path);
            return (
              <li key={item.path}>
                <Link
                  to={item.path}
                  aria-current={isActive ? 'page' : undefined}
                  onClick={onClose}
                  className={`flex items-center gap-2 px-3 py-2 rounded text-sm transition-colors
                    ${isActive
                      ? 'bg-primary/10 text-primary font-medium'
                      : 'text-gray-700 hover:bg-gray-100'
                    }`}
                >
                  {item.icon}
                  {item.label}
                </Link>
              </li>
            );
          })}
          </ul>
        </nav>
        {footer && (
          <div className="p-4 border-t border-gray-200">
            {footer}
          </div>
        )}
      </aside>
    </>
  );
}
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/Sidebar.tsx
git commit -m "fix(mobile): make Sidebar responsive with slide-in drawer on mobile (BUG-06)"
```

---

### Task 11: BUG-06 — UserLayout hamburger menu

**Files:**
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add mobile menu state and hamburger button**

Replace the full content of `portal-frontend/src/components/layout/UserLayout.tsx`:

```tsx
import { useState } from 'react';
import { Outlet } from 'react-router-dom';
import { Sidebar, type NavItem } from './Sidebar';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';

const USER_NAV: NavItem[] = [
  { path: '/dashboard', label: 'Dashboard' },
  { path: '/messages', label: 'Messages' },
  { path: '/providers', label: 'Providers' },
  { path: '/api-keys', label: 'API Keys' },
  { path: '/webhooks', label: 'Webhooks' },
  { path: '/analytics', label: 'Analytics' },
  { path: '/sub-accounts', label: 'Sub-accounts' },
  { path: '/profile', label: 'Profile' },
  { path: '/audit-log', label: 'Audit Log' },
];

export function UserLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />

      {/* Mobile header with hamburger */}
      <div className="fixed top-0 left-0 right-0 h-14 bg-white border-b border-gray-200 flex items-center px-4 z-30 md:hidden">
        <button
          onClick={() => setIsMobileMenuOpen(true)}
          className="text-gray-600 hover:text-gray-900"
          aria-label="Open menu"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="ml-3 font-semibold text-gray-900">SMS Portal</span>
      </div>

      <Sidebar
        title="SMS Portal"
        items={USER_NAV}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div>
            <div className="text-sm text-gray-600 truncate mb-2">{user?.email}</div>
            <button
              onClick={logout}
              className="text-sm text-gray-500 hover:text-gray-700"
            >
              Logout
            </button>
          </div>
        }
      />
      <main id="main-content" className="flex-1 p-4 md:p-6 bg-gray-50/50 overflow-auto pt-18 md:pt-6">
        <Outlet />
      </main>
    </div>
  );
}
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "fix(mobile): add hamburger menu and mobile header to UserLayout (BUG-06)"
```

---

### Task 12: BUG-06 — DataTable responsive columns

**Files:**
- Modify: `portal-frontend/src/components/data/DataTable.tsx:4-9`

Добавим `responsive` flag на Column, который скрывает колонку на маленьких экранах.

- [ ] **Step 1: Add responsive flag to Column and apply hidden classes**

In `portal-frontend/src/components/data/DataTable.tsx`, add `responsive` to the `Column` interface:

```typescript
export interface Column<T> {
  key: string;
  header: string;
  render?: (item: T) => ReactNode;
  sortable?: boolean;
  responsive?: boolean; // hidden on small screens
}
```

In the `<th>` element (line 51), add the responsive class:

```tsx
                <th
                  key={col.key}
                  className={`px-4 py-3 text-left font-medium text-gray-700
                    ${col.sortable ? 'cursor-pointer hover:text-gray-900 select-none' : ''}
                    ${col.responsive ? 'hidden sm:table-cell' : ''}`}
                  onClick={() => col.sortable && onSort?.(col.key)}
                >
```

In the `<td>` element (line 81), add the responsive class:

```tsx
                    <td key={col.key} className={`px-4 py-3 text-gray-800 ${col.responsive ? 'hidden sm:table-cell' : ''}`}>
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/data/DataTable.tsx
git commit -m "fix(mobile): add responsive column hiding to DataTable (BUG-06)"
```

---

### Task 13: BUG-06 — Dashboard/Analytics grid breakpoints

**Files:**
- Modify: `portal-frontend/src/pages/dashboard/DashboardPage.tsx:77`

- [ ] **Step 1: Update grid classes**

In `portal-frontend/src/pages/dashboard/DashboardPage.tsx`, change the grid classes on line 77 from:

```tsx
<div className="grid grid-cols-2 md:grid-cols-3 gap-4">
```

to:

```tsx
<div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
```

This gives 1 column on phones, 2 on small tablets, 3 on desktops.

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/dashboard/DashboardPage.tsx
git commit -m "fix(mobile): improve Dashboard grid breakpoints for small screens (BUG-06)"
```

---

## Блок 4: Services & Infra

### Task 14: BUG-09 — ListPlans не реализован

**Files:**
- Modify: `internal/services/client/application/client_service.go`
- Modify: `internal/services/client/grpc/server.go`

Proto `rpc ListPlans` уже определён. Репозиторий `PlanRepository.ListActive()` уже существует. Нужно:
1. Добавить `ListPlans` метод в `ClientService`
2. Реализовать `ListPlans` RPC в gRPC server

- [ ] **Step 1: Add PlanRepository interface and ListPlans to ClientService**

In `internal/services/client/application/client_service.go`, add a plan repository interface and method.

After existing repository interfaces (around line 39), add:

```go
type PlanRepositoryInterface interface {
	ListActive(ctx context.Context) ([]*domain.Plan, error)
}
```

Add `planRepo` field to `ClientService` struct:

```go
type ClientService struct {
	clientRepo ClientRepositoryInterface
	configRepo ConfigRepositoryInterface
	planRepo   PlanRepositoryInterface
}
```

Update the constructor to accept `planRepo`:

```go
func NewClientService(clientRepo ClientRepositoryInterface, configRepo ConfigRepositoryInterface, planRepo PlanRepositoryInterface) *ClientService {
	return &ClientService{
		clientRepo: clientRepo,
		configRepo: configRepo,
		planRepo:   planRepo,
	}
}
```

Add `ListPlans` method:

```go
func (s *ClientService) ListPlans(ctx context.Context) ([]*domain.Plan, error) {
	return s.planRepo.ListActive(ctx)
}
```

- [ ] **Step 2: Verify constructor callers**

Find all callers of `NewClientService` and add the `planRepo` argument:

```bash
grep -rn "NewClientService" --include="*.go" /home/magomed/projects/sms
```

Update each call site to pass a `PlanRepository` instance (or `nil` if unavailable). Typically this is in `cmd/` or `main.go` of the client-service.

- [ ] **Step 3: Implement ListPlans RPC in gRPC server**

In `internal/services/client/grpc/server.go`, after the `ToggleSandbox` method (after line 521), add:

```go
// ListPlans возвращает список активных подписочных планов
func (s *Server) ListPlans(ctx context.Context, req *clientv1.ListPlansRequest) (*clientv1.ListPlansResponse, error) {
	plans, err := s.clientService.ListPlans(ctx)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения списка планов")
		return nil, status.Error(codes.Internal, "failed to list plans")
	}

	protoPlans := make([]*clientv1.SubscriptionPlan, 0, len(plans))
	for _, p := range plans {
		protoPlans = append(protoPlans, &clientv1.SubscriptionPlan{
			Id:                 p.ID.String(),
			Name:               p.Name,
			DisplayName:        p.DisplayName,
			MonthlyPriceRub:    p.MonthlyPriceRub,
			MaxSmsPerMonth:     int32(p.MaxSMSPerMonth),
			MaxSmppConnections: int32(p.MaxSMPPConnections),
			MaxUsers:           int32(p.MaxUsers),
			RateLimits: &clientv1.RateLimits{
				PerSecond: int32(p.RateLimitPerSecond),
				PerMinute: int32(p.RateLimitPerMinute),
				PerHour:   int32(p.RateLimitPerHour),
				PerDay:    int32(p.RateLimitPerDay),
			},
			Features: map[string]bool{
				"analytics":    p.Features.Analytics,
				"webhooks":     p.Features.Webhooks,
				"hlr":          p.Features.HLR,
				"smart_routing": p.Features.SmartRouting,
				"sub_accounts": p.Features.SubAccounts,
				"white_label":  p.Features.WhiteLabel,
			},
			Active: p.Active,
		})
	}

	return &clientv1.ListPlansResponse{
		Plans: protoPlans,
	}, nil
}
```

- [ ] **Step 4: Build and verify**

```bash
go build ./internal/services/client/...
```

- [ ] **Step 5: Commit**

```bash
git add internal/services/client/application/client_service.go internal/services/client/grpc/server.go
git commit -m "feat: implement ListPlans gRPC RPC (BUG-09)"
```

---

### Task 15: BUG-11 — Deploy не пересобирает образы

**Files:**
- Modify: `scripts/server.sh:73-85`

**Статус: ALREADY FIXED.** Проверка кода показала, что `cmd_deploy()` (строки 73–85) уже содержит `--build` флаг:
- Строка 78: `remote_compose "up -d --build $service"`
- Строка 81: `remote_compose "up -d --build"`

Никаких изменений не требуется. Если баг воспроизводится — причина в другом (кеш Docker, `do_sync` не подтягивает изменения, etc.).

- [ ] **Step 1: Verify the fix is already in place**

```bash
grep -n "up -d" /home/magomed/projects/sms/scripts/server.sh
```

Expected: все вызовы `docker compose up -d` уже содержат `--build`.

- [ ] **Step 2: Skip — no changes needed**

Пометить BUG-11 как неприменимый (код уже корректный).

---

### Task 16: U1 — Кнопка Send Message

**Files:**
- Modify: `internal/gateway/portal/handlers/messages.go` — add `SendMessage` handler
- Modify: `internal/gateway/portal/router/router.go:100` — wire the route
- Modify: `portal-frontend/src/api/client.ts` — add `send` method to `messagesApi`
- Modify: `portal-frontend/src/pages/messages/MessagesPage.tsx` — add Send SMS button + modal

- [ ] **Step 1: Implement SendMessage backend handler**

In `internal/gateway/portal/handlers/messages.go`, add `"encoding/json"` to imports and add the handler after `ListMessages`:

```go
// sendMessageRequest представляет запрос на отправку SMS из портала
type sendMessageRequest struct {
	Destination string `json:"destination"`
	Text        string `json:"text"`
	Source      string `json:"source"`
}

// SendMessage обрабатывает POST /messages — отправка одного SMS
func (h *MessageHandlers) SendMessage(w http.ResponseWriter, r *http.Request) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return
	}

	var req sendMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Destination == "" {
		respondError(w, shared.ErrInvalidInput("Поле destination обязательно"))
		return
	}
	if req.Text == "" {
		respondError(w, shared.ErrInvalidInput("Поле text обязательно"))
		return
	}

	if h.messagingClient == nil {
		respondError(w, shared.ErrServiceUnavailable("Сервис отправки сообщений недоступен"))
		return
	}

	resp, err := h.messagingClient.SendMessage(r.Context(), &messagingv1.SendMessageRequest{
		ClientId:    clientID.String(),
		Source:      req.Source,
		Destination: req.Destination,
		Text:        req.Text,
	})
	if err != nil {
		log.Error().Err(err).Msg("ошибка отправки SMS")
		respondGRPCError(w, err)
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"message_id": resp.MessageId,
		"status":     resp.Status,
	})
}
```

- [ ] **Step 2: Wire the route**

In `internal/gateway/portal/router/router.go`, replace line 100 (`notImplemented`):

```go
	messages.HandleFunc("", messageHandlers.SendMessage).Methods("POST")
```

- [ ] **Step 3: Build backend**

```bash
go build ./internal/gateway/portal/...
```

- [ ] **Step 4: Add frontend API method**

In `portal-frontend/src/api/client.ts`, add `send` to `messagesApi`:

```typescript
export const messagesApi = {
  list: (params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<unknown>(`/messages?${qs}`);
  },
  send: (data: { destination: string; text: string; source?: string }) =>
    apiFetch<{ message_id: string; status: string }>('/messages', {
      method: 'POST',
      body: JSON.stringify(data),
    }),
};
```

- [ ] **Step 5: Add Send SMS button and modal to MessagesPage**

In `portal-frontend/src/pages/messages/MessagesPage.tsx`, add the imports:

```typescript
import { useEffect, useState, useCallback } from 'react';
import { messagesApi } from '../../api/client';
import { PageHeader } from '../../components/layout/PageHeader';
import { FilterBar, type FilterDef } from '../../components/data/FilterBar';
import { DataTable, type Column } from '../../components/data/DataTable';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
```

Inside `MessagesPage`, add state and handlers before the `return`:

```typescript
  const [showSendModal, setShowSendModal] = useState(false);
  const [sendDest, setSendDest] = useState('');
  const [sendText, setSendText] = useState('');
  const [sendSource, setSendSource] = useState('');
  const [sending, setSending] = useState(false);
  const [sendError, setSendError] = useState('');

  const handleSendSMS = async () => {
    setSending(true);
    setSendError('');
    try {
      await messagesApi.send({ destination: sendDest, text: sendText, source: sendSource || undefined });
      setShowSendModal(false);
      setSendDest('');
      setSendText('');
      setSendSource('');
      fetchMessages();
    } catch (err) {
      setSendError(err instanceof Error ? err.message : 'Ошибка отправки');
    } finally {
      setSending(false);
    }
  };
```

Update the return statement — replace `<PageHeader title="Messages" />` with:

```tsx
      <PageHeader
        title="Messages"
        actions={
          <Button onClick={() => setShowSendModal(true)}>Отправить SMS</Button>
        }
      />

      <Modal
        open={showSendModal}
        onClose={() => setShowSendModal(false)}
        title="Отправить SMS"
        description="Отправка тестового SMS сообщения"
      >
        <div className="space-y-3">
          {sendError && <p role="alert" className="text-red-600 text-sm">{sendError}</p>}
          <Input
            label="Номер получателя"
            value={sendDest}
            onChange={(e) => setSendDest(e.target.value)}
            placeholder="+79001234567"
            required
          />
          <Input
            label="Sender ID"
            value={sendSource}
            onChange={(e) => setSendSource(e.target.value)}
            placeholder="MyCompany"
          />
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Текст сообщения</label>
            <textarea
              className="w-full rounded border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
              rows={3}
              value={sendText}
              onChange={(e) => setSendText(e.target.value)}
              required
            />
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setShowSendModal(false)}>Отмена</Button>
            <Button onClick={handleSendSMS} disabled={sending || !sendDest || !sendText}>
              {sending ? 'Отправка...' : 'Отправить'}
            </Button>
          </div>
        </div>
      </Modal>
```

- [ ] **Step 6: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 7: Commit**

```bash
git add internal/gateway/portal/handlers/messages.go internal/gateway/portal/router/router.go portal-frontend/src/api/client.ts portal-frontend/src/pages/messages/MessagesPage.tsx
git commit -m "feat: add Send SMS button and modal to Messages page (U1)"
```

---

### Task 17: U2 — Providers empty state (локализация)

**Files:**
- Modify: `portal-frontend/src/pages/providers/ProvidersPage.tsx`

Empty state уже существует, но текст на английском ("No providers yet."). Нужно локализовать и улучшить описание.

- [ ] **Step 1: Update empty state text**

In `portal-frontend/src/pages/providers/ProvidersPage.tsx`, find the empty state block (around line 72) and replace:

```tsx
{!loading && !error && providers.length === 0 && (
  <div className="text-center py-12 text-gray-500">
    <p className="mb-2">Провайдеры не настроены</p>
    <p className="text-sm mb-4">Подключите SMPP-провайдера для начала отправки SMS</p>
    <Button variant="ghost" onClick={() => navigate('/providers/new')}>
      Добавить провайдера
    </Button>
  </div>
)}
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/providers/ProvidersPage.tsx
git commit -m "fix(ux): improve and localize Providers empty state (U2)"
```

---

### Task 18: U3 — Dashboard empty state

**Files:**
- Modify: `portal-frontend/src/pages/dashboard/DashboardPage.tsx`

Если все метрики = 0 и нет сообщений, показать guidance-блок.

- [ ] **Step 1: Add empty state guidance**

In `portal-frontend/src/pages/dashboard/DashboardPage.tsx`, add the guidance block after `<PageHeader title="Dashboard" />` (line 76) and before the cards grid:

```tsx
      <PageHeader title="Dashboard" />

      {data.messages_today === 0 && data.active_api_keys === 0 && data.active_webhooks === 0 && (
        <div className="border border-primary/30 bg-primary/5 rounded-lg p-5 mb-6">
          <h3 className="font-semibold text-gray-900 mb-3">Начните работу с платформой</h3>
          <ol className="list-decimal list-inside space-y-1 text-sm text-gray-700">
            <li>
              <a href="/api-keys" className="text-primary hover:underline">Создайте API ключ</a> для доступа к API
            </li>
            <li>
              <a href="/providers" className="text-primary hover:underline">Подключите SMPP-провайдера</a> для отправки SMS
            </li>
            <li>
              <a href="/messages" className="text-primary hover:underline">Отправьте тестовое SMS</a> и отслеживайте статистику
            </li>
          </ol>
        </div>
      )}

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
```

- [ ] **Step 2: Verify build**

```bash
cd /home/magomed/projects/sms/portal-frontend && npx tsc --noEmit
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/dashboard/DashboardPage.tsx
git commit -m "fix(ux): add onboarding guidance block to Dashboard empty state (U3)"
```

# Aggregator Cabinet Redesign — Phase 1 & 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give aggregators a dedicated workspace with mode switching and unified moderation of sub-account sender names, templates, and operator registrations.

**Architecture:** A `ModeSwitcher` component lets resellers toggle between "own account" and "network management" modes (persisted in localStorage). Network mode renders a `NetworkLayout` with its own sidebar under `/network/*` routes. Moderation backend uses SQL ownership checks + delegation to existing admin gRPC methods — no proto changes needed.

**Tech Stack:** TypeScript 5.7, React 19, React Router 7.1, Tailwind CSS 4.2, Go 1.24.0, gorilla/mux, pgx/v5, gRPC (sendernamev1, templatev1)

---

## Task 1: ModeSwitcher component

**Files:**
- Create: `portal-frontend/src/components/layout/ModeSwitcher.tsx`

- [ ] **Step 1: Create ModeSwitcher component**

```tsx
// portal-frontend/src/components/layout/ModeSwitcher.tsx
import { useNavigate, useLocation } from 'react-router-dom';

const LS_KEY = 'reseller_mode';

export type ResellerMode = 'own' | 'network';

export function getResellerMode(): ResellerMode {
  return (localStorage.getItem(LS_KEY) as ResellerMode) || 'own';
}

export function setResellerMode(mode: ResellerMode) {
  localStorage.setItem(LS_KEY, mode);
}

interface ModeSwitcherProps {
  currentMode: ResellerMode;
}

export function ModeSwitcher({ currentMode }: ModeSwitcherProps) {
  const navigate = useNavigate();
  const location = useLocation();

  function toggle() {
    if (currentMode === 'own') {
      setResellerMode('network');
      navigate('/network/sub-accounts');
    } else {
      setResellerMode('own');
      navigate('/command-center');
    }
  }

  const isNetwork = currentMode === 'network';

  return (
    <button
      onClick={toggle}
      className="flex items-center gap-2 w-full px-3 py-2.5 rounded-lg text-sm font-medium transition-colors border border-gray-200 hover:bg-gray-100"
      title={isNetwork ? 'Перейти к своему аккаунту' : 'Управление сетью'}
    >
      {isNetwork ? (
        <>
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
          </svg>
          <span>Свой аккаунт</span>
          <span className="ml-auto text-gray-400">&larr;</span>
        </>
      ) : (
        <>
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span>Управление сетью</span>
          <span className="ml-auto text-gray-400">&rarr;</span>
        </>
      )}
    </button>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/layout/ModeSwitcher.tsx
git commit -m "feat(portal): add ModeSwitcher component with localStorage persistence"
```

---

## Task 2: NetworkLayout component

**Files:**
- Create: `portal-frontend/src/components/layout/NetworkLayout.tsx`

- [ ] **Step 1: Create NetworkLayout**

```tsx
// portal-frontend/src/components/layout/NetworkLayout.tsx
import { useState, useEffect } from 'react';
import { Outlet, useLocation } from 'react-router-dom';
import { Sidebar, type NavItem, type NavGroup } from './Sidebar';
import { ModeSwitcher, setResellerMode } from './ModeSwitcher';
import { SkipLink } from '../SkipLink';
import { useAuth } from '../../contexts/AuthContext';
import { NotificationBell } from '../ui/NotificationBell';
import { apiFetch } from '../../api/client';

const NAV_ITEMS: NavItem[] = [
  { path: '/network/sub-accounts', label: 'Суб-аккаунты' },
];

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Управление',
    items: [
      { path: '/network/moderation', label: 'Модерация' },
    ],
  },
];

export function NetworkLayout() {
  const { user, logout } = useAuth();
  const [isMobileMenuOpen, setIsMobileMenuOpen] = useState(false);
  const [moderationCount, setModerationCount] = useState(0);

  useEffect(() => {
    setResellerMode('network');
  }, []);

  useEffect(() => {
    apiFetch<{ sender_names: number; templates: number; registrations: number }>('/reseller/moderation/counts')
      .then((data) => setModerationCount(data.sender_names + data.templates + data.registrations))
      .catch(() => {});
  }, []);

  // Build nav items with badge for moderation
  const navItemsWithBadge: NavItem[] = [...NAV_ITEMS];
  const navGroupsWithBadge: NavGroup[] = NAV_GROUPS.map((group) => ({
    ...group,
    items: group.items.map((item) => {
      if (item.path === '/network/moderation' && moderationCount > 0) {
        return { ...item, label: `Модерация (${moderationCount})` };
      }
      return item;
    }),
  }));

  return (
    <div className="flex min-h-screen">
      <SkipLink targetId="main-content" />

      {/* Mobile header */}
      <div className="fixed top-0 left-0 right-0 h-14 bg-white border-b border-gray-200 flex items-center px-4 z-30 md:hidden">
        <button
          onClick={() => setIsMobileMenuOpen(true)}
          className="text-gray-600 hover:text-gray-900"
          aria-label="Открыть меню"
        >
          <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
          </svg>
        </button>
        <span className="ml-3 font-semibold text-gray-900">Управление сетью</span>
        <div className="ml-auto">
          <NotificationBell />
        </div>
      </div>

      <Sidebar
        title="Управление сетью"
        items={navItemsWithBadge}
        groups={navGroupsWithBadge}
        isOpen={isMobileMenuOpen}
        onClose={() => setIsMobileMenuOpen(false)}
        footer={
          <div className="space-y-3">
            <ModeSwitcher currentMode="network" />
            <div>
              <div className="text-sm text-gray-600 truncate">{user?.email}</div>
              <button onClick={logout} className="text-sm text-gray-500 hover:text-gray-700">
                Выйти
              </button>
            </div>
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

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/components/layout/NetworkLayout.tsx
git commit -m "feat(portal): add NetworkLayout with network sidebar and moderation badge"
```

---

## Task 3: Add ModeSwitcher to UserLayout

**Files:**
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`

- [ ] **Step 1: Add import and ModeSwitcher to UserLayout**

In `portal-frontend/src/components/layout/UserLayout.tsx`, add import at top:

```tsx
import { ModeSwitcher } from './ModeSwitcher';
```

Then replace the footer `<div>` inside `<Sidebar>` to include ModeSwitcher for resellers. Replace this block:

```tsx
        footer={
          <div>
            <div className="flex items-center justify-between mb-2">
```

With:

```tsx
        footer={
          <div>
            {!!user?.is_reseller && (
              <div className="mb-3">
                <ModeSwitcher currentMode="own" />
              </div>
            )}
            <div className="flex items-center justify-between mb-2">
```

- [ ] **Step 2: Remove sub-accounts from Settings group**

In `portal-frontend/src/components/layout/UserLayout.tsx`, in `buildNavGroups`, remove the reseller conditional from the Settings items. Replace:

```tsx
        ...(isReseller ? [{ path: '/sub-accounts', label: 'Суб-аккаунты' }] : []),
```

With nothing (delete the line). The `isReseller` parameter to `buildNavGroups` is no longer used — remove it:

Change the function signature from:

```tsx
function buildNavGroups(isReseller: boolean): NavGroup[] {
```

To:

```tsx
function buildNavGroups(): NavGroup[] {
```

And the call from:

```tsx
  const navGroups = buildNavGroups(!!user?.is_reseller);
```

To:

```tsx
  const navGroups = buildNavGroups();
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/UserLayout.tsx
git commit -m "feat(portal): add network mode switcher to UserLayout, remove sub-accounts from settings"
```

---

## Task 4: Add /network/* routes and redirects to App.tsx

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Add lazy imports for NetworkLayout**

At the top of `portal-frontend/src/App.tsx`, after existing imports, add:

```tsx
import { NetworkLayout } from './components/layout/NetworkLayout';
```

- [ ] **Step 2: Add network routes inside RequireAuth**

In `portal-frontend/src/App.tsx`, inside `<Route element={<RequireAuth />}>`, after the `cascade/history/:id` route, add the network routes and redirects:

```tsx
        {/* Network mode (reseller) */}
        <Route path="/network" element={<RequireReseller><NetworkLayout /></RequireReseller>}>
          <Route index element={<Navigate to="/network/sub-accounts" replace />} />
          <Route path="sub-accounts" element={<SubAccountsListPage />} />
          <Route path="sub-accounts/:id" element={<SubAccountDetailPage />} />
          <Route path="moderation" element={<div>Moderation placeholder</div>} />
        </Route>

        {/* Backward compat redirects */}
        <Route path="/sub-accounts" element={<Navigate to="/network/sub-accounts" replace />} />
        <Route path="/sub-accounts/:id" element={<Navigate to="/network/sub-accounts/:id" replace />} />
```

Also remove the old `/sub-accounts` routes (the two lines with `RequireReseller`):

```tsx
        <Route path="/sub-accounts" element={<RequireReseller><SubAccountsListPage /></RequireReseller>} />
        <Route path="/sub-accounts/:id" element={<RequireReseller><SubAccountDetailPage /></RequireReseller>} />
```

- [ ] **Step 3: Fix sub-account redirect with param**

The `/sub-accounts/:id` redirect can't forward params with plain `<Navigate>`. Create a small redirect component at the top of App.tsx:

```tsx
function SubAccountRedirect() {
  const { id } = useParams();
  return <Navigate to={`/network/sub-accounts/${id}`} replace />;
}
```

Add `useParams` to the react-router-dom import. Then replace the redirect route:

```tsx
        <Route path="/sub-accounts/:id" element={<SubAccountRedirect />} />
```

- [ ] **Step 4: Fix internal links in SubAccountsListPage**

In `portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx`, update the Link path. Replace:

```tsx
      <Link to={`/sub-accounts/${sa.id}`} className="text-primary hover:underline">
```

With:

```tsx
      <Link to={`/network/sub-accounts/${sa.id}`} className="text-primary hover:underline">
```

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/App.tsx portal-frontend/src/pages/sub-accounts/SubAccountsListPage.tsx
git commit -m "feat(portal): add /network/* routes, move sub-accounts under network layout"
```

---

## Task 5: Backend — Reseller sender names handlers

**Files:**
- Create: `internal/gateway/portal/handlers/reseller_sender_names.go`

- [ ] **Step 1: Create reseller sender names handler**

```go
// internal/gateway/portal/handlers/reseller_sender_names.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	sendernamev1 "github.com/smpp-server/smpp-server/api/proto/sendernamev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerSenderNameHandlers struct {
	pool             *pgxpool.Pool
	senderNameClient sendernamev1.SenderNameServiceClient
}

func NewResellerSenderNameHandlers(pool *pgxpool.Pool, senderNameClient sendernamev1.SenderNameServiceClient) *ResellerSenderNameHandlers {
	return &ResellerSenderNameHandlers{pool: pool, senderNameClient: senderNameClient}
}

func (h *ResellerSenderNameHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return "", false
	}
	return clientID.String(), true
}

// ListResellerSenderNames GET /portal/v1/reseller/sender-names
func (h *ResellerSenderNameHandlers) ListResellerSenderNames(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	query := `SELECT sn.id, sn.client_id, c.email AS sub_account_email,
	                 sn.name, sn.status, sn.rejection_reason, sn.created_at
	          FROM sender_names sn
	          JOIN clients c ON c.id = sn.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}

	if statusFilter != "" {
		query += " AND sn.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY sn.created_at DESC LIMIT 50"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения sender names субаккаунтов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type senderNameJSON struct {
		ID              string     `json:"id"`
		ClientID        string     `json:"client_id"`
		SubAccountEmail string     `json:"sub_account_email"`
		Name            string     `json:"name"`
		Status          string     `json:"status"`
		RejectionReason *string    `json:"rejection_reason"`
		CreatedAt       time.Time  `json:"created_at"`
	}
	items := make([]senderNameJSON, 0)
	for rows.Next() {
		var sn senderNameJSON
		if err := rows.Scan(&sn.ID, &sn.ClientID, &sn.SubAccountEmail,
			&sn.Name, &sn.Status, &sn.RejectionReason, &sn.CreatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, sn)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"sender_names": items, "total": len(items)})
}

// ApproveResellerSenderName POST /portal/v1/reseller/sender-names/{id}/approve
func (h *ResellerSenderNameHandlers) ApproveResellerSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT sn.status FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE sn.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if currentStatus != "pending" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса pending"))
		return
	}

	resp, err := h.senderNameClient.ApproveSenderName(r.Context(), &sendernamev1.ApproveSenderNameRequest{
		Id:      id,
		ActorId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerSenderNameToJSON(resp.SenderName))
}

// RejectResellerSenderName POST /portal/v1/reseller/sender-names/{id}/reject
func (h *ResellerSenderNameHandlers) RejectResellerSenderName(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Reason == "" {
		respondError(w, shared.ErrInvalidInput("reason обязателен"))
		return
	}

	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT sn.status FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE sn.id = $1 AND c.parent_client_id = $2`,
		id, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("имя отправителя не найдено"))
		return
	}
	if currentStatus != "pending" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса pending"))
		return
	}

	resp, err := h.senderNameClient.RejectSenderName(r.Context(), &sendernamev1.RejectSenderNameRequest{
		Id:      id,
		ActorId: userID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerSenderNameToJSON(resp.SenderName))
}

func resellerSenderNameToJSON(sn *sendernamev1.SenderNameInfo) map[string]interface{} {
	m := map[string]interface{}{
		"id":               sn.Id,
		"client_id":        sn.ClientId,
		"name":             sn.Name,
		"status":           sn.Status,
		"rejection_reason": sn.RejectionReason,
	}
	if sn.CreatedAt != nil {
		m["created_at"] = sn.CreatedAt.AsTime()
	}
	if sn.UpdatedAt != nil {
		m["updated_at"] = sn.UpdatedAt.AsTime()
	}
	return m
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_sender_names.go
git commit -m "feat(portal): add reseller sender name moderation handlers"
```

---

## Task 6: Backend — Reseller templates handlers

**Files:**
- Create: `internal/gateway/portal/handlers/reseller_templates.go`

- [ ] **Step 1: Create reseller templates handler**

```go
// internal/gateway/portal/handlers/reseller_templates.go
package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"

	templatev1 "github.com/smpp-server/smpp-server/api/proto/templatev1"
	"github.com/smpp-server/smpp-server/internal/gateway/portal/middleware"
	"github.com/smpp-server/smpp-server/internal/shared"
)

type ResellerTemplateHandlers struct {
	pool           *pgxpool.Pool
	templateClient templatev1.TemplateServiceClient
}

func NewResellerTemplateHandlers(pool *pgxpool.Pool, templateClient templatev1.TemplateServiceClient) *ResellerTemplateHandlers {
	return &ResellerTemplateHandlers{pool: pool, templateClient: templateClient}
}

func (h *ResellerTemplateHandlers) checkReseller(w http.ResponseWriter, r *http.Request) (string, bool) {
	clientID, ok := middleware.GetClientID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("Клиент не найден"))
		return "", false
	}
	var isReseller bool
	if err := h.pool.QueryRow(r.Context(),
		`SELECT is_reseller FROM clients WHERE id = $1`, clientID,
	).Scan(&isReseller); err != nil || !isReseller {
		respondError(w, shared.ErrUnauthorized("доступ только для агрегаторов"))
		return "", false
	}
	return clientID.String(), true
}

func (h *ResellerTemplateHandlers) checkOwnershipAndStatus(w http.ResponseWriter, r *http.Request, clientID, templateID string) (string, bool) {
	var currentStatus string
	err := h.pool.QueryRow(r.Context(),
		`SELECT t.status FROM templates t
		 JOIN clients c ON c.id = t.client_id
		 WHERE t.id = $1 AND c.parent_client_id = $2`,
		templateID, clientID,
	).Scan(&currentStatus)
	if err != nil {
		respondError(w, shared.ErrNotFound("шаблон не найден"))
		return "", false
	}
	return currentStatus, true
}

// ListResellerTemplates GET /portal/v1/reseller/templates
func (h *ResellerTemplateHandlers) ListResellerTemplates(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	statusFilter := r.URL.Query().Get("status")
	query := `SELECT t.id, t.client_id, c.email AS sub_account_email,
	                 t.name, LEFT(t.body, 80) AS body_preview, t.status,
	                 t.rejection_reason, t.created_at
	          FROM templates t
	          JOIN clients c ON c.id = t.client_id
	          WHERE c.parent_client_id = $1`
	args := []interface{}{clientID}

	if statusFilter != "" {
		query += " AND t.status = $2"
		args = append(args, statusFilter)
	}
	query += " ORDER BY t.created_at DESC LIMIT 50"

	rows, err := h.pool.Query(r.Context(), query, args...)
	if err != nil {
		log.Error().Err(err).Msg("ошибка получения шаблонов субаккаунтов")
		respondError(w, shared.ErrInternalServer("ошибка получения данных"))
		return
	}
	defer rows.Close()

	type templateJSON struct {
		ID              string    `json:"id"`
		ClientID        string    `json:"client_id"`
		SubAccountEmail string    `json:"sub_account_email"`
		Name            string    `json:"name"`
		BodyPreview     string    `json:"body_preview"`
		Status          string    `json:"status"`
		RejectionReason *string   `json:"rejection_reason"`
		CreatedAt       time.Time `json:"created_at"`
	}
	items := make([]templateJSON, 0)
	for rows.Next() {
		var t templateJSON
		if err := rows.Scan(&t.ID, &t.ClientID, &t.SubAccountEmail,
			&t.Name, &t.BodyPreview, &t.Status, &t.RejectionReason, &t.CreatedAt); err != nil {
			respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
			return
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		respondError(w, shared.ErrInternalServer("ошибка чтения данных"))
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"templates": items, "total": len(items)})
}

// ApproveResellerTemplate POST /portal/v1/reseller/templates/{id}/approve
func (h *ResellerTemplateHandlers) ApproveResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("approve возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.ApproveTemplate(r.Context(), &templatev1.ApproveTemplateRequest{
		Id:      id,
		ActorId: userID.String(),
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

// RejectResellerTemplate POST /portal/v1/reseller/templates/{id}/reject
func (h *ResellerTemplateHandlers) RejectResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var req struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Reason == "" {
		respondError(w, shared.ErrInvalidInput("reason обязателен"))
		return
	}

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("reject возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.RejectTemplate(r.Context(), &templatev1.RejectTemplateRequest{
		Id:      id,
		ActorId: userID.String(),
		Reason:  req.Reason,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

// RequestRevisionResellerTemplate POST /portal/v1/reseller/templates/{id}/request-revision
func (h *ResellerTemplateHandlers) RequestRevisionResellerTemplate(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}
	userID, ok := middleware.GetUserID(r.Context())
	if !ok {
		respondError(w, shared.ErrUnauthorized("пользователь не найден"))
		return
	}
	id := mux.Vars(r)["id"]

	var req struct {
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, shared.ErrInvalidInput("Неверный формат запроса"))
		return
	}
	if req.Comment == "" {
		respondError(w, shared.ErrInvalidInput("comment обязателен"))
		return
	}

	status, ok := h.checkOwnershipAndStatus(w, r, clientID, id)
	if !ok {
		return
	}
	if status != "pending" && status != "revision_requested" {
		respondError(w, shared.ErrInvalidInput("request-revision возможен только из статуса pending или revision_requested"))
		return
	}

	resp, err := h.templateClient.RequestRevision(r.Context(), &templatev1.RequestRevisionRequest{
		TemplateId: id,
		ReviewerId: userID.String(),
		Comment:    req.Comment,
	})
	if err != nil {
		respondGRPCError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, resellerTemplateToJSON(resp.Template))
}

func resellerTemplateToJSON(t *templatev1.TemplateInfo) map[string]interface{} {
	if t == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":               t.Id,
		"client_id":        t.ClientId,
		"name":             t.Name,
		"body":             t.Body,
		"status":           t.Status,
		"rejection_reason": t.RejectionReason,
	}
	if t.CreatedAt != nil {
		m["created_at"] = t.CreatedAt.AsTime()
	}
	if t.UpdatedAt != nil {
		m["updated_at"] = t.UpdatedAt.AsTime()
	}
	return m
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_templates.go
git commit -m "feat(portal): add reseller template moderation handlers"
```

---

## Task 7: Backend — Moderation counts endpoint

**Files:**
- Modify: `internal/gateway/portal/handlers/reseller_moderation.go`

- [ ] **Step 1: Add GetModerationCounts handler**

At the end of `internal/gateway/portal/handlers/reseller_moderation.go`, before the `resellerNullStr` function, add:

```go
// GetModerationCounts GET /portal/v1/reseller/moderation/counts
func (h *ResellerModerationHandlers) GetModerationCounts(w http.ResponseWriter, r *http.Request) {
	clientID, ok := h.checkReseller(w, r)
	if !ok {
		return
	}

	var snCount, tplCount, regCount int
	err := h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM sender_names sn
		 JOIN clients c ON c.id = sn.client_id
		 WHERE c.parent_client_id = $1 AND sn.status = 'pending'`,
		clientID,
	).Scan(&snCount)
	if err != nil {
		snCount = 0
	}

	err = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM templates t
		 JOIN clients c ON c.id = t.client_id
		 WHERE c.parent_client_id = $1 AND (t.status = 'pending' OR t.status = 'revision_requested')`,
		clientID,
	).Scan(&tplCount)
	if err != nil {
		tplCount = 0
	}

	err = h.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM operator_registrations or2
		 JOIN sender_names sn ON sn.id = or2.sender_name_id
		 JOIN clients c ON c.id = sn.client_id
		 WHERE c.parent_client_id = $1 AND or2.status = 'submitted'`,
		clientID,
	).Scan(&regCount)
	if err != nil {
		regCount = 0
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"sender_names":  snCount,
		"templates":     tplCount,
		"registrations": regCount,
	})
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/gateway/portal/handlers/reseller_moderation.go
git commit -m "feat(portal): add moderation counts endpoint for reseller dashboard badge"
```

---

## Task 8: Backend — Register new handlers and routes

**Files:**
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Add new handler params to SetupRouter**

In `internal/gateway/portal/router/router.go`, add two new parameters to `SetupRouter` function, right after `resellerHandlers *handlers.ResellerModerationHandlers,`:

```go
	resellerSenderNameHandlers *handlers.ResellerSenderNameHandlers,
	resellerTemplateHandlers *handlers.ResellerTemplateHandlers,
```

- [ ] **Step 2: Register new routes**

In the same file, right after the existing reseller `request-revision` route (line ~393), add:

```go
	// Moderation counts
	reseller.HandleFunc("/moderation/counts", resellerHandlers.GetModerationCounts).Methods("GET")

	// Reseller sender names moderation
	resellerSN := reseller.PathPrefix("/sender-names").Subrouter()
	resellerSN.HandleFunc("", resellerSenderNameHandlers.ListResellerSenderNames).Methods("GET")
	resellerSN.HandleFunc("/{id}/approve", resellerSenderNameHandlers.ApproveResellerSenderName).Methods("POST")
	resellerSN.HandleFunc("/{id}/reject", resellerSenderNameHandlers.RejectResellerSenderName).Methods("POST")

	// Reseller templates moderation
	resellerTpl := reseller.PathPrefix("/templates").Subrouter()
	resellerTpl.HandleFunc("", resellerTemplateHandlers.ListResellerTemplates).Methods("GET")
	resellerTpl.HandleFunc("/{id}/approve", resellerTemplateHandlers.ApproveResellerTemplate).Methods("POST")
	resellerTpl.HandleFunc("/{id}/reject", resellerTemplateHandlers.RejectResellerTemplate).Methods("POST")
	resellerTpl.HandleFunc("/{id}/request-revision", resellerTemplateHandlers.RequestRevisionResellerTemplate).Methods("POST")
```

- [ ] **Step 3: Initialize handlers in main.go**

In `cmd/portal-gateway/main.go`, after line `resellerHandlers := handlers.NewResellerModerationHandlers(dbPool)` (line 283), add:

```go
	resellerSenderNameHandlers := handlers.NewResellerSenderNameHandlers(dbPool, serviceClients.SenderNameClient)
	resellerTemplateHandlers := handlers.NewResellerTemplateHandlers(dbPool, serviceClients.TemplateClient)
```

Then in the `SetupRouter` call, add the two new handlers right after `resellerHandlers,` (line 319):

```go
		resellerSenderNameHandlers,
		resellerTemplateHandlers,
```

- [ ] **Step 4: Verify build**

Run:
```bash
cd /c/projects/sms && go build ./cmd/portal-gateway/
```
Expected: Compiles without errors.

- [ ] **Step 5: Commit**

```bash
git add internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(portal): register reseller moderation routes and handlers"
```

---

## Task 9: Frontend — Reseller API methods

**Files:**
- Modify: `portal-frontend/src/api/client.ts`

- [ ] **Step 1: Add reseller API object**

At the end of `portal-frontend/src/api/client.ts`, before the last export, add:

```ts
// Reseller moderation API
export interface ResellerSenderName {
  id: string;
  client_id: string;
  sub_account_email: string;
  name: string;
  status: string;
  rejection_reason: string | null;
  created_at: string;
}

export interface ResellerTemplate {
  id: string;
  client_id: string;
  sub_account_email: string;
  name: string;
  body_preview: string;
  status: string;
  rejection_reason: string | null;
  created_at: string;
}

export interface ModerationCounts {
  sender_names: number;
  templates: number;
  registrations: number;
}

export const resellerApi = {
  getModerationCounts: () =>
    apiFetch<ModerationCounts>('/reseller/moderation/counts'),

  // Sender names
  listSenderNames: (params?: { status?: string }) => {
    const qs = params?.status ? `?status=${params.status}` : '';
    return apiFetch<{ sender_names: ResellerSenderName[]; total: number }>(`/reseller/sender-names${qs}`);
  },
  approveSenderName: (id: string) =>
    apiFetch<unknown>(`/reseller/sender-names/${id}/approve`, { method: 'POST' }),
  rejectSenderName: (id: string, reason: string) =>
    apiFetch<unknown>(`/reseller/sender-names/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),

  // Templates
  listTemplates: (params?: { status?: string }) => {
    const qs = params?.status ? `?status=${params.status}` : '';
    return apiFetch<{ templates: ResellerTemplate[]; total: number }>(`/reseller/templates${qs}`);
  },
  approveTemplate: (id: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/approve`, { method: 'POST' }),
  rejectTemplate: (id: string, reason: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ reason }),
    }),
  requestRevisionTemplate: (id: string, comment: string) =>
    apiFetch<unknown>(`/reseller/templates/${id}/request-revision`, {
      method: 'POST',
      body: JSON.stringify({ comment }),
    }),

  // Operator registrations (existing endpoints, typed access)
  listOperatorRegistrations: (params?: { status?: string; sub_account_id?: string }) => {
    const qs = new URLSearchParams();
    if (params?.status) qs.set('status', params.status);
    if (params?.sub_account_id) qs.set('sub_account_id', params.sub_account_id);
    const q = qs.toString();
    return apiFetch<{ registrations: unknown[] }>(`/reseller/operator-registrations${q ? `?${q}` : ''}`);
  },
  approveOperatorRegistration: (id: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/approve`, { method: 'POST' }),
  rejectOperatorRegistration: (id: string, note?: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/reject`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),
  requestRevisionOperatorRegistration: (id: string, note?: string) =>
    apiFetch<unknown>(`/reseller/operator-registrations/${id}/request-revision`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),
};
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/api/client.ts
git commit -m "feat(portal): add resellerApi with moderation endpoints"
```

---

## Task 10: Frontend — ModerationPage with 3 tabs

**Files:**
- Create: `portal-frontend/src/pages/network/ModerationPage.tsx`

- [ ] **Step 1: Create ModerationPage**

```tsx
// portal-frontend/src/pages/network/ModerationPage.tsx
import { useState, useEffect, type ReactNode } from 'react';
import { PageHeader } from '../../components/layout/PageHeader';
import { DataTable, type Column } from '../../components/data/DataTable';
import { StatusBadge } from '../../components/ui/Badge';
import { Button } from '../../components/ui/Button';
import { Modal } from '../../components/ui/Modal';
import { Input } from '../../components/ui/Input';
import { useToast } from '../../components/ui/Toast';
import {
  resellerApi,
  ApiError,
  type ResellerSenderName,
  type ResellerTemplate,
  type ModerationCounts,
} from '../../api/client';

type Tab = 'sender_names' | 'templates' | 'registrations';

function TabButton({
  active,
  onClick,
  children,
  count,
}: {
  active: boolean;
  onClick: () => void;
  children: ReactNode;
  count?: number;
}) {
  return (
    <button
      onClick={onClick}
      className={`px-4 py-2 text-sm font-medium rounded-t-lg border-b-2 transition-colors ${
        active
          ? 'border-primary text-primary bg-white'
          : 'border-transparent text-gray-500 hover:text-gray-700 hover:border-gray-300'
      }`}
    >
      {children}
      {count != null && count > 0 && (
        <span className="ml-1.5 inline-flex items-center justify-center px-1.5 py-0.5 text-xs font-bold rounded-full bg-red-100 text-red-700">
          {count}
        </span>
      )}
    </button>
  );
}

export function ModerationPage() {
  const toast = useToast();
  const [tab, setTab] = useState<Tab>('sender_names');
  const [counts, setCounts] = useState<ModerationCounts>({ sender_names: 0, templates: 0, registrations: 0 });

  // Sender names state
  const [senderNames, setSenderNames] = useState<ResellerSenderName[]>([]);
  const [snLoading, setSnLoading] = useState(false);
  const [snStatusFilter, setSnStatusFilter] = useState('');

  // Templates state
  const [templates, setTemplates] = useState<ResellerTemplate[]>([]);
  const [tplLoading, setTplLoading] = useState(false);
  const [tplStatusFilter, setTplStatusFilter] = useState('');

  // Operator registrations state
  const [registrations, setRegistrations] = useState<unknown[]>([]);
  const [regLoading, setRegLoading] = useState(false);
  const [regStatusFilter, setRegStatusFilter] = useState('');

  // Modal state
  const [modal, setModal] = useState<{
    type: 'reject_sn' | 'reject_tpl' | 'revision_tpl' | 'reject_reg' | 'revision_reg';
    id: string;
  } | null>(null);
  const [modalText, setModalText] = useState('');
  const [modalSubmitting, setModalSubmitting] = useState(false);

  useEffect(() => {
    resellerApi.getModerationCounts().then(setCounts).catch(() => {});
  }, []);

  // Load sender names
  useEffect(() => {
    if (tab !== 'sender_names') return;
    setSnLoading(true);
    resellerApi
      .listSenderNames(snStatusFilter ? { status: snStatusFilter } : undefined)
      .then((r) => setSenderNames(r.sender_names))
      .catch(() => toast.error('Не удалось загрузить имена'))
      .finally(() => setSnLoading(false));
  }, [tab, snStatusFilter]);

  // Load templates
  useEffect(() => {
    if (tab !== 'templates') return;
    setTplLoading(true);
    resellerApi
      .listTemplates(tplStatusFilter ? { status: tplStatusFilter } : undefined)
      .then((r) => setTemplates(r.templates))
      .catch(() => toast.error('Не удалось загрузить шаблоны'))
      .finally(() => setTplLoading(false));
  }, [tab, tplStatusFilter]);

  // Load registrations
  useEffect(() => {
    if (tab !== 'registrations') return;
    setRegLoading(true);
    resellerApi
      .listOperatorRegistrations(regStatusFilter ? { status: regStatusFilter } : undefined)
      .then((r) => setRegistrations(r.registrations))
      .catch(() => toast.error('Не удалось загрузить регистрации'))
      .finally(() => setRegLoading(false));
  }, [tab, regStatusFilter]);

  function refreshTab() {
    if (tab === 'sender_names') setSnStatusFilter((s) => s); // force re-fetch via useEffect
    if (tab === 'templates') setTplStatusFilter((s) => s);
    if (tab === 'registrations') setRegStatusFilter((s) => s);
    resellerApi.getModerationCounts().then(setCounts).catch(() => {});
  }

  async function handleApproveSN(id: string) {
    try {
      await resellerApi.approveSenderName(id);
      toast.success('Имя одобрено');
      setSenderNames((prev) => prev.filter((sn) => sn.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleApproveTpl(id: string) {
    try {
      await resellerApi.approveTemplate(id);
      toast.success('Шаблон одобрен');
      setTemplates((prev) => prev.filter((t) => t.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleApproveReg(id: string) {
    try {
      await resellerApi.approveOperatorRegistration(id);
      toast.success('Регистрация одобрена');
      setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== id));
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    }
  }

  async function handleModalSubmit() {
    if (!modal || !modalText.trim()) return;
    setModalSubmitting(true);
    try {
      switch (modal.type) {
        case 'reject_sn':
          await resellerApi.rejectSenderName(modal.id, modalText);
          setSenderNames((prev) => prev.filter((sn) => sn.id !== modal.id));
          toast.success('Имя отклонено');
          break;
        case 'reject_tpl':
          await resellerApi.rejectTemplate(modal.id, modalText);
          setTemplates((prev) => prev.filter((t) => t.id !== modal.id));
          toast.success('Шаблон отклонён');
          break;
        case 'revision_tpl':
          await resellerApi.requestRevisionTemplate(modal.id, modalText);
          setTemplates((prev) => prev.filter((t) => t.id !== modal.id));
          toast.success('Запрос на доработку отправлен');
          break;
        case 'reject_reg':
          await resellerApi.rejectOperatorRegistration(modal.id, modalText);
          setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== modal.id));
          toast.success('Регистрация отклонена');
          break;
        case 'revision_reg':
          await resellerApi.requestRevisionOperatorRegistration(modal.id, modalText);
          setRegistrations((prev) => (prev as { id: string }[]).filter((r) => r.id !== modal.id));
          toast.success('Запрос на доработку отправлен');
          break;
      }
      resellerApi.getModerationCounts().then(setCounts).catch(() => {});
      setModal(null);
      setModalText('');
    } catch (e) {
      toast.error(e instanceof ApiError ? e.message : 'Ошибка');
    } finally {
      setModalSubmitting(false);
    }
  }

  const modalTitle =
    modal?.type === 'reject_sn' || modal?.type === 'reject_tpl' || modal?.type === 'reject_reg'
      ? 'Отклонить'
      : 'Запросить доработку';
  const modalLabel =
    modal?.type === 'revision_tpl' || modal?.type === 'revision_reg' ? 'Комментарий' : 'Причина отклонения';

  // --- Column defs ---
  const snColumns: Column<ResellerSenderName>[] = [
    { key: 'sub_account_email', header: 'Субаккаунт' },
    { key: 'name', header: 'Имя отправителя' },
    {
      key: 'status',
      header: 'Статус',
      render: (sn) => <StatusBadge status={sn.status} />,
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (sn) => new Date(sn.created_at).toLocaleDateString('ru-RU'),
    },
    {
      key: 'actions' as keyof ResellerSenderName,
      header: '',
      render: (sn) =>
        sn.status === 'pending' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveSN(sn.id)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_sn', id: sn.id }); setModalText(''); }}>
              Отклонить
            </Button>
          </div>
        ) : null,
    },
  ];

  const tplColumns: Column<ResellerTemplate>[] = [
    { key: 'sub_account_email', header: 'Субаккаунт' },
    { key: 'name', header: 'Название' },
    { key: 'body_preview', header: 'Содержание' },
    {
      key: 'status',
      header: 'Статус',
      render: (t) => <StatusBadge status={t.status} />,
    },
    {
      key: 'created_at',
      header: 'Создан',
      render: (t) => new Date(t.created_at).toLocaleDateString('ru-RU'),
    },
    {
      key: 'actions' as keyof ResellerTemplate,
      header: '',
      render: (t) =>
        t.status === 'pending' || t.status === 'revision_requested' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveTpl(t.id)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_tpl', id: t.id }); setModalText(''); }}>
              Отклонить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => { setModal({ type: 'revision_tpl', id: t.id }); setModalText(''); }}>
              Доработка
            </Button>
          </div>
        ) : null,
    },
  ];

  const regColumns: Column<Record<string, unknown>>[] = [
    { key: 'sub_account_id' as string, header: 'Субаккаунт' },
    { key: 'sender_name' as string, header: 'Имя отправителя' },
    { key: 'operator_name' as string, header: 'Оператор' },
    { key: 'registration_type' as string, header: 'Тип' },
    {
      key: 'status' as string,
      header: 'Статус',
      render: (r) => <StatusBadge status={r.status as string} />,
    },
    {
      key: 'actions' as string,
      header: '',
      render: (r) =>
        r.status === 'submitted' ? (
          <div className="flex gap-1">
            <Button size="sm" onClick={() => handleApproveReg(r.id as string)}>
              Одобрить
            </Button>
            <Button size="sm" variant="danger" onClick={() => { setModal({ type: 'reject_reg', id: r.id as string }); setModalText(''); }}>
              Отклонить
            </Button>
            <Button size="sm" variant="secondary" onClick={() => { setModal({ type: 'revision_reg', id: r.id as string }); setModalText(''); }}>
              Доработка
            </Button>
          </div>
        ) : null,
    },
  ];

  function StatusFilter({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: { value: string; label: string }[] }) {
    return (
      <select
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="border border-gray-300 rounded px-2 py-1 text-sm"
      >
        <option value="">Все статусы</option>
        {options.map((o) => (
          <option key={o.value} value={o.value}>{o.label}</option>
        ))}
      </select>
    );
  }

  return (
    <div className="max-w-6xl">
      <PageHeader title="Модерация" subtitle="Заявки субаккаунтов" />

      {/* Tabs */}
      <div className="flex gap-1 border-b border-gray-200 mb-4">
        <TabButton active={tab === 'sender_names'} onClick={() => setTab('sender_names')} count={counts.sender_names}>
          Имена отправителей
        </TabButton>
        <TabButton active={tab === 'templates'} onClick={() => setTab('templates')} count={counts.templates}>
          Шаблоны
        </TabButton>
        <TabButton active={tab === 'registrations'} onClick={() => setTab('registrations')} count={counts.registrations}>
          Регистрации у операторов
        </TabButton>
      </div>

      {/* Sender names tab */}
      {tab === 'sender_names' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={snStatusFilter}
              onChange={setSnStatusFilter}
              options={[
                { value: 'pending', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
              ]}
            />
          </div>
          {snLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : senderNames.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={snColumns} data={senderNames} total={senderNames.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Templates tab */}
      {tab === 'templates' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={tplStatusFilter}
              onChange={setTplStatusFilter}
              options={[
                { value: 'pending', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
                { value: 'revision_requested', label: 'На доработке' },
              ]}
            />
          </div>
          {tplLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : templates.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={tplColumns} data={templates} total={templates.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Registrations tab */}
      {tab === 'registrations' && (
        <>
          <div className="mb-3">
            <StatusFilter
              value={regStatusFilter}
              onChange={setRegStatusFilter}
              options={[
                { value: 'submitted', label: 'На рассмотрении' },
                { value: 'approved', label: 'Одобрено' },
                { value: 'rejected', label: 'Отклонено' },
                { value: 'revision_requested', label: 'На доработке' },
              ]}
            />
          </div>
          {regLoading ? (
            <div className="py-8 text-center text-gray-400">Загрузка...</div>
          ) : registrations.length === 0 ? (
            <div className="py-8 text-center text-gray-400">Нет заявок</div>
          ) : (
            <DataTable columns={regColumns} data={registrations as Record<string, unknown>[]} total={registrations.length} page={1} pageSize={50} onPageChange={() => {}} keyField="id" />
          )}
        </>
      )}

      {/* Reject / Request Revision modal */}
      <Modal
        open={!!modal}
        onClose={() => { setModal(null); setModalText(''); }}
        title={modalTitle}
      >
        <div className="flex flex-col gap-4">
          <Input
            label={modalLabel}
            value={modalText}
            onChange={(e) => setModalText(e.target.value)}
            required
            placeholder={modalLabel === 'Комментарий' ? 'Укажите, что нужно исправить' : 'Укажите причину'}
          />
          <div className="flex gap-2">
            <Button onClick={handleModalSubmit} disabled={modalSubmitting || !modalText.trim()}>
              {modalSubmitting ? 'Отправка...' : 'Подтвердить'}
            </Button>
            <Button variant="secondary" onClick={() => { setModal(null); setModalText(''); }}>
              Отмена
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/network/ModerationPage.tsx
git commit -m "feat(portal): add ModerationPage with 3 tabs for reseller moderation"
```

---

## Task 11: Wire ModerationPage into routes

**Files:**
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Replace moderation placeholder with real page**

In `portal-frontend/src/App.tsx`, add the import:

```tsx
import { ModerationPage } from './pages/network/ModerationPage';
```

Then replace the placeholder route:

```tsx
          <Route path="moderation" element={<div>Moderation placeholder</div>} />
```

With:

```tsx
          <Route path="moderation" element={<ModerationPage />} />
```

- [ ] **Step 2: Verify frontend build**

Run:
```bash
cd /c/projects/sms/portal-frontend && npx tsc --noEmit
```
Expected: No type errors.

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/App.tsx
git commit -m "feat(portal): wire ModerationPage into /network/moderation route"
```

---

## Task 12: Final verification

- [ ] **Step 1: Verify Go build**

```bash
cd /c/projects/sms && go build ./cmd/portal-gateway/
```
Expected: Compiles without errors.

- [ ] **Step 2: Verify frontend build**

```bash
cd /c/projects/sms/portal-frontend && npx tsc --noEmit
```
Expected: No type errors.

- [ ] **Step 3: Verify Vite dev build**

```bash
cd /c/projects/sms/portal-frontend && npx vite build 2>&1 | tail -5
```
Expected: Build completes successfully.

- [ ] **Step 4: Manual test checklist**

Start the dev server and verify in browser:

1. Login as a reseller user
2. See "Управление сетью →" switcher in the sidebar footer
3. Click it — navigates to `/network/sub-accounts`
4. See network sidebar with "Суб-аккаунты" and "Модерация"
5. Old `/sub-accounts` URL redirects to `/network/sub-accounts`
6. Click "Модерация" — see 3 tabs
7. Switch tabs — each loads data
8. Click "← Свой аккаунт" — returns to normal portal
9. Refresh — mode is preserved
10. Login as non-reseller — no switcher visible

- [ ] **Step 5: Final commit if any fixes needed**

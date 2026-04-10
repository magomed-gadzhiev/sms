# Message Detail — Live Status Updates via SSE

**Date:** 2026-04-10
**Status:** Approved

## Goal

Connect the existing SSE infrastructure to `MessageDetailPage` so that status changes are pushed to the detail view in real time, without polling. When the status becomes terminal the SSE connection is closed automatically.

## Context

The SSE infrastructure is fully operational:

- `internal/gateway/portal/sse/hub.go` — Kafka `sms.status` consumer, routes events per `client_id`
- `GET /portal/v1/messages/stream` — SSE endpoint (`StreamMessages` handler)
- `portal-frontend/src/hooks/useMessageStream.ts` — React hook; exposes `updates: Record<string, MessageStatusEvent>`, `streamStatus`, `close()`

`MessagesPage` already uses `useMessageStream` for live status badges in the list. `MessageDetailPage` loads data once on mount and never refreshes — the only gap.

## Scope

**One file changed:** `portal-frontend/src/pages/messages/MessageDetailPage.tsx`

No backend changes. No new files. No new hooks.

## Design

### Terminal statuses

```ts
const TERMINAL_STATUSES = new Set(['delivered', 'failed', 'expired', 'rejected']);
```

### SSE integration in `MessageDetailPage`

```ts
// Enable SSE only when the loaded message is in a non-terminal state.
const streamEnabled = !!message && !TERMINAL_STATUSES.has(message.status);
const { streamStatus, updates, close } = useMessageStream(streamEnabled);

useEffect(() => {
  if (!id || !message) return;
  const update = updates[id];
  if (!update) return;

  // Re-fetch full detail (status, timestamps, DLR, billing).
  loadMessage();

  if (TERMINAL_STATUSES.has(update.status)) {
    close(); // unsubscribe — no more updates possible
  }
}, [updates[id]?.status]); // fires only on status change for this message
```

### Data flow

```
Kafka sms.status
  → SSE Hub (hub.go)
    → GET /portal/v1/messages/stream
      → useMessageStream → updates[id].status changes
        → loadMessage() → GET /messages/{id} → full re-render
        → if terminal → close()
```

### Visual indicator

In `PageHeader` actions, show a pulsing green "Live" dot while `streamStatus === 'connected'` and the message is not terminal — identical pattern to `MessagesPage`. The dot disappears automatically when `close()` is called.

```tsx
{streamEnabled && streamStatus === 'connected' && (
  <span className="flex items-center gap-1 text-xs text-green-600">
    <span className="inline-block w-2 h-2 rounded-full bg-green-500 animate-pulse" />
    Live
  </span>
)}
```

## Edge Cases

| Situation | Behaviour |
|---|---|
| Page opened on already-terminal message | `streamEnabled = false`; SSE never opens |
| SSE unavailable / error | `useMessageStream` retries with backoff; detail page shows data from initial fetch |
| Two rapid status updates | `loadMessage` called twice — idempotent; second call returns latest state |
| `close()` called before `loadMessage` resolves | Not a problem — `close` only closes SSE; in-flight fetch completes normally |
| User navigates away before terminal status | `useMessageStream` cleanup in its own `useEffect` closes the connection |

## Implementation

### Changes to `MessageDetailPage.tsx`

1. Add `useMessageStream` import
2. Add `TERMINAL_STATUSES` constant
3. Add `streamEnabled` derived value
4. Call `useMessageStream(streamEnabled)` — destructure `streamStatus`, `updates`, `close`
5. Add `useEffect` watching `updates[id]?.status` — call `loadMessage()`, then `close()` if terminal
6. Add Live dot to `PageHeader` actions

No other files change.

## Out of Scope

- WebSocket upgrade (SSE is sufficient for unidirectional status push)
- Extending SSE events with additional fields (timestamps, DLR) — refetch provides all data
- Applying partial updates without refetch
- Auto-refresh on the messages list (already works via SSE)

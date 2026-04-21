# Test Credentials (sandbox server)

Last updated: 2026-04-22

All accounts below live on `sms-server` and are re-seeded by
`test/load/fixtures/seed.sql`. Passwords here are checked into the repo
on purpose — this is a dev sandbox per `project_server_is_sandbox.md`.
Do not reuse any of these credentials on production.

## Portal portal users

| Email                   | Password    | Role   | Client (parent)                      | Notes                               |
|-------------------------|-------------|--------|--------------------------------------|-------------------------------------|
| `loadtest-admin@test.local`  | `Admin123!` | admin  | —                                    | seed.sql §4                         |
| `loadtest-client@test.local` | `Admin123!` | client | `c0000000-…-000000000001`            | seed.sql §4                         |
| `subacc@test.local`     | `Test1234!` | client | `a0000000-…-000000000002` (TestSubAccount, parent = `a0000000-…-000000000001`) | sub-account QA (generic)            |
| `clean-a@test.local`    | `Test1234!` | client | `f6c4b3fd-…` (Test Clean A)          | sub-account cycle 3 test (A path)   |
| `problem-b@test.local`  | `Test1234!` | client | `6063f9b6-…` (Test Problem B)        | sub-account cycle 3 test (B path)   |
| `heavy-c@test.local`    | `Test1234!` | client | `de3712f8-…` (Test Heavy C)          | sub-account cycle 3 test (C path)   |

Verify login (from server):

```bash
curl -s -o /dev/null -w '%{http_code}\n' \
  -X POST http://localhost:18084/portal/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"subacc@test.local","password":"Test1234!"}'
# expected: 200
```

## History

- 2026-04-22: reset `subacc/clean-a/problem-b/heavy-c` from unknown-plaintext
  bcrypt hash (`$2a$10$xF1l8O6cAKP9A...`) to `Test1234!`. The unknown hash
  had been blocking all portal sub-account QA (bug #8). Added idempotent
  re-seed block in `test/load/fixtures/seed.sql` §10 so future
  `scripts/server.sh seed` restores the known password.

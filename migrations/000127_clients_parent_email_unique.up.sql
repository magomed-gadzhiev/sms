-- BUG-80 / D.6: close TOCTOU race in SubAccountService.CreateSubAccount.
-- Application-level check (ExistsByEmailUnderParent) runs SELECT then INSERT,
-- so two concurrent requests with the same (parent, email) can both pass the
-- check and produce duplicate rows. The unique index enforces atomicity at
-- the DB layer.
--
-- Predicate matches the application semantics in
-- sub_account_service.go:115-117: email is reserved across the soft-delete
-- boundary (no `active = true` filter), so admins cannot recreate an offboarded
-- sub-account under the same email — that is intentional and required for
-- login-by-email to keep returning a single row. Empty/NULL emails are
-- excluded so multiple sub-accounts without email coexist.

CREATE UNIQUE INDEX IF NOT EXISTS idx_clients_parent_email
    ON clients (parent_client_id, lower(email))
    WHERE parent_client_id IS NOT NULL AND email IS NOT NULL AND email <> '';

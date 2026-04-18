-- Add use_subscriber_timezone flag to campaigns.
-- Closes drift D-01 (docs/ac/_DRIFT.md): field was in frontend state and API
-- request body but had no backend persistence — silent data loss.
--
-- Spec: docs/superpowers/specs/2026-04-13-campaign-wizard-redesign.md §"Шаг 3"
-- "Доставить в указанное время по часовому поясу абонента"

ALTER TABLE campaigns
    ADD COLUMN use_subscriber_timezone BOOLEAN NOT NULL DEFAULT FALSE;

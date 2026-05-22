-- migrations/000049_local_time_delivery.down.sql
DROP TABLE IF EXISTS client_quiet_hours;
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS deliver_at;

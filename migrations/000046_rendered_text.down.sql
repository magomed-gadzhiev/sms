-- migrations/000046_rendered_text.down.sql
ALTER TABLE campaign_recipients DROP COLUMN IF EXISTS rendered_text;

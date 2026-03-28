-- migrations/000046_rendered_text.up.sql
ALTER TABLE campaign_recipients ADD COLUMN rendered_text TEXT;

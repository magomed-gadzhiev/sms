-- migrations/000047_link_service.down.sql
DROP TABLE IF EXISTS click_events CASCADE;
DROP TABLE IF EXISTS short_links CASCADE;
DROP TABLE IF EXISTS client_domains CASCADE;

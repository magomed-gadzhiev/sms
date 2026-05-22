-- Add IP whitelist support to API keys
ALTER TABLE api_keys ADD COLUMN allowed_ips TEXT[] DEFAULT NULL;

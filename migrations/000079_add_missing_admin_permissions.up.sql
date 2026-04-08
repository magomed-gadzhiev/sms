-- Add missing permissions for admin panel resources
INSERT INTO permissions (id, resource, action, description) VALUES
    ('10000000-0000-0000-0000-000000000019', 'tarification', 'read',   'Просмотр тарификации'),
    ('10000000-0000-0000-0000-000000000020', 'tarification', 'write',  'Управление тарификацией'),
    ('10000000-0000-0000-0000-000000000021', 'tarification', 'delete', 'Удаление тарифов'),
    ('10000000-0000-0000-0000-000000000022', 'hlr',          'read',   'Просмотр HLR-провайдеров'),
    ('10000000-0000-0000-0000-000000000023', 'hlr',          'write',  'Управление HLR-провайдерами'),
    ('10000000-0000-0000-0000-000000000024', 'hlr',          'delete', 'Удаление HLR-провайдеров'),
    ('10000000-0000-0000-0000-000000000025', 'countries',    'read',   'Просмотр стран и операторов'),
    ('10000000-0000-0000-0000-000000000026', 'countries',    'write',  'Управление странами и операторами'),
    ('10000000-0000-0000-0000-000000000027', 'countries',    'delete', 'Удаление стран и операторов'),
    ('10000000-0000-0000-0000-000000000028', 'webhooks',     'read',   'Просмотр вебхуков'),
    ('10000000-0000-0000-0000-000000000029', 'webhooks',     'write',  'Управление вебхуками'),
    ('10000000-0000-0000-0000-000000000030', 'webhooks',     'delete', 'Удаление вебхуков'),
    ('10000000-0000-0000-0000-000000000031', 'audit',        'read',   'Просмотр аудита')
ON CONFLICT (resource, action) DO NOTHING;

-- Assign all new permissions to admin role
INSERT INTO role_permissions (role_id, permission_id)
SELECT '00000000-0000-0000-0000-000000000001', id
FROM permissions
WHERE resource IN ('tarification', 'hlr', 'countries', 'webhooks', 'audit')
ON CONFLICT DO NOTHING;

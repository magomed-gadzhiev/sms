#!/usr/bin/env node

/**
 * Скрипт настройки коллекций в Directus
 * 
 * Настраивает метаданные полей, скрывает чувствительные поля,
 * настраивает relationships между коллекциями.
 * 
 * Использование:
 *   node scripts/setup-directus.js [--url http://localhost:8055] [--email admin@example.com] [--password admin]
 */

const http = require('http');
const https = require('https');
const { URL } = require('url');

// Конфигурация по умолчанию
const DEFAULT_URL = process.env.DIRECTUS_URL || 'http://localhost:8055';
const DEFAULT_EMAIL = process.env.DIRECTUS_ADMIN_EMAIL || 'admin@example.com';
const DEFAULT_PASSWORD = process.env.DIRECTUS_ADMIN_PASSWORD || 'admin';

// Парсинг аргументов командной строки
const args = process.argv.slice(2);
let directusUrl = DEFAULT_URL;
let adminEmail = DEFAULT_EMAIL;
let adminPassword = DEFAULT_PASSWORD;

for (let i = 0; i < args.length; i++) {
    if (args[i] === '--url' && args[i + 1]) {
        directusUrl = args[i + 1];
        i++;
    } else if (args[i] === '--email' && args[i + 1]) {
        adminEmail = args[i + 1];
        i++;
    } else if (args[i] === '--password' && args[i + 1]) {
        adminPassword = args[i + 1];
        i++;
    }
}

let authToken = null;

// Вспомогательная функция для HTTP запросов
function httpRequest(options, data = null) {
    return new Promise((resolve, reject) => {
        const url = new URL(options.url || options.path, directusUrl);
        const isHttps = url.protocol === 'https:';
        const httpModule = isHttps ? https : http;

        const requestOptions = {
            hostname: url.hostname,
            port: url.port || (isHttps ? 443 : 80),
            path: url.pathname + url.search,
            method: options.method || 'GET',
            headers: {
                'Content-Type': 'application/json',
                ...(authToken && { 'Authorization': `Bearer ${authToken}` }),
                ...(options.headers || {})
            }
        };

        const req = httpModule.request(requestOptions, (res) => {
            let body = '';
            res.on('data', (chunk) => body += chunk);
            res.on('end', () => {
                try {
                    const jsonBody = body ? JSON.parse(body) : {};
                    if (res.statusCode >= 200 && res.statusCode < 300) {
                        resolve({ status: res.statusCode, data: jsonBody });
                    } else {
                        reject(new Error(`HTTP ${res.statusCode}: ${JSON.stringify(jsonBody)}`));
                    }
                } catch (e) {
                    resolve({ status: res.statusCode, data: body });
                }
            });
        });

        req.on('error', reject);

        if (data) {
            req.write(JSON.stringify(data));
        }

        req.end();
    });
}

// Аутентификация
async function authenticate() {
    console.log('🔐 Аутентификация в Directus...');
    try {
        const response = await httpRequest({
            url: '/auth/login',
            method: 'POST'
        }, {
            email: adminEmail,
            password: adminPassword
        });
        authToken = response.data.data.access_token;
        console.log('✓ Успешная аутентификация');
        return true;
    } catch (error) {
        console.error('✗ Ошибка аутентификации:', error.message);
        return false;
    }
}

// Получение всех полей коллекции
async function getFields(collection) {
    try {
        const response = await httpRequest({
            url: `/fields/${collection}`,
            method: 'GET'
        });
        return response.data.data || [];
    } catch (error) {
        console.error(`✗ Ошибка получения полей коллекции ${collection}:`, error.message);
        return [];
    }
}

// Проверка существования коллекции
async function collectionExists(collection) {
    try {
        const response = await httpRequest({
            url: `/collections/${collection}`,
            method: 'GET'
        });
        return true;
    } catch (error) {
        return false;
    }
}

// Получение существующей relationship
async function getRelation(collection, field) {
    try {
        const response = await httpRequest({
            url: `/relations/${collection}/${field}`,
            method: 'GET'
        });
        return response.data.data;
    } catch (error) {
        return null;
    }
}

// Создание или обновление relationship
async function setupRelation(collection, field, relatedCollection, options = {}) {
    try {
        // Проверяем, существует ли уже relationship
        const existing = await getRelation(collection, field);
        
        const relationData = {
            collection: collection,
            field: field,
            related_collection: relatedCollection,
            schema: {
                on_delete: options.onDelete || 'SET NULL',
            },
            meta: {
                one_field: options.oneField || null,
                one_collection_field: options.oneCollectionField || null,
                one_allowed_collections: options.oneAllowedCollections || null,
                many_field: options.manyField || null,
                many_collection_field: options.manyCollectionField || null,
                many_allowed_collections: options.manyAllowedCollections || null,
                one_deselect_action: options.oneDeselectAction || 'nullify',
                junction_field: options.junctionField || null,
                ...(options.meta || {})
            }
        };

        if (existing) {
            // Обновляем существующую relationship
            await httpRequest({
                url: `/relations/${collection}/${field}`,
                method: 'PATCH'
            }, relationData);
            console.log(`  ✓ Обновлена relationship: ${collection}.${field} -> ${relatedCollection}`);
        } else {
            // Создаем новую relationship
            await httpRequest({
                url: '/relations',
                method: 'POST'
            }, relationData);
            console.log(`  ✓ Создана relationship: ${collection}.${field} -> ${relatedCollection}`);
        }
    } catch (error) {
        console.error(`  ✗ Ошибка настройки relationship ${collection}.${field}:`, error.message);
    }
}

// Обновление метаданных поля
async function updateField(collection, field, updates) {
    try {
        // Сначала получаем текущие метаданные поля
        const fields = await getFields(collection);
        const fieldData = fields.find(f => f.field === field);
        
        if (!fieldData) {
            console.log(`  ⚠ Поле ${collection}.${field} не найдено, пропускаем`);
            return;
        }

        // Объединяем текущие метаданные с обновлениями
        const mergedUpdates = {
            ...fieldData,
            meta: {
                ...fieldData.meta,
                ...updates.meta
            },
            ...(updates.schema && { schema: { ...fieldData.schema, ...updates.schema } })
        };

        await httpRequest({
            url: `/fields/${collection}/${field}`,
            method: 'PATCH'
        }, mergedUpdates);
        console.log(`  ✓ Обновлено поле: ${collection}.${field}`);
    } catch (error) {
        console.error(`  ✗ Ошибка обновления поля ${collection}.${field}:`, error.message);
    }
}

// Настройка коллекции users
async function setupUsersCollection() {
    console.log('\n📋 Настройка коллекции users...');
    
    if (!await collectionExists('users')) {
        console.log('  ⚠ Коллекция users не найдена, пропускаем');
        return;
    }
    
    // Скрыть password_hash полностью
    await updateField('users', 'password_hash', {
        meta: {
            hidden: true,
            interface: 'input-hash',
            readonly: true
        }
    });

    // Настроить role_id как relationship с roles
    await setupRelation('users', 'role_id', 'roles', {
        onDelete: 'RESTRICT'
    });
    
    await updateField('users', 'role_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            options: {
                template: '{{name}}'
            }
        }
    });
}

// Настройка коллекции roles
async function setupRolesCollection() {
    console.log('\n📋 Настройка коллекции roles...');
    
    if (!await collectionExists('roles')) {
        console.log('  ⚠ Коллекция roles не найдена, пропускаем');
        return;
    }
    
    // Настраиваем отображение полей
    await updateField('roles', 'name', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true
        }
    });
    
    await updateField('roles', 'description', {
        meta: {
            interface: 'input-multiline',
            width: 'half'
        }
    });
}

// Настройка коллекции clients
async function setupClientsCollection() {
    console.log('\n📋 Настройка коллекции clients...');
    
    if (!await collectionExists('clients')) {
        console.log('  ⚠ Коллекция clients не найдена, пропускаем');
        return;
    }
    
    // secret - скрыть полностью (защищенное поле)
    await updateField('clients', 'secret', {
        meta: {
            hidden: true,
            readonly: true,
            interface: 'input',
            options: {
                masked: true,
                iconRight: 'lock'
            }
        }
    });

    // api_key - только для чтения, скрыто для обычных пользователей
    // Показывается только администраторам в режиме только чтения
    await updateField('clients', 'api_key', {
        meta: {
            hidden: true,
            readonly: true,
            interface: 'input',
            options: {
                font: 'monospace',
                iconRight: 'vpn_key'
            }
        }
    });

    // Настроить allowed_source_addresses как теги
    await updateField('clients', 'allowed_source_addresses', {
        meta: {
            interface: 'tags',
            width: 'full'
        }
    });
}

// Настройка коллекции providers
async function setupProvidersCollection() {
    console.log('\n📋 Настройка коллекции providers...');
    
    if (!await collectionExists('providers')) {
        console.log('  ⚠ Коллекция providers не найдена, пропускаем');
        return;
    }
    
    // password - скрыть полностью (защищенное поле)
    await updateField('providers', 'password', {
        meta: {
            hidden: true,
            readonly: true,
            interface: 'input',
            options: {
                masked: true,
                iconRight: 'lock'
            }
        }
    });

    // system_id - только для чтения (чувствительное поле)
    await updateField('providers', 'system_id', {
        meta: {
            readonly: true,
            interface: 'input',
            width: 'half',
            required: true,
            options: {
                font: 'monospace'
            }
        }
    });
}

// Настройка коллекции routes
async function setupRoutesCollection() {
    console.log('\n📋 Настройка коллекции routes...');
    
    if (!await collectionExists('routes')) {
        console.log('  ⚠ Коллекция routes не найдена, пропускаем');
        return;
    }
    
    // Настроить provider_id как relationship с providers
    await setupRelation('routes', 'provider_id', 'providers', {
        onDelete: 'RESTRICT'
    });
    
    await updateField('routes', 'provider_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}}'
            }
        }
    });

    // Настроить failover_provider_id как relationship с providers (опциональный)
    await setupRelation('routes', 'failover_provider_id', 'providers', {
        onDelete: 'SET NULL'
    });
    
    await updateField('routes', 'failover_provider_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: false,
            options: {
                template: '{{name}}'
            }
        }
    });
}

// Настройка коллекции accounts
async function setupAccountsCollection() {
    console.log('\n📋 Настройка коллекции accounts...');
    
    if (!await collectionExists('accounts')) {
        console.log('  ⚠ Коллекция accounts не найдена, пропускаем');
        return;
    }
    
    // Настроить client_id как relationship с clients
    await setupRelation('accounts', 'client_id', 'clients', {
        onDelete: 'CASCADE'
    });
    
    await updateField('accounts', 'client_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}}'
            }
        }
    });

    // Настроить balance как число с форматированием
    await updateField('accounts', 'balance', {
        meta: {
            interface: 'input',
            width: 'half',
            readonly: true, // Баланс обычно изменяется через транзакции
            options: {
                step: 0.000001,
                iconRight: 'attach_money'
            }
        }
    });
    
    // Настроить currency
    await updateField('accounts', 'currency', {
        meta: {
            interface: 'select-dropdown',
            width: 'half',
            options: {
                choices: [
                    { text: 'RUB', value: 'RUB' }
                ]
            }
        }
    });
}

// Настройка коллекции transactions
async function setupTransactionsCollection() {
    console.log('\n📋 Настройка коллекции transactions...');
    
    if (!await collectionExists('transactions')) {
        console.log('  ⚠ Коллекция transactions не найдена, пропускаем');
        return;
    }
    
    // Настроить client_id как relationship с clients
    await setupRelation('transactions', 'client_id', 'clients', {
        onDelete: 'CASCADE'
    });
    
    await updateField('transactions', 'client_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}}'
            }
        }
    });

    // Настроить message_id как relationship с messages (опциональный)
    // Примечание: messages партиционирована, relationship может не работать идеально
    try {
        await setupRelation('transactions', 'message_id', 'messages', {
            onDelete: 'SET NULL'
        });
        
        await updateField('transactions', 'message_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: false
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship с messages (возможно, партиционированная таблица)');
    }

    // Настроить type как выпадающий список
    await updateField('transactions', 'type', {
        meta: {
            interface: 'select-dropdown',
            width: 'half',
            required: true,
            options: {
                choices: [
                    { text: 'Charge', value: 'charge' },
                    { text: 'Credit', value: 'credit' },
                    { text: 'Refund', value: 'refund' },
                    { text: 'Adjustment', value: 'adjustment' }
                ]
            }
        }
    });

    // Настроить amount как число
    await updateField('transactions', 'amount', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true,
            options: {
                step: 0.000001,
                iconRight: 'attach_money'
            }
        }
    });

    // Настроить balance_before и balance_after (только для чтения)
    await updateField('transactions', 'balance_before', {
        meta: {
            interface: 'input',
            width: 'half',
            readonly: true,
            options: {
                step: 0.000001
            }
        }
    });

    await updateField('transactions', 'balance_after', {
        meta: {
            interface: 'input',
            width: 'half',
            readonly: true,
            options: {
                step: 0.000001
            }
        }
    });

    // Настроить metadata как JSON
    await updateField('transactions', 'metadata', {
        meta: {
            interface: 'input-code',
            width: 'full',
            options: {
                language: 'json',
                lineNumber: true
            }
        }
    });
}

// Настройка коллекции pricing_rules
async function setupPricingRulesCollection() {
    console.log('\n📋 Настройка коллекции pricing_rules...');
    
    if (!await collectionExists('pricing_rules')) {
        console.log('  ⚠ Коллекция pricing_rules не найдена, пропускаем');
        return;
    }
    
    // Настроить client_id как relationship с clients (опциональный, может быть NULL для глобальных правил)
    await setupRelation('pricing_rules', 'client_id', 'clients', {
        onDelete: 'CASCADE'
    });
    
    await updateField('pricing_rules', 'client_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: false,
            options: {
                template: '{{name}}',
                allowNone: true
            }
        }
    });

    // Настроить destination_pattern
    await updateField('pricing_rules', 'destination_pattern', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true,
            options: {
                placeholder: 'Regex pattern или prefix'
            }
        }
    });

    // Настроить price_per_message как число
    await updateField('pricing_rules', 'price_per_message', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true,
            options: {
                step: 0.000001,
                iconRight: 'attach_money'
            }
        }
    });
    
    // Настроить priority
    await updateField('pricing_rules', 'priority', {
        meta: {
            interface: 'input',
            width: 'half',
            options: {
                step: 1,
                min: 0
            }
        }
    });
    
    // Настроить active как checkbox
    await updateField('pricing_rules', 'active', {
        meta: {
            interface: 'boolean',
            width: 'half',
            options: {
                label: 'Активно'
            }
        }
    });
}

// Настройка коллекции api_keys
async function setupApiKeysCollection() {
    console.log('\n📋 Настройка коллекции api_keys...');
    
    if (!await collectionExists('api_keys')) {
        console.log('  ⚠ Коллекция api_keys не найдена, пропускаем');
        return;
    }
    
    // key_hash - скрыть полностью (защищенное поле)
    await updateField('api_keys', 'key_hash', {
        meta: {
            hidden: true,
            readonly: true,
            interface: 'input-hash',
            options: {
                masked: true,
                iconRight: 'lock'
            }
        }
    });

    // Настроить user_id как relationship с users
    await setupRelation('api_keys', 'user_id', 'users', {
        onDelete: 'CASCADE'
    });
    
    await updateField('api_keys', 'user_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{username}} ({{email}})'
            }
        }
    });

    // key_prefix - только для чтения, показывает первые символы ключа
    await updateField('api_keys', 'key_prefix', {
        meta: {
            readonly: true,
            interface: 'input',
            width: 'half',
            options: {
                font: 'monospace',
                iconRight: 'vpn_key'
            }
        }
    });

    // active - checkbox
    await updateField('api_keys', 'active', {
        meta: {
            interface: 'boolean',
            width: 'half',
            options: {
                label: 'Активен'
            }
        }
    });

    // expires_at - дата и время
    await updateField('api_keys', 'expires_at', {
        meta: {
            interface: 'datetime',
            width: 'half',
            options: {
                includeSeconds: true
            }
        }
    });

    // last_used_at - только для чтения
    await updateField('api_keys', 'last_used_at', {
        meta: {
            readonly: true,
            interface: 'datetime',
            width: 'half',
            options: {
                includeSeconds: true
            }
        }
    });
}

// Настройка коллекции refresh_tokens
async function setupRefreshTokensCollection() {
    console.log('\n📋 Настройка коллекции refresh_tokens...');
    
    if (!await collectionExists('refresh_tokens')) {
        console.log('  ⚠ Коллекция refresh_tokens не найдена, пропускаем');
        return;
    }
    
    // token_hash - скрыть полностью (защищенное поле)
    await updateField('refresh_tokens', 'token_hash', {
        meta: {
            hidden: true,
            readonly: true,
            interface: 'input-hash',
            options: {
                masked: true,
                iconRight: 'lock'
            }
        }
    });

    // Настроить user_id как relationship с users
    await setupRelation('refresh_tokens', 'user_id', 'users', {
        onDelete: 'CASCADE'
    });
    
    await updateField('refresh_tokens', 'user_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{username}} ({{email}})'
            }
        }
    });

    // expires_at - дата и время, только для чтения
    await updateField('refresh_tokens', 'expires_at', {
        meta: {
            readonly: true,
            interface: 'datetime',
            width: 'half',
            required: true,
            options: {
                includeSeconds: true
            }
        }
    });

    // revoked - checkbox, только для чтения
    await updateField('refresh_tokens', 'revoked', {
        meta: {
            readonly: true,
            interface: 'boolean',
            width: 'half',
            options: {
                label: 'Отозван'
            }
        }
    });

    // revoked_at - дата и время, только для чтения
    await updateField('refresh_tokens', 'revoked_at', {
        meta: {
            readonly: true,
            interface: 'datetime',
            width: 'half',
            options: {
                includeSeconds: true
            }
        }
    });
}

// Настройка коллекции messages
async function setupMessagesCollection() {
    console.log('\n📋 Настройка коллекции messages...');
    
    if (!await collectionExists('messages')) {
        console.log('  ⚠ Коллекция messages не найдена, пропускаем');
        return;
    }
    
    // Настроить provider_id как relationship с providers (опциональный)
    try {
        await setupRelation('messages', 'provider_id', 'providers', {
            onDelete: 'SET NULL'
        });
        
        await updateField('messages', 'provider_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: false,
                options: {
                    template: '{{name}}'
                }
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship messages.provider_id:', error.message);
    }

    // Настроить route_id как relationship с routes (опциональный)
    try {
        await setupRelation('messages', 'route_id', 'routes', {
            onDelete: 'SET NULL'
        });
        
        await updateField('messages', 'route_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: false,
                options: {
                    template: '{{name}}'
                }
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship messages.route_id:', error.message);
    }

    // Настроить client_id как relationship с clients (опциональный)
    try {
        await setupRelation('messages', 'client_id', 'clients', {
            onDelete: 'SET NULL'
        });
        
        await updateField('messages', 'client_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: false,
                options: {
                    template: '{{name}}'
                }
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship messages.client_id:', error.message);
    }
}

// Настройка коллекции role_permissions
async function setupRolePermissionsCollection() {
    console.log('\n📋 Настройка коллекции role_permissions...');
    
    if (!await collectionExists('role_permissions')) {
        console.log('  ⚠ Коллекция role_permissions не найдена, пропускаем');
        return;
    }
    
    // Настроить role_id как relationship с roles
    await setupRelation('role_permissions', 'role_id', 'roles', {
        onDelete: 'CASCADE'
    });
    
    await updateField('role_permissions', 'role_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}}'
            }
        }
    });

    // Настроить permission_id как relationship с permissions
    await setupRelation('role_permissions', 'permission_id', 'permissions', {
        onDelete: 'CASCADE'
    });
    
    await updateField('role_permissions', 'permission_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{resource}}.{{action}}'
            }
        }
    });
}

// Настройка коллекции api_key_scopes
async function setupApiKeyScopesCollection() {
    console.log('\n📋 Настройка коллекции api_key_scopes...');
    
    if (!await collectionExists('api_key_scopes')) {
        console.log('  ⚠ Коллекция api_key_scopes не найдена, пропускаем');
        return;
    }
    
    // Настроить api_key_id как relationship с api_keys
    await setupRelation('api_key_scopes', 'api_key_id', 'api_keys', {
        onDelete: 'CASCADE'
    });
    
    await updateField('api_key_scopes', 'api_key_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}} ({{key_prefix}})'
            }
        }
    });

    // Настроить scope как текстовое поле
    await updateField('api_key_scopes', 'scope', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true,
            options: {
                placeholder: 'Например: messages:read'
            }
        }
    });
}

// Настройка коллекции client_configs
async function setupClientConfigsCollection() {
    console.log('\n📋 Настройка коллекции client_configs...');
    
    if (!await collectionExists('client_configs')) {
        console.log('  ⚠ Коллекция client_configs не найдена, пропускаем');
        return;
    }
    
    // Настроить client_id как relationship с clients (уникальный)
    await setupRelation('client_configs', 'client_id', 'clients', {
        onDelete: 'CASCADE'
    });
    
    await updateField('client_configs', 'client_id', {
        meta: {
            interface: 'select-dropdown-m2o',
            width: 'half',
            required: true,
            options: {
                template: '{{name}}'
            }
        }
    });

    // Настроить allowed_sources и blocked_destinations как теги
    await updateField('client_configs', 'allowed_sources', {
        meta: {
            interface: 'tags',
            width: 'half'
        }
    });

    await updateField('client_configs', 'blocked_destinations', {
        meta: {
            interface: 'tags',
            width: 'half'
        }
    });

    // Настроить settings как JSON
    await updateField('client_configs', 'settings', {
        meta: {
            interface: 'input-code',
            width: 'full',
            options: {
                language: 'json',
                lineNumber: true
            }
        }
    });
}

// Настройка коллекции dlr_receipts
async function setupDlrReceiptsCollection() {
    console.log('\n📋 Настройка коллекции dlr_receipts...');
    
    if (!await collectionExists('dlr_receipts')) {
        console.log('  ⚠ Коллекция dlr_receipts не найдена, пропускаем');
        return;
    }
    
    // Настроить provider_id как relationship с providers (опциональный)
    try {
        await setupRelation('dlr_receipts', 'provider_id', 'providers', {
            onDelete: 'SET NULL'
        });
        
        await updateField('dlr_receipts', 'provider_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: false,
                options: {
                    template: '{{name}}'
                }
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship dlr_receipts.provider_id:', error.message);
    }

    // Настроить message_id как relationship с messages
    // Примечание: messages партиционирована с составным ключом (id, created_at),
    // но настраиваем relationship только по message_id
    try {
        await setupRelation('dlr_receipts', 'message_id', 'messages', {
            onDelete: 'CASCADE'
        });
        
        await updateField('dlr_receipts', 'message_id', {
            meta: {
                interface: 'select-dropdown-m2o',
                width: 'half',
                required: true
            }
        });
    } catch (error) {
        console.log('  ⚠ Не удалось настроить relationship dlr_receipts.message_id (возможно, партиционированная таблица или составной ключ):', error.message);
    }

    // message_created_at - поле для составного foreign key
    // Directus может не поддерживать составные foreign keys полностью,
    // поэтому это поле остается как обычное поле
    await updateField('dlr_receipts', 'message_created_at', {
        meta: {
            interface: 'datetime',
            width: 'half',
            required: true,
            options: {
                includeSeconds: true
            }
        }
    });

    // Настроить stat как выпадающий список
    await updateField('dlr_receipts', 'stat', {
        meta: {
            interface: 'input',
            width: 'half',
            required: true
        }
    });
}

// Получение всех ролей Directus
async function getRoles() {
    try {
        const response = await httpRequest({
            url: '/roles',
            method: 'GET'
        });
        return response.data.data || [];
    } catch (error) {
        console.error('✗ Ошибка получения ролей:', error.message);
        return [];
    }
}

// Получение роли по имени
async function getRoleByName(name) {
    const roles = await getRoles();
    return roles.find(role => role.name === name) || null;
}

// Создание или получение роли
async function createOrGetRole(name, description) {
    try {
        let role = await getRoleByName(name);
        
        if (role) {
            console.log(`  ✓ Роль "${name}" уже существует (ID: ${role.id})`);
            return role;
        }

        // Структура для Directus (может отличаться в зависимости от версии)
        const roleData = {
            name: name,
            description: description,
            app_access: true
        };

        // Для Directus до версии 11
        if (name === 'Administrator') {
            roleData.admin_access = true;
        } else {
            roleData.admin_access = false;
        }

        const response = await httpRequest({
            url: '/roles',
            method: 'POST'
        }, roleData);

        role = response.data.data;
        console.log(`  ✓ Создана роль "${name}" (ID: ${role.id})`);
        return role;
    } catch (error) {
        // Возможно, используется Directus 11 с другой структурой
        try {
            const roleData = {
                name: name,
                description: description
            };
            const response = await httpRequest({
                url: '/roles',
                method: 'POST'
            }, roleData);
            const role = response.data.data;
            console.log(`  ✓ Создана роль "${name}" (ID: ${role.id})`);
            return role;
        } catch (retryError) {
            console.error(`  ✗ Ошибка создания роли "${name}":`, error.message);
            return null;
        }
    }
}

// Получение всех permissions для роли
async function getRolePermissions(roleId) {
    try {
        const response = await httpRequest({
            url: `/permissions?filter[role][_eq]=${roleId}`,
            method: 'GET'
        });
        return response.data.data || [];
    } catch (error) {
        console.error(`  ✗ Ошибка получения permissions для роли ${roleId}:`, error.message);
        return [];
    }
}

// Создание или обновление permission
async function createOrUpdatePermission(roleId, collection, action, permissionData) {
    try {
        // Проверяем существование permission
        const existingPermissions = await getRolePermissions(roleId);
        const existing = existingPermissions.find(
            p => p.collection === collection && p.action === action
        );

        // Базовая структура permission
        const permissionPayload = {
            role: roleId,
            collection: collection,
            action: action
        };

        // Добавляем дополнительные поля
        if (permissionData.permissions !== undefined) {
            permissionPayload.permissions = permissionData.permissions;
        }
        if (permissionData.validation !== undefined) {
            permissionPayload.validation = permissionData.validation;
        }
        if (permissionData.presets !== undefined) {
            permissionPayload.presets = permissionData.presets;
        }
        if (permissionData.fields !== undefined) {
            permissionPayload.fields = permissionData.fields;
        }

        if (existing) {
            // Обновляем существующий permission
            await httpRequest({
                url: `/permissions/${existing.id}`,
                method: 'PATCH'
            }, permissionPayload);
            console.log(`    ✓ Обновлен permission: ${collection}.${action}`);
        } else {
            // Создаем новый permission
            await httpRequest({
                url: '/permissions',
                method: 'POST'
            }, permissionPayload);
            console.log(`    ✓ Создан permission: ${collection}.${action}`);
        }
    } catch (error) {
        // Возможно, используется другой формат API (например, Directus 11)
        try {
            // Повторно получаем существующие permissions
            const existingPermissions = await getRolePermissions(roleId);
            const existing = existingPermissions.find(
                p => p.collection === collection && p.action === action
            );

            // Пробуем использовать формат с access: 'full'
            const permissionPayload = {
                role: roleId,
                collection: collection,
                action: action,
                access: permissionData.permissions && Object.keys(permissionData.permissions).length === 0 ? 'full' : 'custom',
                ...(permissionData.permissions && Object.keys(permissionData.permissions).length > 0 && {
                    permissions: permissionData.permissions
                })
            };

            if (existing) {
                await httpRequest({
                    url: `/permissions/${existing.id}`,
                    method: 'PATCH'
                }, permissionPayload);
                console.log(`    ✓ Обновлен permission (alt): ${collection}.${action}`);
            } else {
                await httpRequest({
                    url: '/permissions',
                    method: 'POST'
                }, permissionPayload);
                console.log(`    ✓ Создан permission (alt): ${collection}.${action}`);
            }
        } catch (retryError) {
            console.error(`    ✗ Ошибка создания permission ${collection}.${action}:`, error.message);
            console.error(`      Дополнительная ошибка:`, retryError.message);
        }
    }
}

// Настройка прав доступа для ролей
async function setupRolesAndPermissions() {
    console.log('\n🔐 Настройка ролей и прав доступа...');

    // Создаем роли
    const adminRole = await createOrGetRole(
        'Administrator',
        'Полный доступ ко всем коллекциям и функциям системы'
    );
    
    const operatorRole = await createOrGetRole(
        'Operator',
        'Чтение всех данных, ограниченное редактирование'
    );
    
    const viewerRole = await createOrGetRole(
        'Viewer',
        'Только чтение данных без возможности редактирования'
    );

    if (!adminRole || !operatorRole || !viewerRole) {
        console.error('  ✗ Не удалось создать все роли');
        return;
    }

    // Определяем коллекции для настройки (только существующие)
    const allCollections = [
        'users',
        'roles',
        'clients',
        'providers',
        'routes',
        'accounts',
        'transactions',
        'pricing_rules',
        'messages',
        'permissions',
        'role_permissions',
        'api_keys',
        'refresh_tokens',
        'api_key_scopes',
        'client_configs',
        'dlr_receipts'
    ];

    // Фильтруем только существующие коллекции
    const collections = [];
    for (const collection of allCollections) {
        if (await collectionExists(collection)) {
            collections.push(collection);
        } else {
            console.log(`    ⚠ Коллекция ${collection} не найдена, пропускаем`);
        }
    }

    // Полный доступ (all) для Administrator
    console.log('\n  📝 Настройка прав для Administrator...');
    for (const collection of collections) {
        for (const action of ['create', 'read', 'update', 'delete']) {
            await createOrUpdatePermission(adminRole.id, collection, action, {
                permissions: {}, // Пустой объект означает полный доступ
                validation: {},
                presets: null,
                fields: ['*'] // Все поля
            });
        }
    }

    // Настройка прав для Operator
    console.log('\n  📝 Настройка прав для Operator...');
    
    // Operator: чтение для всех коллекций (кроме чувствительных api_keys и refresh_tokens)
    const allReadCollections = ['users', 'clients', 'providers', 'routes', 'messages', 
                                 'accounts', 'transactions', 'pricing_rules', 
                                 'client_configs', 'dlr_receipts'];
    for (const collection of allReadCollections) {
        if (collections.includes(collection)) {
            await createOrUpdatePermission(operatorRole.id, collection, 'read', {
                permissions: {},
                validation: {},
                presets: null,
                fields: ['*']
            });
        }
    }

    // Operator: ограниченное редактирование для clients (только update, без create/delete)
    if (collections.includes('clients')) {
        await createOrUpdatePermission(operatorRole.id, 'clients', 'update', {
            permissions: {},
            validation: {},
            presets: null,
            fields: ['*']
        });
    }

    // Настройка прав для Viewer
    console.log('\n  📝 Настройка прав для Viewer...');
    
    // Viewer: только чтение для всех коллекций (кроме users, roles, permissions, api_keys, refresh_tokens)
    const viewerCollections = ['clients', 'providers', 'routes', 'accounts', 'transactions', 
                                'pricing_rules', 'messages', 'client_configs', 'dlr_receipts'];
    for (const collection of viewerCollections) {
        if (collections.includes(collection)) {
            await createOrUpdatePermission(viewerRole.id, collection, 'read', {
                permissions: {},
                validation: {},
                presets: null,
                fields: ['*']
            });
        }
    }

    // api_keys и refresh_tokens - только для Administrator (полный доступ)
    // Operator и Viewer не должны иметь доступ к этим чувствительным коллекциям
    const sensitiveCollections = ['api_keys', 'refresh_tokens', 'api_key_scopes'];
    for (const collection of sensitiveCollections) {
        if (collections.includes(collection)) {
            // Убеждаемся, что только Administrator имеет доступ
            // (права для Administrator уже установлены выше)
            // Явно запрещаем доступ для Operator и Viewer (не создаем permissions)
            console.log(`    ⚠ Коллекция ${collection} доступна только для Administrator`);
        }
    }

    console.log('\n  ✅ Настройка прав доступа завершена');
    console.log('\n  📊 Итоговые права доступа:');
    console.log('     - Administrator: полный доступ ко всем коллекциям (CRUD)');
    console.log('     - Operator: чтение всех данных, ограниченное редактирование клиентов');
    console.log('     - Viewer: только чтение (кроме users, roles, permissions, api_keys, refresh_tokens)');
    console.log('     - Чувствительные коллекции (api_keys, refresh_tokens): доступны только Administrator');
}

// Проверка доступности Directus
async function checkDirectusHealth() {
    try {
        await httpRequest({
            url: '/server/health',
            method: 'GET'
        });
        return true;
    } catch (error) {
        return false;
    }
}

// Основная функция
async function main() {
    console.log('🚀 Настройка коллекций Directus');
    console.log(`📡 URL: ${directusUrl}`);
    console.log(`👤 Email: ${adminEmail}\n`);

    // Проверка доступности Directus
    console.log('🔍 Проверка доступности Directus...');
    if (!await checkDirectusHealth()) {
        console.error(`✗ Directus недоступен по адресу ${directusUrl}`);
        console.error('  Убедитесь, что Directus запущен и доступен.');
        process.exit(1);
    }
    console.log('✓ Directus доступен\n');

    // Аутентификация
    if (!await authenticate()) {
        console.error('\n❌ Не удалось аутентифицироваться. Проверьте email и пароль администратора.');
        process.exit(1);
    }

    // Настройка коллекций согласно плану
    console.log('\n📦 Начинаем настройку коллекций...\n');
    
    await setupUsersCollection();
    await setupRolesCollection();
    await setupClientsCollection();
    await setupProvidersCollection();
    await setupRoutesCollection();
    await setupAccountsCollection();
    await setupTransactionsCollection();
    await setupPricingRulesCollection();
    await setupApiKeysCollection();
    await setupRefreshTokensCollection();
    await setupMessagesCollection();
    await setupRolePermissionsCollection();
    await setupApiKeyScopesCollection();
    await setupClientConfigsCollection();
    await setupDlrReceiptsCollection();

    console.log('\n✅ Настройка коллекций завершена!');

    // Настройка ролей и прав доступа
    await setupRolesAndPermissions();

    console.log('\n✅ Настройка Directus полностью завершена!');
    console.log('\n📋 Настроенные коллекции:');
    console.log('   - users (скрыт password_hash, настроен role_id)');
    console.log('   - roles (базовая настройка)');
    console.log('   - clients (скрыты secret и api_key)');
    console.log('   - providers (скрыт password, readonly system_id)');
    console.log('   - routes (relationships с providers)');
    console.log('   - accounts (relationship с clients)');
    console.log('   - transactions (relationships с clients и messages)');
    console.log('   - pricing_rules (relationship с clients, опциональный)');
    console.log('   - api_keys (скрыт key_hash, relationship с users)');
    console.log('   - refresh_tokens (скрыт token_hash, relationship с users)');
    console.log('   - messages (relationships с providers, routes, clients)');
    console.log('   - role_permissions (relationships с roles и permissions)');
    console.log('   - api_key_scopes (relationship с api_keys)');
    console.log('   - client_configs (relationship с clients)');
    console.log('   - dlr_receipts (relationships с providers и messages)');
    console.log('\n🔗 Настроенные relationships:');
    console.log('   - users.role_id -> roles');
    console.log('   - routes.provider_id -> providers');
    console.log('   - routes.failover_provider_id -> providers');
    console.log('   - accounts.client_id -> clients');
    console.log('   - transactions.client_id -> clients');
    console.log('   - transactions.message_id -> messages');
    console.log('   - pricing_rules.client_id -> clients');
    console.log('   - api_keys.user_id -> users');
    console.log('   - refresh_tokens.user_id -> users');
    console.log('   - messages.provider_id -> providers');
    console.log('   - messages.route_id -> routes');
    console.log('   - messages.client_id -> clients');
    console.log('   - role_permissions.role_id -> roles');
    console.log('   - role_permissions.permission_id -> permissions');
    console.log('   - api_key_scopes.api_key_id -> api_keys');
    console.log('   - client_configs.client_id -> clients');
    console.log('   - dlr_receipts.provider_id -> providers');
    console.log('   - dlr_receipts.message_id -> messages');
    console.log('\n🔐 Настроенные роли:');
    console.log('   - Administrator (полный доступ ко всем коллекциям)');
    console.log('   - Operator (чтение всех данных, ограниченное редактирование)');
    console.log('   - Viewer (только чтение)');
    console.log('\n💡 Откройте Directus UI по адресу:', directusUrl);
}

// Запуск
main().catch((error) => {
    console.error('\n❌ Критическая ошибка:', error);
    process.exit(1);
});

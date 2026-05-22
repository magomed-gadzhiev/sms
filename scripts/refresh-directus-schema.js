#!/usr/bin/env node

/**
 * Скрипт для обновления схемы Directus через API
 * Автоматически обнаруживает таблицы и создает поля
 */

const http = require('http');
const https = require('https');
const { URL } = require('url');

const DIRECTUS_URL = process.env.DIRECTUS_URL || 'http://localhost:8055';
const ADMIN_EMAIL = process.env.DIRECTUS_ADMIN_EMAIL || 'amfoterius@gmail.com';
const ADMIN_PASSWORD = process.env.DIRECTUS_ADMIN_PASSWORD || '';

let authToken = null;

function httpRequest(options, data = null) {
    return new Promise((resolve, reject) => {
        const url = new URL(options.url || options.path, DIRECTUS_URL);
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

async function authenticate() {
    console.log('🔐 Аутентификация...');
    if (!ADMIN_PASSWORD) {
        console.error('❌ Пароль не указан. Установите DIRECTUS_ADMIN_PASSWORD');
        return false;
    }
    
    try {
        const response = await httpRequest({
            url: '/auth/login',
            method: 'POST'
        }, {
            email: ADMIN_EMAIL,
            password: ADMIN_PASSWORD
        });
        authToken = response.data.data.access_token;
        console.log('✓ Аутентификация успешна');
        return true;
    } catch (error) {
        console.error('✗ Ошибка аутентификации:', error.message);
        return false;
    }
}

async function refreshSchema() {
    console.log('\n🔄 Обновление схемы...');
    
    try {
        // Получаем текущую схему
        const diffResponse = await httpRequest({
            url: '/schema/diff',
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            }
        }, {});
        
        console.log('✓ Схема получена');
        
        // Применяем изменения (если есть)
        if (diffResponse.data.data && diffResponse.data.data.length > 0) {
            console.log(`  Найдено ${diffResponse.data.data.length} изменений`);
            const applyResponse = await httpRequest({
                url: '/schema/apply',
                method: 'POST'
            }, diffResponse.data);
            
            console.log('✓ Схема обновлена');
        } else {
            console.log('  Изменений не найдено');
        }
        
        return true;
    } catch (error) {
        console.error('✗ Ошибка обновления схемы:', error.message);
        return false;
    }
}

async function main() {
    console.log('🚀 Обновление схемы Directus\n');
    
    if (!await authenticate()) {
        process.exit(1);
    }
    
    await refreshSchema();
    
    console.log('\n✅ Готово!');
}

main().catch(console.error);

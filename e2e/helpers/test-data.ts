/**
 * Test data constants.
 * IMPORTANT: Only use invalid/test phone numbers — NEVER real numbers through real providers!
 */
export const TEST_PHONES = {
  INVALID: '+70000000000',
  INVALID_2: '+70000000001',
  INVALID_3: '+70000000002',
  // Short number for validation error
  SHORT: '+7000',
  // Invalid format
  BAD_FORMAT: 'not-a-phone',
};

export const TEST_SMS_TEXT = 'E2E Test Message — автотест';
export const TEST_SMS_TEXT_LONG = 'A'.repeat(200); // Multi-segment message
export const TEST_SMS_TEXT_CYRILLIC = 'Тестовое сообщение для проверки кириллицы и юникода — автотест E2E платформы';

export const CAMPAIGN_NAME_PREFIX = 'E2E-Campaign';

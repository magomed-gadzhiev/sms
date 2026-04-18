import { test, expect } from '@playwright/test';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

/**
 * AC-CW-S-14: API round-trip for use_subscriber_timezone.
 * Verifies D-01 closure — backend must persist and return the flag.
 *
 * This is an API-only test (no UI). It creates a draft campaign, fetches
 * it back, asserts the flag is preserved, then deletes it to keep the
 * sandbox clean.
 *
 * Skip conditions surfaced via explicit errors, not silent skip.
 */
test.describe('Campaign API — use_subscriber_timezone persistence', () => {

  test('AC-CW-S-14: POST → GET round-trip preserves use_subscriber_timezone=true', async ({ request }) => {
    const api = new ApiHelper(request);

    // Pre-seed check: need at least one contact list to bind the campaign to
    const lists = await api.listContactLists();
    const firstList = lists.items?.[0];
    if (!firstList?.id) {
      throw new Error(
        'Seed missing: test user has no contact lists. Cannot run AC-CW-S-14. ' +
        'Create at least one contact list before running this test.',
      );
    }

    // NOTE: scheduled_at deliberately omitted. JSON decode of ISO-string into
    // protobuf Timestamp via stdlib json fails with "Неверный формат запроса"
    // (would need protojson, not stdlib). The draft campaign stays without a
    // schedule, which is valid — this test is about the use_subscriber_timezone
    // flag's persistence, not scheduling semantics.
    let createdId: string | undefined;
    try {
      // Step 1: Create with flag = true
      const created = await api.createCampaign({
        name: `AC-CW-S-14 timezone roundtrip ${Date.now()}`,
        contact_list_id: firstList.id,
        source: 'TestSender',
        send_rate: 100,
        use_subscriber_timezone: true,
      });

      expect(created).toBeDefined();
      expect(created.id, 'Server must return created campaign id').toBeTruthy();
      createdId = created.id;

      // Spec: response should echo the flag
      expect(
        created.use_subscriber_timezone,
        'POST response must echo use_subscriber_timezone=true',
      ).toBe(true);

      // Step 2: Fetch and verify persistence
      const fetched = await api.getCampaign(createdId!);
      expect(
        fetched.use_subscriber_timezone,
        'GET response must include use_subscriber_timezone=true (D-01 closure)',
      ).toBe(true);
    } finally {
      // Step 3: Cleanup — assert DELETE succeeded; a leaked campaign
      // pollutes the shared sandbox
      if (createdId) {
        const delRes = await api.deleteCampaign(createdId);
        expect(
          delRes.ok(),
          `DELETE /campaigns/${createdId} failed (status ${delRes.status()}). Manual cleanup required.`,
        ).toBe(true);
      }
    }
  });

});

import { test, expect } from '@playwright/test';
import { CampaignWizardPage } from '../../pages/CampaignsPage';
import { ApiHelper } from '../../helpers/api';

test.use({ storageState: './auth-state.json' });

/**
 * AC source: docs/ac/campaign-wizard-ac.md (Phase B: Submit + Drafts)
 *
 * Each test creates a real campaign and MUST clean it up. Use afterEach
 * with the shared createdIds list to DELETE via API regardless of test
 * outcome.
 *
 * AC-CW-C-13 uses test.fail() to document drift D-09 (backend can't
 * parse ISO timestamp in scheduled_at). Test stays GREEN while drift
 * exists, flips RED when backend is fixed with protojson.
 */
test.describe('Campaign Wizard — Submit & Drafts Phase B', () => {
  const createdIds: string[] = [];

  test.afterEach(async ({ request }) => {
    if (createdIds.length === 0) return;
    const api = new ApiHelper(request);
    for (const id of createdIds) {
      try {
        const res = await api.deleteCampaign(id);
        if (!res.ok()) {
          console.warn(`Cleanup: DELETE /campaigns/${id} returned ${res.status()}`);
        }
      } catch (err) {
        console.warn(`Cleanup: DELETE /campaigns/${id} threw: ${err}`);
      }
    }
    createdIds.length = 0;
  });

  test('AC-CW-C-12: Submit "Сейчас" creates campaign + calls /launch + redirects', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);

    // Stub /launch so we don't actually trigger SMS delivery in sandbox.
    // Count calls for assertion.
    let launchCalled = 0;
    await page.route('**/portal/v1/campaigns/*/launch', (route) => {
      launchCalled++;
      route.fulfill({ status: 200, body: '{}' });
    });

    // Intercept POST /campaigns to capture created ID
    const createPromise = page.waitForResponse(
      (res) =>
        res.request().method() === 'POST' &&
        new URL(res.url()).pathname === '/portal/v1/campaigns' &&
        res.status() < 300,
      { timeout: 20_000 },
    );

    await wizard.reachStep4Now();
    await wizard.submitButton().click();

    const resp = await createPromise;
    const created = await resp.json();
    // Push BEFORE assertion — if assertion throws, afterEach still cleans up
    if (created.id) createdIds.push(created.id);
    expect(created.id, 'Created campaign must have id').toBeTruthy();

    // Wait a tick for /launch intercept
    await expect.poll(() => launchCalled, { timeout: 5_000 }).toBeGreaterThan(0);

    // Verify redirect to /campaigns/{id} (anchored to id to prevent loose match)
    await expect(page).toHaveURL(new RegExp(`/campaigns/${created.id}(\\?|$)`));
  });

  test('AC-CW-C-13: Submit "Позже" creates scheduled campaign + redirects (D-09 closure)', async ({ page }) => {
    const wizard = new CampaignWizardPage(page);

    // Tomorrow noon — safely in the future
    const tomorrow = new Date(Date.now() + 24 * 60 * 60 * 1000);
    const date = `${tomorrow.getFullYear()}-${String(tomorrow.getMonth() + 1).padStart(2, '0')}-${String(tomorrow.getDate()).padStart(2, '0')}`;

    // Response listener pushes id for cleanup BEFORE assertions, so if any
    // step throws after POST succeeded, afterEach still deletes.
    page.on('response', async (res) => {
      if (
        res.request().method() === 'POST' &&
        new URL(res.url()).pathname === '/portal/v1/campaigns' &&
        res.status() < 300
      ) {
        try {
          const body = await res.json();
          if (body.id) createdIds.push(body.id);
        } catch {}
      }
    });

    // /launch should NOT be called for later mode (campaign is scheduled, not launched)
    let launchCalled = 0;
    await page.route('**/portal/v1/campaigns/*/launch', (route) => {
      launchCalled++;
      route.fulfill({ status: 200, body: '{}' });
    });

    await wizard.reachStep4Later(date, '12:00');
    await wizard.submitButton().click();

    // Redirect to /campaigns/{id} — D-09 was fixed (protojson in handler)
    await expect(page).toHaveURL(/\/campaigns\/[a-f0-9-]{36}(\?|$)/, { timeout: 15_000 });

    // Confirm launch was not triggered for "Позже" mode
    expect(launchCalled, '/launch must NOT be called for later mode').toBe(0);
  });

  test('AC-CW-N-10: Save as draft from cancel dialog → POST + navigate to /campaigns', async ({ page, request }) => {
    // Seed guard: handleSaveDraft falls back to contactLists[0]?.id — if user has none,
    // it shows "Нет контактных баз..." error and skips POST. Test would hang
    // waiting for a request that never fires.
    const api = new ApiHelper(request);
    const lists = await api.listContactLists();
    if (!lists.items?.length) {
      test.skip(true, 'AC-N-10 needs ≥1 contact list (fallback for empty state). Seed one.');
    }

    const wizard = new CampaignWizardPage(page);
    await wizard.goto();
    await wizard.expectActiveStep('Сообщение');

    // Fill minimal Step 1 fields so handleSaveDraft has contactListId fallback
    await wizard.fillMessageText('Draft test message for AC-N-10');

    // Open cancel dialog
    await wizard.cancelButton().click();
    await expect(wizard.cancelDialog()).toBeVisible();

    // Intercept POST /campaigns
    const createPromise = page.waitForResponse(
      (res) =>
        res.request().method() === 'POST' &&
        new URL(res.url()).pathname === '/portal/v1/campaigns' &&
        res.status() < 300,
      { timeout: 15_000 },
    );

    await wizard.dialogSaveDraftButton().click();

    const resp = await createPromise;
    const created = await resp.json();
    // Push BEFORE assertion — cleanup safety
    if (created.id) createdIds.push(created.id);
    expect(created.id, 'Draft must have id').toBeTruthy();
    // Server default is status=draft (CreateCampaign service sets StatusDraft)
    expect(created.status).toBe('draft');

    // Redirect to /campaigns list
    await expect(page).toHaveURL(/\/campaigns(\?|$)/);
  });

  // AC-CW-N-11: blocked by D-10 (?draft= URL handler not implemented in frontend).
  // Using test.fixme — test is skipped with visible reason pointing at drift.
  // When D-10 is fixed, remove the fixme and assert draft-loaded state.
  test.fixme(
    'AC-CW-N-11: Opening /campaigns/new?draft={id} loads draft into wizard (BLOCKED by D-10)',
    async ({ page, request }) => {
      const api = new ApiHelper(request);
      const lists = await api.listContactLists();
      const firstList = lists.items?.[0];
      expect(firstList?.id, 'Seed missing: no contact lists').toBeTruthy();

      const draftName = `AC-N-11 test draft ${Date.now()}`;
      const draft = await api.createCampaign({
        name: draftName,
        contact_list_id: firstList!.id,
        source: 'TestSender',
        send_rate: 100,
      });
      if (draft.id) createdIds.push(draft.id);
      expect(draft.id).toBeTruthy();

      const wizard = new CampaignWizardPage(page);
      await page.goto(`/campaigns/new?draft=${draft.id}`);
      await wizard.expectActiveStep('Сообщение');

      // When D-10 is fixed, the following should pass:
      // - campaignName state loaded (visible on Step 4 after reaching it)
      // - contactListId loaded (Step 2 shows the right list preselected)
      // - templateId / source / sendRate loaded
      // Navigate through wizard to verify each field prefilled.
      // For now (drift live), this assertion will fail because state stays default.
      await wizard.reachStep4Now(); // This would fail at Step 2 (no list preselected)
      await expect(wizard.campaignNameDisplay()).toContainText(draftName);
    },
  );

});

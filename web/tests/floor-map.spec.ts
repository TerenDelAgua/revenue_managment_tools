import { test, expect } from '@playwright/test';

test.describe('Floor Map Builder (FMB-001) Integration & E2E Flow', () => {
	test.beforeEach(async ({ page }) => {
		// Go to map page (operations mode by default)
		await page.goto('/map?mode=ops');
		// Wait for the room grid to render
		await page.waitForSelector('.room-grid');
	});

	test('FT-07 & FT-08: Click Room Token opens Drawer (SlideIn) and closes it (SlideOut)', async ({ page }) => {
		// Find any room token (e.g. room 101 or first matching room-token class)
		const roomToken = page.locator('.room-token').first();
		await expect(roomToken).toBeVisible();

		// Click room token
		await roomToken.click();

		// Drawer panel should slide in (be visible)
		const drawer = page.locator('text=Details');
		await expect(drawer).toBeVisible();

		// Backdrop blur button should be visible
		const backdrop = page.locator('button[aria-label="Close drawer"]');
		await expect(backdrop).toBeVisible();

		// Click backdrop to close
		await backdrop.click();

		// Drawer panel should slide out (be translated offscreen)
		const drawerPanel = page.locator('.fixed.top-0.right-0');
		await expect(drawerPanel).toHaveClass(/translate-x-full/);
	});

	test('FT-09: Click "Block Room" expands Block Form inline', async ({ page }) => {
		// Open first available room token (should be green)
		const availableToken = page.locator('.room-token.bg-\\[\\#16A34A\\]').first();
		if (await availableToken.count() > 0) {
			await availableToken.click();

			// Click "Block Room" button in footer
			const blockButton = page.locator('button', { hasText: 'Block Room' });
			await expect(blockButton).toBeVisible();
			await blockButton.click();

			// Inline Block Form should expand (date input for start and end date should be visible)
			const blockHeader = page.locator('h3', { hasText: 'Block Room' });
			await expect(blockHeader).toBeVisible();

			// Confirm Block button should become visible
			const confirmButton = page.locator('button', { hasText: 'Confirm Block' });
			await expect(confirmButton).toBeVisible();
		}
	});

	test('IT-03 to IT-05: Operational booking and blocking state updates reflect correctly', async ({ page }) => {
		// Find a green (available) room
		const availableToken = page.locator('.room-token.bg-\\[\\#16A34A\\]').first();
		if (await availableToken.count() > 0) {
			const roomNumber = await availableToken.locator('span.font-bold').textContent();

			await availableToken.click();

			// Verify clicking "Block Room" expansions
			const blockBtn = page.locator('button', { hasText: 'Block Room' });
			await blockBtn.click();

			const confirmBtn = page.locator('button', { hasText: 'Confirm Block' });
			await confirmBtn.click();

			// Room drawer should close, and room token status should turn to Blocked (bg-[#44403C])
			const updatedToken = page.locator(`.room-token:has-text("${roomNumber}")`).first();
			await expect(updatedToken).toHaveClass(/bg-\[\#44403C\]/);
		}
	});
});

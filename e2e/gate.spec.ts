import { test, expect } from '@playwright/test';
import { BASE_PATH, GATE_URL, PANEL_URL } from './helpers/constants';

test.describe('unauthenticated gate', () => {
	test('the panel is not listening yet, so nothing below can have reached it', async ({ request }) => {
		await expect(request.get(PANEL_URL + BASE_PATH + '/healthz', { timeout: 2000 })).rejects.toThrow();
	});

	test('an unauthenticated request never reaches the panel', async ({ page, request }) => {
		const response = await page.goto(GATE_URL + '/', { waitUntil: 'load' });

		expect(response?.status()).toBe(401);
		await expect(page.getByRole('button', { name: 'Sign in with Dex' })).toBeVisible();
		await expect(page.locator('body')).not.toContainText('Starterkit');

		const api = await request.post(GATE_URL + BASE_PATH + '/graphql', {
			data: { query: '{ viewer { role } }' },
		});
		expect(api.status()).toBe(401);
	});

	test('the gate sets the browser security headers', async ({ request }) => {
		const headers = (await request.get(GATE_URL + '/')).headers();

		expect(headers['x-frame-options']).toBe('DENY');
		expect(headers['x-content-type-options']).toBe('nosniff');
		expect(headers['referrer-policy']).toBe('same-origin');
		expect(
			headers['content-security-policy'],
			'the gate must not impose the panel policy on oauth2-proxy pages',
		).toBeUndefined();
	});
});

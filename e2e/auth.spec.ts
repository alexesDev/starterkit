import { test, expect } from '@playwright/test';
import { loginViaDex } from './helpers/session';
import { ADMIN, ADMIN_STATE_PATH, BASE_PATH, PANEL_COMMIT } from './helpers/constants';
import { apiContext, gql } from './helpers/api';

test('admin logs in once, lands on the SPA, and is bootstrapped as admin', async ({ page }) => {
	await loginViaDex(page, ADMIN);

	await expect(page.getByText(ADMIN.email).first()).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Users' })).toBeVisible();

	await page.context().storageState({ path: ADMIN_STATE_PATH });

	const api = await apiContext(ADMIN_STATE_PATH);
	try {
		const result = await gql(api, '{ viewer { role user { email } } }');
		expect(result.errors).toBeUndefined();
		expect(result.data.viewer.role).toBe('ADMIN');
		expect(result.data.viewer.user.email).toBe(ADMIN.email);
	} finally {
		await api.dispose();
	}
});

test.describe('replaying the session the login above cached', () => {
	test.use({ storageState: ADMIN_STATE_PATH });

	test('the footer names the revision the panel was built from', async ({ page }) => {
		expect(PANEL_COMMIT, 'the runner did not export PANEL_COMMIT').not.toBe('');

		await page.goto('/');

		await expect(page.getByText(ADMIN.email).first()).toBeVisible();
		await expect(page.getByText(PANEL_COMMIT.slice(0, 7), { exact: true })).toBeVisible();
		await expect(page.getByText('dev', { exact: true })).toHaveCount(0);
	});

	test('build answers the revision the footer shows, and every answer carries it', async () => {
		const api = await apiContext(ADMIN_STATE_PATH);
		try {
			const response = await api.post(`${BASE_PATH}/graphql`, {
				data: { query: '{ admin { build { commit builtAt { unix } } } }' },
			});
			const result = await response.json();

			expect(result.errors).toBeUndefined();
			expect(result.data.admin.build.commit).toBe(PANEL_COMMIT);
			expect(result.data.admin.build.builtAt.unix).toBeGreaterThan(0);
			expect(response.headers()['x-build-commit']).toBe(PANEL_COMMIT);
		} finally {
			await api.dispose();
		}
	});

	test('an answer from another build asks for a reload, and the reload clears it', async ({ page }) => {
		const graphql = `**${BASE_PATH}/graphql`;
		await page.route(graphql, async (route) => {
			const response = await route.fetch();
			await route.fulfill({ response, headers: { ...response.headers(), 'x-build-commit': 'f'.repeat(40) } });
		});

		await page.goto('/');
		await expect(page.getByText('A new version is deployed')).toBeVisible();

		await page.unroute(graphql);
		await page.getByRole('button', { name: 'Reload' }).click();

		await expect(page.getByText(ADMIN.email).first()).toBeVisible();
		await expect(page.getByText('A new version is deployed')).toHaveCount(0);
	});
});

test('the panel serves its own CSP and the gate does not serve one', async ({ browser }) => {
	const anonymous = await browser.newContext();
	try {
		const gate = await anonymous.newPage().then((page) => page.goto('/'));
		expect(
			gate?.headers()['content-security-policy'],
			'the gate imposed a policy on an oauth2-proxy page',
		).toBeUndefined();
	} finally {
		await anonymous.close();
	}

	const api = await apiContext(ADMIN_STATE_PATH);
	try {
		const panel = await api.get('/');
		const policy = panel.headers()['content-security-policy'];

		expect(policy, 'the panel served no CSP').toContain("frame-ancestors 'none'");
		expect(policy, 'inline script would be allowed').toContain("default-src 'self'");
		expect(panel.headers()['x-frame-options']).toBe('DENY');
	} finally {
		await api.dispose();
	}
});

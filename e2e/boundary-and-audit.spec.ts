import { test, expect } from '@playwright/test';
import { loginViaDex } from './helpers/session';
import { ADMIN, ADMIN_STATE_PATH, GATE_URL, USER, USER_STATE_PATH } from './helpers/constants';
import { apiContext, fetchWholeAuditLog, gql } from './helpers/api';
import { fetchStarterkitMetrics, waitForMetrics } from './helpers/metrics';

const DENIED_USERS_QUERY = 'starterkit_admin_denied_total{operation="denied:query:users"}';

test('user logs in, is not an admin, and the admin namespace refuses them', async ({ page }) => {
	const adminApi = await apiContext(ADMIN_STATE_PATH);

	try {
		const deniedBefore = (await fetchStarterkitMetrics(adminApi))[DENIED_USERS_QUERY] ?? 0;

		await loginViaDex(page, USER);

		await expect(page.getByText(USER.email).first()).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Users' })).not.toBeVisible();

		await page.context().storageState({ path: USER_STATE_PATH });

		const userApi = await apiContext(USER_STATE_PATH);
		try {
			const viewer = await gql(userApi, '{ viewer { role } }');
			expect(viewer.data.viewer.role).toBe('USER');

			const denied = await gql(userApi, '{ admin { users { totalCount } } }');
			expect(denied.data?.admin).toBeUndefined();
			expect(denied.errors?.length).toBeGreaterThan(0);
			expect(denied.errors?.[0]?.message).toBe('unauthorized');
		} finally {
			await userApi.dispose();
		}

		await page.goto(GATE_URL + '/');
		await expect(page.getByText(USER.email).first()).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Users' })).not.toBeVisible();

		const metrics = await waitForMetrics(
			adminApi,
			(snapshot) => (snapshot[DENIED_USERS_QUERY] ?? 0) > deniedBefore,
		);
		expect(metrics[DENIED_USERS_QUERY], 'the refused crossing was not counted').toBe(deniedBefore + 1);

		const entries = await fetchWholeAuditLog(adminApi);

		const adminSignIn = entries.find((e) => e.action === 'sign_in' && e.email === ADMIN.email);
		const userSignIn = entries.find((e) => e.action === 'sign_in' && e.email === USER.email);
		expect(adminSignIn, 'admin sign_in row').toBeTruthy();
		expect(userSignIn, 'user sign_in row').toBeTruthy();
		expect(adminSignIn!.detail, 'a sign_in row carries no variables').toBe('');

		const deniedRows = entries.filter((e) => e.action.startsWith('denied:') && e.email === USER.email);
		expect(deniedRows.length, 'denied: rows for the non-admin user').toBeGreaterThan(0);
		expect(deniedRows.map((e) => e.action)).toContain('denied:query:users');
	} finally {
		await adminApi.dispose();
	}
});

test.describe('rendered audit log', () => {
	test.use({ storageState: ADMIN_STATE_PATH });

	test('shows the refused crossing', async ({ page }) => {
		await page.goto(GATE_URL + '/');
		await page.getByText('Audit log', { exact: true }).click();

		await expect(page.getByText('denied:query:users').first()).toBeVisible();
		await expect(page.getByText(USER.email).first()).toBeVisible();
	});
});

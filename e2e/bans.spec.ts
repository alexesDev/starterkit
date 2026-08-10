import { test, expect } from '@playwright/test';
import { ADMIN_STATE_PATH, BASE_PATH, GATE_URL, USER, USER_STATE_PATH } from './helpers/constants';
import { apiContext, findUserIdByEmail, gql } from './helpers/api';
import { waitForMetrics } from './helpers/metrics';

const BAN_NOTICES = 'starterkit_notifications_total{kind="user_banned"}';

const BAN_USER = `mutation($input: BanUserInput!) {
	admin { payload: banUser(input: $input) {
		__typename
		... on BanUserPayload { userId }
		... on ErrorPayload { message }
	} }
}`;

const UNBAN_USER = `mutation($input: UnbanUserInput!) {
	admin { payload: unbanUser(input: $input) {
		__typename
		... on UnbanUserPayload { userId }
		... on ErrorPayload { message }
	} }
}`;

async function asUser(query: string, variables?: Record<string, unknown>) {
	const api = await apiContext(USER_STATE_PATH);
	try {
		return await gql(api, query, variables);
	} finally {
		await api.dispose();
	}
}

test('banning revokes an existing, still-valid session; unbanning restores it', async ({ browser }) => {
	const adminApi = await apiContext(ADMIN_STATE_PATH);

	try {
		const userId = await findUserIdByEmail(adminApi, USER.email);

		const before = await asUser('{ viewer { user { email } } }');
		expect(before.errors).toBeUndefined();
		expect(before.data.viewer.user.email).toBe(USER.email);

		const ban = await gql(adminApi, BAN_USER, {
			input: { userId, reason: 'e2e: testing ban revocation' },
		});
		expect(ban.errors, JSON.stringify(ban.errors)).toBeUndefined();
		expect(ban.data.admin.payload.__typename).toBe('BanUserPayload');

		const bannedContext = await browser.newContext({ storageState: USER_STATE_PATH });
		const page = await bannedContext.newPage();
		const response = await page.goto(GATE_URL + '/', { waitUntil: 'load' });
		expect(response?.status()).toBe(403);
		await bannedContext.close();

		const duringBanApi = await apiContext(USER_STATE_PATH);
		const afterBan = await duringBanApi.post(BASE_PATH + '/graphql', {
			data: { query: '{ viewer { role } }' },
		});
		expect(afterBan.status()).toBe(403);
		expect(await afterBan.text()).toContain('access revoked');
		await duringBanApi.dispose();

		const metrics = await waitForMetrics(adminApi, (snapshot) => (snapshot[BAN_NOTICES] ?? 0) >= 1);
		expect(metrics[BAN_NOTICES], 'the ban notice job did not run').toBe(1);

		const unban = await gql(adminApi, UNBAN_USER, { input: { userId } });
		expect(unban.errors, JSON.stringify(unban.errors)).toBeUndefined();
		expect(unban.data.admin.payload.__typename).toBe('UnbanUserPayload');

		const after = await asUser('{ viewer { user { email } } }');
		expect(after.errors).toBeUndefined();
		expect(after.data.viewer.user.email).toBe(USER.email);
	} finally {
		await adminApi.dispose();
	}
});

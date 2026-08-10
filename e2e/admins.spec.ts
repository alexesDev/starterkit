import { test, expect } from '@playwright/test';
import { ADMIN, ADMIN_STATE_PATH, USER, USER_STATE_PATH } from './helpers/constants';
import { apiContext, findUserIdByEmail, gql } from './helpers/api';

async function asUser(query: string) {
	const api = await apiContext(USER_STATE_PATH);
	try {
		return await gql(api, query);
	} finally {
		await api.dispose();
	}
}

const VIEWER_ROLE = '{ viewer { role } }';
const ADMIN_USERS = '{ admin { users { totalCount } } }';
const ADMIN_LIST = '{ admin { admins { totalCount nodes { email } } } }';

const MAKE_ADMIN = `mutation($input: MakeAdminInput!) {
	admin { payload: makeAdmin(input: $input) {
		__typename
		... on MakeAdminPayload { userId }
		... on ErrorPayload { message }
	} }
}`;

const REMOVE_ADMIN = `mutation($input: RemoveAdminInput!) {
	admin { payload: removeAdmin(input: $input) {
		__typename
		... on RemoveAdminPayload { userId }
		... on ErrorPayload { message }
	} }
}`;

test('granting and revoking admin take effect on the next request, without a new login', async () => {
	const adminApi = await apiContext(ADMIN_STATE_PATH);

	try {
		const userId = await findUserIdByEmail(adminApi, USER.email);
		const adminUserId = await findUserIdByEmail(adminApi, ADMIN.email);

		expect((await asUser(VIEWER_ROLE)).data.viewer.role).toBe('USER');

		const before = await asUser(ADMIN_USERS);
		expect(before.errors?.[0]?.message, 'the non-admin reached AdminQuery').toBe('unauthorized');

		const granted = await gql(adminApi, MAKE_ADMIN, { input: { userId } });
		expect(granted.errors, JSON.stringify(granted.errors)).toBeUndefined();
		expect(granted.data.admin.payload.__typename).toBe('MakeAdminPayload');

		expect((await asUser(VIEWER_ROLE)).data.viewer.role).toBe('ADMIN');

		const promoted = await asUser(ADMIN_USERS);
		expect(promoted.errors, JSON.stringify(promoted.errors)).toBeUndefined();
		expect(promoted.data.admin.users.totalCount).toBeGreaterThan(0);

		const admins = await gql(adminApi, ADMIN_LIST);
		expect(admins.data.admin.admins.totalCount).toBe(2);
		expect(admins.data.admin.admins.nodes.map((a: { email: string }) => a.email).sort()).toEqual(
			[ADMIN.email, USER.email].sort(),
		);

		const revoked = await gql(adminApi, REMOVE_ADMIN, { input: { userId } });
		expect(revoked.errors, JSON.stringify(revoked.errors)).toBeUndefined();
		expect(revoked.data.admin.payload.__typename).toBe('RemoveAdminPayload');

		expect((await asUser(VIEWER_ROLE)).data.viewer.role).toBe('USER');

		const demoted = await asUser(ADMIN_USERS);
		expect(demoted.errors?.[0]?.message, 'a revoked admin still reached AdminQuery').toBe('unauthorized');

		const remaining = await gql(adminApi, ADMIN_LIST);
		expect(remaining.data.admin.admins.totalCount, 'the admins table emptied').toBe(1);
		expect(remaining.data.admin.admins.nodes.map((a: { email: string }) => a.email)).toEqual([ADMIN.email]);

		const self = await gql(adminApi, REMOVE_ADMIN, { input: { userId: adminUserId } });
		expect(self.errors, JSON.stringify(self.errors)).toBeUndefined();
		expect(self.data.admin.payload.__typename).toBe('ErrorPayload');
		expect(self.data.admin.payload.message).toContain('your own');

		const stillThere = await gql(adminApi, ADMIN_LIST);
		expect(stillThere.data.admin.admins.totalCount, 'the last admin was removable').toBe(1);
	} finally {
		await adminApi.dispose();
	}
});

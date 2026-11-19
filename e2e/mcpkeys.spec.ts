import { test, expect, type APIRequestContext } from '@playwright/test';
import { ADMIN, ADMIN_STATE_PATH, BASE_PATH, GATE_URL } from './helpers/constants';
import { apiContext, fetchWholeAuditLog } from './helpers/api';

const MCP_URL = GATE_URL + BASE_PATH + '/mcp';

const SHOTS = 'e2e/.artifacts/screenshots';

async function callTool(key: string, name: string, args: Record<string, unknown>) {
	const res = await fetch(MCP_URL, {
		method: 'POST',
		headers: {
			Authorization: `Bearer ${key}`,
			'Content-Type': 'application/json',
			Accept: 'application/json, text/event-stream',
		},
		body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'tools/call', params: { name, arguments: args } }),
	});

	const body = res.ok ? await res.json() : null;

	return { status: res.status, result: body?.result };
}

async function keyIdByName(adminApi: APIRequestContext, name: string): Promise<number> {
	const res = await adminApi.post(BASE_PATH + '/graphql', {
		data: { query: '{ admin { mcpKeys { nodes { id name } } } }' },
	});
	const body = await res.json();
	const found = body.data.admin.mcpKeys.nodes.find((node: { name: string }) => node.name === name);
	expect(found, `no key named ${name}`).toBeTruthy();

	return found.id;
}

test('an admin creates a key, an MCP client uses it through the gate, then it is revoked and deleted', async ({ browser }) => {
	const context = await browser.newContext({ storageState: ADMIN_STATE_PATH });
	const page = await context.newPage();
	const adminApi = await apiContext(ADMIN_STATE_PATH);

	try {
		await page.goto(GATE_URL + '/#!nav=mcpkeys');
		await expect(page.getByText('No keys yet')).toBeVisible();
		await page.screenshot({ path: `${SHOTS}/mcpkeys-1-empty.png` });

		await page.getByTitle('New key').click();
		await expect(page.getByRole('heading', { name: 'New MCP key' })).toBeVisible();

		await page.getByRole('button', { name: 'Create' }).click();
		await expect(page.getByText('cannot be blank')).toBeVisible();

		await page.getByPlaceholder('What will use this key').fill('e2e client');
		await page.getByRole('button', { name: 'Create' }).click();

		const secret = page.getByText(/^mcp_[A-Za-z0-9_-]{43}$/);
		await expect(secret).toBeVisible();
		const key = (await secret.textContent())!.trim();
		await page.screenshot({ path: `${SHOTS}/mcpkeys-2-created.png` });
		await expect(page.getByRole('button', { name: 'Create', exact: true })).toHaveCount(0);
		await expect(page.getByRole('button', { name: 'Create another' })).toBeVisible();

		const keyId = await keyIdByName(adminApi, 'e2e client');

		const viewer = await callTool(key, 'graphql_query', { query: '{ viewer { role user { email } } }' });
		expect(viewer.status, 'the MCP path did not pass the gate').toBe(200);
		expect(viewer.result.isError).toBeFalsy();
		expect(JSON.parse(viewer.result.content[0].text)).toEqual({
			data: { viewer: { role: 'ADMIN', user: { email: ADMIN.email } } },
		});

		const sdl = await callTool(key, 'graphql_introspection', { types: ['AdminQuery'] });
		expect(sdl.result.content[0].text).toContain('mcpKeys(first: Int64, after: Cursor): MCPKeysConnection!');

		await page.getByRole('link', { name: 'Open the key' }).click();
		await expect(page.getByRole('heading', { name: 'e2e client' })).toBeVisible();
		await page.screenshot({ path: `${SHOTS}/mcpkeys-3-item-used.png` });

		await page.getByRole('button', { name: 'Revoke' }).click();
		await expect(page.getByText('Revoked', { exact: true })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Revoke' })).toHaveCount(0);
		await page.screenshot({ path: `${SHOTS}/mcpkeys-4-revoked.png` });

		const refused = await callTool(key, 'graphql_query', { query: '{ viewer { role } }' });
		expect(refused.status, 'a revoked key was still accepted').toBe(401);

		await page.getByRole('button', { name: 'Delete' }).click();
		await expect(page.getByRole('heading', { name: 'e2e client' })).toHaveCount(0);
		await expect(page.getByText('No keys yet')).toBeVisible();
		await page.screenshot({ path: `${SHOTS}/mcpkeys-5-deleted.png` });

		const audit = await fetchWholeAuditLog(adminApi);
		const actions = audit.map((entry) => entry.action);
		expect(actions).toContain('mutation:createMCPKey');
		expect(actions).toContain('mcp:graphql_query');
		expect(actions).toContain('mutation:revokeMCPKey');
		expect(actions).toContain('denied:mcp');
		expect(actions).toContain('mutation:deleteMCPKey');
		expect(audit.some((entry) => entry.detail.includes(`"mcpKeyId":${keyId}`))).toBe(true);
		expect(JSON.stringify(audit), 'the raw key reached the audit log').not.toContain(key);
	} finally {
		await adminApi.dispose();
		await context.close();
	}
});

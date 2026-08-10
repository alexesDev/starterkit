import { test, expect, request as pwRequest } from '@playwright/test';
import { ADMIN_STATE_PATH, BASE_PATH, PANEL_URL } from './helpers/constants';
import { apiContext } from './helpers/api';
import { panelExitCode, signalPanel } from './helpers/panel';

test('SIGTERM drains: readyz fails first, traffic keeps flowing, then the process exits 0', async () => {
	const plain = await pwRequest.newContext({ baseURL: PANEL_URL });
	const asAdmin = await apiContext(ADMIN_STATE_PATH);

	try {
		expect((await plain.get(BASE_PATH + '/healthz')).status()).toBe(200);
		expect((await plain.get(BASE_PATH + '/readyz')).status()).toBe(200);

		await signalPanel('TERM');

		const [health, ready, traffic] = await Promise.all([
			plain.get(BASE_PATH + '/healthz'),
			plain.get(BASE_PATH + '/readyz'),
			asAdmin.post(BASE_PATH + '/graphql', { data: { query: '{ viewer { role } }' } }),
		]);

		expect(health.status(), '/healthz during the delay window').toBe(200);
		expect(ready.status(), '/readyz during the delay window').toBe(503);
		expect(traffic.ok(), 'an in-flight-equivalent request during the drain').toBeTruthy();

		const trafficBody = await traffic.json();
		expect(trafficBody.data.viewer.role).toBe('ADMIN');

		expect(await panelExitCode(30_000), 'panel exit status after SIGTERM').toBe(0);
		await expect(plain.get(BASE_PATH + '/healthz', { timeout: 2000 })).rejects.toThrow();
	} finally {
		await plain.dispose();
		await asAdmin.dispose();
	}
});

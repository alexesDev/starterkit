import { test, expect } from '@playwright/test';
import fs from 'node:fs';
import path from 'node:path';
import { ADMIN_STATE_PATH } from './helpers/constants';
import { apiContext, fetchWholeAuditLog } from './helpers/api';
import { waitForMetrics } from './helpers/metrics';

const SNAPSHOT_PATH = path.join(__dirname, '__snapshots__', 'audit-and-metrics.snapshot.json');

const DETERMINISTIC_METRIC_KEYS = [
	'starterkit_sign_ins_total',
	'starterkit_users_banned_total',
	'starterkit_admin_changed_total',
	'starterkit_notifications_total',
	'starterkit_table_rows',
];

const AUDIT_ROWS_KEY = 'starterkit_table_rows{table="audit_log"}';
const MIGRATIONS_ROWS_KEY = 'starterkit_table_rows{table="schema_migrations"}';

test('audit log + metrics match the committed behavioural snapshot', async ({}, testInfo) => {
	const adminApi = await apiContext(ADMIN_STATE_PATH);

	try {
		const entries = await fetchWholeAuditLog(adminApi);
		expect(entries.length).toBeGreaterThan(0);

		const seconds = entries.map((e) => e.createdAt.unix);
		expect(
			seconds.every((unix, i) => i === 0 || seconds[i - 1] <= unix),
			'the audit log did not come back oldest first',
		).toBe(true);

		const metrics = await waitForMetrics(adminApi, (snap) => snap[AUDIT_ROWS_KEY] === entries.length);
		expect(metrics[AUDIT_ROWS_KEY], 'gauge did not converge in time').toBe(entries.length);
		expect(metrics[MIGRATIONS_ROWS_KEY], 'migrations table is empty').toBeGreaterThan(0);

		const normalizedAuditLog = entries.map((e) => ({ action: e.action, email: e.email, detail: e.detail }));

		const normalizedMetrics: Record<string, number> = {};
		for (const key of Object.keys(metrics).sort()) {
			if (key === MIGRATIONS_ROWS_KEY) {
				continue;
			}

			if (DETERMINISTIC_METRIC_KEYS.some((prefix) => key === prefix || key.startsWith(prefix + '{'))) {
				normalizedMetrics[key] = metrics[key];
			}
		}

		const actual = { auditLog: normalizedAuditLog, metrics: normalizedMetrics };

		if (process.env.UPDATE_SNAPSHOTS === '1') {
			fs.mkdirSync(path.dirname(SNAPSHOT_PATH), { recursive: true });
			fs.writeFileSync(SNAPSHOT_PATH, JSON.stringify(actual, null, '\t') + '\n');
			testInfo.annotations.push({ type: 'snapshot-updated', description: SNAPSHOT_PATH });
			return;
		}

		if (!fs.existsSync(SNAPSHOT_PATH)) {
			throw new Error(
				`no committed snapshot at ${SNAPSHOT_PATH}. Run the e2e suite with UPDATE_SNAPSHOTS=1 to create it.`,
			);
		}

		const expected = JSON.parse(fs.readFileSync(SNAPSHOT_PATH, 'utf8'));
		expect(actual).toEqual(expected);
	} finally {
		await adminApi.dispose();
	}
});

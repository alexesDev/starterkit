import type { APIRequestContext } from '@playwright/test';
import { METRICS_URL } from './constants';

export type MetricsSnapshot = Record<string, number>;

const SERIES_PREFIX = 'starterkit_';

export function parseStarterkitMetrics(text: string): MetricsSnapshot {
	const out: MetricsSnapshot = {};

	for (const line of text.split('\n')) {
		if (!line || line.startsWith('#') || !line.startsWith(SERIES_PREFIX)) {
			continue;
		}

		const sep = line.lastIndexOf(' ');
		if (sep === -1) {
			continue;
		}

		const value = Number(line.slice(sep + 1));
		if (Number.isNaN(value)) {
			continue;
		}

		out[line.slice(0, sep)] = value;
	}

	return out;
}

export async function fetchStarterkitMetrics(api: APIRequestContext): Promise<MetricsSnapshot> {
	const res = await api.get(METRICS_URL + '/metrics');
	return parseStarterkitMetrics(await res.text());
}

export async function waitForMetrics(
	api: APIRequestContext,
	check: (snapshot: MetricsSnapshot) => boolean,
	{ timeoutMs = 8000, intervalMs = 250 }: { timeoutMs?: number; intervalMs?: number } = {},
): Promise<MetricsSnapshot> {
	const deadline = Date.now() + timeoutMs;
	let last: MetricsSnapshot = {};

	do {
		last = await fetchStarterkitMetrics(api);
		if (check(last)) {
			return last;
		}

		await new Promise((resolve) => setTimeout(resolve, intervalMs));
	} while (Date.now() < deadline);

	return last;
}

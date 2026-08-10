import { execFile } from 'node:child_process';
import { promisify } from 'node:util';
import { BASE_PATH, PANEL_URL } from './constants';

const run = promisify(execFile);

export const PANEL_CONTAINER = process.env.PANEL_CONTAINER || 'starterkit-e2e-panel';

const PANEL_HEALTHZ = `${PANEL_URL}${BASE_PATH}/healthz`;

async function docker(args: string[], timeoutMs = 30_000): Promise<string> {
	const { stdout } = await run('docker', args, { timeout: timeoutMs });
	return stdout.trim();
}

export async function stopPanel(): Promise<void> {
	await docker(['stop', PANEL_CONTAINER]);
}

export async function startPanel(): Promise<void> {
	await docker(['start', PANEL_CONTAINER]);
	await waitUntilServing(30_000);
}

export async function signalPanel(signal: string): Promise<void> {
	await docker(['kill', `--signal=${signal}`, PANEL_CONTAINER]);
}

export async function panelExitCode(timeoutMs = 30_000): Promise<number> {
	return Number(await docker(['wait', PANEL_CONTAINER], timeoutMs));
}

async function isServing(): Promise<boolean> {
	try {
		return (await fetch(PANEL_HEALTHZ)).ok;
	} catch {
		return false;
	}
}

async function waitUntilServing(timeoutMs: number): Promise<void> {
	const deadline = Date.now() + timeoutMs;

	while (Date.now() < deadline) {
		if (await isServing()) {
			return;
		}

		await new Promise((resolve) => setTimeout(resolve, 200));
	}

	throw new Error(`panel did not answer ${PANEL_HEALTHZ} within ${timeoutMs}ms`);
}

import type { Page } from '@playwright/test';
import { GATE_URL } from './constants';

async function submitDexLogin(page: Page, identity: { email: string; password: string }) {
	await page.locator('#login').fill(identity.email);
	await page.locator('#password').fill(identity.password);
	await Promise.all([page.waitForLoadState('load'), page.locator('#submit-login').click()]);
}

export async function loginViaDex(page: Page, identity: { email: string; password: string }) {
	await page.goto(GATE_URL + '/');
	await page.getByRole('button', { name: 'Sign in with Dex' }).click();
	await submitDexLogin(page, identity);
	await page.waitForLoadState('networkidle');
}

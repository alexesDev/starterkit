import { test } from '@playwright/test';
import { stopPanel } from './helpers/panel';

test('stop the panel container', async () => {
	await stopPanel();
});

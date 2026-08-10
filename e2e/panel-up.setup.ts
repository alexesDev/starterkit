import { test } from '@playwright/test';
import { startPanel } from './helpers/panel';

test('start the panel container and wait until it serves', async () => {
	await startPanel();
});

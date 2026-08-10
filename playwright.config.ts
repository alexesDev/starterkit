import { defineConfig, devices } from '@playwright/test';

function spec(name: string, dependsOn: string) {
	return {
		name,
		testMatch: `**/${name}.spec.ts`,
		dependencies: [dependsOn],
		use: { ...devices['Desktop Chrome'] },
	};
}

export default defineConfig({
	testDir: './e2e',
	outputDir: './e2e/.artifacts/test-results',

	fullyParallel: false,
	workers: 1,
	forbidOnly: !!process.env.CI,
	retries: 0,

	reporter: process.env.CI
		? 'github'
		: [['list'], ['html', { outputFolder: './e2e/.artifacts/html-report', open: 'never' }]],

	use: {
		baseURL: process.env.APP_URL || 'http://panel.e2e.sk.localbox',
		trace: 'retain-on-failure',
		screenshot: 'only-on-failure',
	},

	projects: [
		{ name: 'panel-down', testMatch: '**/panel-down.setup.ts', timeout: 60_000 },
		spec('gate', 'panel-down'),
		{
			name: 'panel-up',
			testMatch: '**/panel-up.setup.ts',
			dependencies: ['gate'],
			timeout: 60_000,
		},

		spec('auth', 'panel-up'),
		spec('boundary-and-audit', 'auth'),
		spec('bans', 'boundary-and-audit'),
		spec('admins', 'bans'),
		spec('snapshot', 'admins'),
		{ ...spec('drain', 'snapshot'), timeout: 60_000 },
	],
});

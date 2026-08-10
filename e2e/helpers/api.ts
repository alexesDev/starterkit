import { request as pwRequest, type APIRequestContext } from '@playwright/test';
import { BASE_PATH, GATE_URL } from './constants';

export async function apiContext(storageStatePath: string): Promise<APIRequestContext> {
	return pwRequest.newContext({ baseURL: GATE_URL, storageState: storageStatePath });
}

export interface GqlResult<T = any> {
	status: number;
	data?: T;
	errors?: { message: string; path?: (string | number)[] }[];
}

export async function gql<T = any>(
	api: APIRequestContext,
	query: string,
	variables?: Record<string, unknown>,
): Promise<GqlResult<T>> {
	const res = await api.post(BASE_PATH + '/graphql', { data: { query, variables } });
	const body = await res.json().catch(() => ({}));
	return { status: res.status(), data: body.data, errors: body.errors };
}

export interface AuditLogEntry {
	id: number;
	userId: number | null;
	email: string;
	action: string;
	detail: string;
	ip: string;
	createdAt: { unix: number };
}

const AUDIT_LOG_QUERY = `
	query E2eAuditLog($limit: Int64!) {
		admin {
			auditLog(limit: $limit) {
				totalCount
				nodes { id userId email action detail ip createdAt { unix } }
			}
		}
	}
`;

export async function fetchWholeAuditLog(adminApi: APIRequestContext): Promise<AuditLogEntry[]> {
	const result = await gql(adminApi, AUDIT_LOG_QUERY, { limit: 500 });
	if (result.errors?.length) {
		throw new Error(`fetchWholeAuditLog: ${result.errors.map((e) => e.message).join('; ')}`);
	}

	const nodes: AuditLogEntry[] = result.data.admin.auditLog.nodes;
	return [...nodes].reverse();
}

export async function findUserIdByEmail(adminApi: APIRequestContext, email: string): Promise<number> {
	const entries = await fetchWholeAuditLog(adminApi);
	const signIn = entries.find((e) => e.action === 'sign_in' && e.email === email);
	if (!signIn || signIn.userId == null) {
		throw new Error(`no sign_in audit row for ${email} — has that identity logged in yet?`);
	}

	return signIn.userId;
}

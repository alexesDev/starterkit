namespace $ {

	export type $starterkit_gql_opts = {
		revalidate?: boolean
	}

	export class $starterkit_gql_error extends Error {
		constructor(message: string, public detail?: { message: string, path?: string[] }[]) {
			for (const err of detail ?? []) message += `. ${err.message}`
			super(message)
		}
	}

	class marker_holder extends $mol_object2 {
		@$mol_mem
		value(next?: number) {
			return next ?? 0
		}
	}

	const marker = new marker_holder()

	export function $starterkit_gql_marker_watch() {
		marker.value()
	}

	export function $starterkit_gql_marker_bump() {
		marker.value(marker.value() + 1)
	}

	export function $starterkit_gql_post(path: string, query: string, variables?: object, opts?: $starterkit_gql_opts): unknown {
		const mutation = /^\s*mutation\b/.test(query)
		const revalidate = opts?.revalidate !== false

		if (!mutation && revalidate) $starterkit_gql_marker_watch()

		const res = $mol_fetch.json(path, {
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ query, variables }),
		}) as { data?: unknown, errors?: { message: string, path?: string[] }[] }

		if (res.errors) throw new $starterkit_gql_error('GraphQL Error', res.errors)

		if (mutation && revalidate) $starterkit_gql_marker_bump()

		return res.data
	}

	export function $starterkit_gql_request(query: string, variables?: object, opts?: $starterkit_gql_opts): unknown {
		const settings = (globalThis as { starterkit_settings?: { graphqlUrl?: string } }).starterkit_settings
		return $starterkit_gql_post(settings?.graphqlUrl ?? '/_system/starterkit/graphql', query, variables, opts)
	}

	export function $starterkit_gql_make_map<Row extends { id: PropertyKey }>(rows: readonly Row[]) {
		const keys = rows.map(row => row.id.toString())
		const map = new Map(rows.map(row => [row.id.toString(), row]))

		return {
			keys: () => keys,
			size: () => keys.length,
			get: (key: string) => {
				const row = map.get(key.replace(/^key/, ''))
				if (!row) throw new Error(`Key ${key} not found`)
				return row
			},
			mapKeys<Val>(make: (key: string) => Val): Record<string, Val> {
				const out: Record<string, Val> = {}
				for (const key of keys) out[`key${key}`] = make(key)
				return out
			},
		}
	}

	export type $starterkit_gql_ref<Frag> = Frag extends { ' \u0024fragmentName'?: infer Name extends string }
		? { ' \u0024fragmentRefs'?: { [Key in Name]: Frag } }
		: never

}

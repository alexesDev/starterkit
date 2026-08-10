namespace $ {
	export type $starterkit_gql_failure = {
		message: string
		byFields: readonly { name: string; value: string }[]
	}

	export function $starterkit_gql_field_error(error: $starterkit_gql_failure | null, field: string) {
		return error?.byFields.find(item => item.name === field)?.value ?? ''
	}

	export function $starterkit_gql_form_error(error: $starterkit_gql_failure | null) {
		if (!error) return ''

		return error.byFields.length ? '' : error.message
	}
}

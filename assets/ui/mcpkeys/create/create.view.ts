namespace $.$$ {
	export class $starterkit_mcpkeys_create extends $.$starterkit_mcpkeys_create {
		@$mol_mem
		override message(next?: string): string {
			return next ?? ''
		}

		@$mol_mem
		error(next?: $starterkit_gql_failure | null): $starterkit_gql_failure | null {
			return next ?? null
		}

		@$mol_mem
		issued(next?: { mcpKeyId: number, key: string } | null) {
			return next ?? null
		}

		override name_bid() {
			return $starterkit_gql_field_error(this.error(), 'name')
		}

		override secret() {
			return this.issued()?.key ?? ''
		}

		override open_arg() {
			return { mcpkey: 'key' + this.issued()?.mcpKeyId }
		}

		override form_rows() {
			if (this.issued()) return [this.Issued()]

			return [
				this.Name_field(),
				...this.message() ? [this.Status()] : [],
			]
		}

		override actions() {
			return this.issued() ? [this.Again()] : [this.Create()]
		}

		again(next?: null) {
			if (next === undefined) return null

			this.issued(null)
			this.error(null)
			this.message('')

			return null
		}

		create(next?: null) {
			if (next === undefined) return null

			this.error(null)
			this.issued(null)

			const result = $starterkit_mcpkeys_create_create({
				input: { name: this.name().trim() },
			}).admin.createMCPKey

			if (result.__typename === 'ErrorPayload') {
				this.error(result)
				this.message($starterkit_gql_form_error(result))

				return null
			}

			this.issued(result)
			this.name('')
			this.message('')

			return null
		}
	}
}

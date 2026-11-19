namespace $.$$ {
	type Keys = ReturnType<typeof $starterkit_mcpkeys_catalog_list>['admin']['mcpKeys']
	type Key = Keys['nodes'][number]

	export class $starterkit_mcpkeys_catalog extends $.$starterkit_mcpkeys_catalog {
		@$mol_mem
		paging() {
			const view = this

			return new (class extends $starterkit_paging<Key> {
				override page_size() {
					return view.page_size()
				}

				override fetch(after: string | null) {
					return $starterkit_mcpkeys_catalog_list({
						first: this.page_size(),
						after,
					}).admin.mcpKeys
				}
			})()
		}

		@$mol_mem
		data() {
			return $starterkit_gql_make_map(this.paging().items())
		}

		@$mol_mem
		override spreads(): Record<string, $mol_view> {
			return {
				add: this.Create_page(),
				...this.data().mapKeys(key => this.Show_page(key)),
			}
		}

		override Spread(id: string) {
			return id === 'add' ? this.Create_page() : this.Show_page(id)
		}

		@$mol_mem
		override spread_ids_filtered() {
			return super.spread_ids_filtered().filter(id => id !== 'add')
		}

		@$mol_mem
		override menu_body() {
			return this.paging().has_next() ? [...super.menu_body(), this.More()] : super.menu_body()
		}

		more(next?: null) {
			if (next === undefined) return null

			this.paging().more()

			return null
		}

		override total_text() {
			const total = (this.paging().page('') as Keys).totalCount

			return total === 1 ? '1 key' : `${total} keys`
		}

		row(id: string) {
			return this.data().get(id)
		}

		override show_key_id(id: string) {
			return Number(id.replace(/^key/, ''))
		}

		override row_name(id: string) {
			return this.row(id).name
		}

		override row_owner(id: string) {
			return this.row(id).ownerEmail
		}

		override row_status(id: string) {
			return this.row(id).revokedAt ? 'revoked' : 'active'
		}

		override row_created(id: string) {
			return this.row(id).createdAt.formatted
		}
	}
}

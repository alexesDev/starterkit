namespace $.$$ {
	type User = ReturnType<typeof $starterkit_users_catalog_list>['admin']['users']['nodes'][number]

	export class $starterkit_users_catalog extends $.$starterkit_users_catalog {
		@$mol_mem
		paging() {
			const view = this

			return new (class extends $starterkit_paging<User> {
				override page_size() {
					return view.page_size()
				}

				override fetch(after: string | null) {
					return $starterkit_users_catalog_list({
						first: this.page_size(),
						after,
					}).admin.users
				}
			})()
		}

		@$mol_mem
		data() {
			return $starterkit_gql_make_map(this.paging().items())
		}

		@$mol_mem
		override spreads(): Record<string, $mol_view> {
			return this.data().mapKeys(key => this.Show_page(key))
		}

		override Spread(id: string) {
			return this.Show_page(id)
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

		row(id: string) {
			return this.data().get(id)
		}

		override show_user_id(id: string) {
			return Number(id.replace(/^key/, ''))
		}

		override row_id_string(id: string) {
			return String(this.row(id).id)
		}

		override row_email(id: string) {
			return this.row(id).email
		}

		override row_last_sign_in(id: string) {
			return this.row(id).lastSignInAt?.formatted ?? ''
		}

		override row_tone(id: string) {
			const row = this.row(id)
			if (row.banReason) return 'bad'

			return row.isAdmin ? 'ok' : 'neutral'
		}

		override row_status(id: string) {
			const row = this.row(id)
			if (row.banReason) return 'banned'

			return row.isAdmin ? 'admin' : 'user'
		}
	}
}

namespace $.$$ {
	export class $starterkit_users_catalog extends $.$starterkit_users_catalog {
		@$mol_mem
		data() {
			return $starterkit_gql_make_map($starterkit_users_catalog_list().admin.users.nodes)
		}

		@$mol_mem
		override spreads(): Record<string, $mol_view> {
			return this.data().mapKeys(key => this.Show_page(key))
		}

		row(id: string) {
			return this.data().get(id)
		}

		override row_id(id: string) {
			return this.row(id).id
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

		override row_status(id: string) {
			const row = this.row(id)
			if (row.banReason) return 'banned'

			return row.isAdmin ? 'admin' : 'user'
		}
	}
}

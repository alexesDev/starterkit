namespace $.$$ {
	export class $starterkit_users_show extends $.$starterkit_users_show {
		@$mol_mem
		user() {
			const user = $starterkit_users_show_data({ id: this.user_id() }).admin.user
			if (!user) throw new Error('User not found')

			return user
		}

		override email() {
			return this.user().email
		}

		name() {
			return this.user().name
		}

		created() {
			return this.user().createdAt.formatted
		}

		override last_sign_in() {
			return this.user().lastSignInAt?.formatted ?? ''
		}

		id_string() {
			return String(this.user().id)
		}

		ban_reason() {
			return this.user().banReason
		}

		role() {
			return this.admin() ? 'admin' : 'user'
		}

		banned() {
			return Boolean(this.ban_reason())
		}

		admin() {
			return this.user().isAdmin
		}

		override ban_enabled() {
			return !this.banned()
		}

		override unban_enabled() {
			return this.banned()
		}

		override make_admin_enabled() {
			return !this.admin() && !this.banned()
		}

		override remove_admin_enabled() {
			return this.admin()
		}

		@$mol_mem
		override reason(next?: string): string {
			return next ?? this.ban_reason()
		}

		@$mol_mem
		override message(next?: string): string {
			if (next !== undefined) return next

			return this.banned() ? 'Banned: ' + this.ban_reason() : ''
		}

		@$mol_mem
		error(next?: $starterkit_gql_failure | null): $starterkit_gql_failure | null {
			return next ?? null
		}

		reason_bid() {
			return $starterkit_gql_field_error(this.error(), 'reason')
		}

		ban(next?: null) {
			if (next === undefined) return null

			this.error(null)

			const result = $starterkit_users_show_ban({
				input: { userId: this.user_id(), reason: this.reason().trim() },
			}).admin.banUser

			if (result.__typename === 'ErrorPayload') {
				this.error(result)
				this.message($starterkit_gql_form_error(result))

				return null
			}

			this.message('Banned')

			return null
		}

		unban(next?: null) {
			if (next === undefined) return null

			$starterkit_users_show_unban({ input: { userId: this.user_id() } })
			this.message('Unbanned')

			return null
		}

		make_admin(next?: null) {
			if (next === undefined) return null

			const result = $starterkit_users_show_make_admin({
				input: { userId: this.user_id() },
			}).admin.makeAdmin

			this.message(result.__typename === 'ErrorPayload' ? result.message : 'Admin granted')

			return null
		}

		remove_admin(next?: null) {
			if (next === undefined) return null

			const result = $starterkit_users_show_remove_admin({
				input: { userId: this.user_id() },
			}).admin.removeAdmin

			this.message(result.__typename === 'ErrorPayload' ? result.message : 'Admin removed')

			return null
		}
	}
}

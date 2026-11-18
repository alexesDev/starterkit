namespace $.$$ {
	export class $starterkit_app extends $.$starterkit_app {
		@$mol_mem
		viewer() {
			return $starterkit_app_viewer().viewer
		}

		user_email() {
			return this.viewer().user?.email ?? ''
		}

		is_admin() {
			return this.viewer().role === 'ADMIN'
		}

		override signature_content() {
			return this.is_admin() ? [this.User(), this.Build()] : [this.User()]
		}

		override menu_foot() {
			return this.$.$starterkit_buildinfo.stale() ? [this.Foot(), this.Skew()] : [this.Foot()]
		}

		@$mol_mem
		override Spread_default() {
			return this.is_admin() ? this.Users() : null
		}

		sign_out(next?: null) {
			if (next === undefined) return null

			const payload = $starterkit_app_signout().signOut
			this.$.$mol_dom_context.location.href = payload.redirectUrl

			return null
		}
	}
}

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

		@$mol_mem
		git_commit() {
			return this.is_admin() ? $starterkit_app_build().admin.buildGitCommit : ''
		}

		override signature_content() {
			return this.git_commit() ? [this.User(), this.Build()] : [this.User()]
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

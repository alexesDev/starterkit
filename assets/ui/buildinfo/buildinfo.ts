namespace $ {
	export class $starterkit_buildinfo extends $mol_object2 {
		static header = 'X-Build-Commit'

		static bundle() {
			return this.$.$starterkit_buildinfo_commit
		}

		@$mol_mem
		static served(next?: string) {
			return next ?? ''
		}

		static notice(response: $mol_fetch_response) {
			const commit = response.headers().get(this.header)
			if (commit) this.served(commit)
		}

		@$mol_mem
		static stale() {
			const bundle = this.bundle()
			const served = this.served()

			return bundle !== '' && served !== '' && served !== bundle
		}
	}
}

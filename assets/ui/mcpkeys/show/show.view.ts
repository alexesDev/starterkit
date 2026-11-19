namespace $.$$ {
	export class $starterkit_mcpkeys_show extends $.$starterkit_mcpkeys_show {
		@$mol_mem
		key() {
			const key = $starterkit_mcpkeys_show_data({ id: this.mcp_key_id() }).admin.mcpKey
			if (!key) throw new Error('Key not found')

			return key
		}

		override name() {
			return this.key().name
		}

		id_string() {
			return String(this.key().id)
		}

		owner() {
			return this.key().ownerEmail
		}

		status() {
			return this.key().revokedAt ? 'revoked' : 'active'
		}

		created() {
			return this.key().createdAt.formatted
		}

		override last_used() {
			return this.key().lastUsedAt?.formatted ?? ''
		}

		override revoked() {
			return this.key().revokedAt?.formatted ?? ''
		}

		override actions() {
			return this.key().revokedAt ? [this.Delete()] : [this.Revoke(), this.Delete()]
		}

		@$mol_mem
		override message(next?: string): string {
			return next ?? ''
		}

		revoke(next?: null) {
			if (next === undefined) return null

			const result = $starterkit_mcpkeys_show_revoke({
				input: { mcpKeyId: this.mcp_key_id() },
			}).admin.revokeMCPKey

			this.message(result.__typename === 'ErrorPayload' ? result.message : 'Revoked')

			return null
		}

		delete(next?: null) {
			if (next === undefined) return null

			const result = $starterkit_mcpkeys_show_delete({
				input: { mcpKeyId: this.mcp_key_id() },
			}).admin.deleteMCPKey

			if (result.__typename === 'ErrorPayload') {
				this.message(result.message)

				return null
			}

			this.$.$mol_state_arg.value('mcpkey', null)

			return null
		}
	}
}

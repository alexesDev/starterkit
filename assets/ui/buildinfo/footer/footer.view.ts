namespace $.$$ {
	export class $starterkit_buildinfo_footer extends $.$starterkit_buildinfo_footer {
		@$mol_mem
		build() {
			return $starterkit_buildinfo_footer_data().admin.build
		}

		commit() {
			return this.build().commit ?? ''
		}

		override commit_url() {
			return this.build().commitUrl ?? ''
		}

		override built() {
			return this.build().builtAt?.formatted ?? ''
		}

		@$mol_mem
		copied(next?: boolean) {
			return next ?? false
		}

		override commit_label() {
			return this.copied() ? 'copied' : this.commit().slice(0, 7)
		}

		override content() {
			return [
				this.commit() ? this.Commit() : this.Dev(),
				...this.commit_url() ? [this.Open()] : [],
				...this.built() ? [this.Built()] : [],
			]
		}

		override copy(next?: null) {
			if (next === undefined) return null

			this.write_clipboard(this.commit())
			this.copied(true)
			new this.$.$mol_after_timeout(1500, () => this.copied(false))

			return null
		}

		write_clipboard(text: string) {
			const dom = this.$.$mol_dom_context
			const clipboard = dom.navigator.clipboard

			if (dom.isSecureContext && clipboard) {
				clipboard.writeText(text).catch(() => this.select_and_copy(text))
				return
			}

			this.select_and_copy(text)
		}

		select_and_copy(text: string) {
			const doc = this.$.$mol_dom_context.document
			const field = doc.createElement('textarea')

			field.value = text
			field.setAttribute('readonly', '')
			field.style.position = 'fixed'
			field.style.opacity = '0'

			doc.body.appendChild(field)
			field.select()
			doc.execCommand('copy')
			field.remove()
		}
	}
}

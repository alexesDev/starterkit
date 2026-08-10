namespace $.$$ {
	type Entry = ReturnType<typeof $starterkit_auditlog_list>['admin']['auditLog']['nodes'][number]

	export class $starterkit_auditlog extends $.$starterkit_auditlog {
		@$mol_mem
		paging() {
			const view = this

			return new (class extends $starterkit_paging<Entry> {
				override page_size() {
					return view.page_size()
				}

				override fetch(before: number | null) {
					return $starterkit_auditlog_list({
						limit: this.page_size(),
						before,
					}).admin.auditLog
				}
			})()
		}

		entries() {
			return this.paging().items()
		}

		total() {
			return this.paging().total()
		}

		@$mol_mem
		override body(): readonly $mol_view[] {
			return this.paging().has_next() ? [this.List(), this.More()] : [this.List()]
		}

		rows() {
			return this.entries().map((_, index) => this.Row(index))
		}

		entry(index: number) {
			return this.entries()[index]
		}

		override row_created(index: number) {
			return this.entry(index).createdAt.formatted
		}

		override row_action(index: number) {
			return this.entry(index).action
		}

		override row_email(index: number) {
			return this.entry(index).email
		}

		override row_detail(index: number) {
			const entry = this.entry(index)
			return [entry.detail, entry.ip].filter(Boolean).join(' - ')
		}

		more(next?: null) {
			if (next === undefined) return null

			this.paging().more()

			return null
		}
	}
}

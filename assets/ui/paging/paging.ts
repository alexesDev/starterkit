namespace $ {
	export type $starterkit_paging_page<Node> = {
		pageInfo: {
			hasNextPage: boolean
			endCursor?: string | null
		}
		nodes: readonly Node[]
	}

	export class $starterkit_paging<Node> extends $mol_object {
		page_size() {
			return 50
		}

		fetch(after: string | null): $starterkit_paging_page<Node> {
			throw new Error('$starterkit_paging.fetch is not implemented')
		}

		@$mol_mem
		cursors(next?: readonly string[]) {
			return next ?? ['']
		}

		@$mol_mem_key
		page(after: string) {
			return this.fetch(after || null)
		}

		@$mol_mem
		pages() {
			return this.cursors().map(cursor => this.page(cursor))
		}

		@$mol_mem
		items(): readonly Node[] {
			return this.pages().flatMap(page => page.nodes as Node[])
		}

		last() {
			const pages = this.pages()
			return pages[pages.length - 1]
		}

		has_next() {
			return this.last()?.pageInfo.hasNextPage ?? false
		}

		more() {
			const end = this.last()?.pageInfo.endCursor
			if (!this.has_next() || !end) return

			this.cursors([...this.cursors(), end])
		}
	}
}

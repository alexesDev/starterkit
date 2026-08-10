namespace $ {
	export type $starterkit_paging_page<Node> = {
		totalCount: number
		pageInfo: {
			hasNextPage: boolean
			endCursor?: number | null
		}
		nodes: readonly Node[]
	}

	export class $starterkit_paging<Node> extends $mol_object {
		page_size() {
			return 50
		}

		fetch(before: number | null): $starterkit_paging_page<Node> {
			throw new Error('$starterkit_paging.fetch is not implemented')
		}

		@$mol_mem
		cursors(next?: number[]) {
			return next ?? [0]
		}

		@$mol_mem_key
		page(before: number) {
			return this.fetch(before || null)
		}

		@$mol_mem
		pages() {
			return this.cursors().map(cursor => this.page(cursor))
		}

		@$mol_mem
		items(): readonly Node[] {
			return this.pages().flatMap(page => page.nodes as Node[])
		}

		total() {
			return this.pages()[0]?.totalCount ?? 0
		}

		last() {
			const pages = this.pages()
			return pages[pages.length - 1]
		}

		has_next() {
			return this.last()?.pageInfo.hasNextPage ?? false
		}

		more() {
			const last = this.last()
			if (!last?.pageInfo.hasNextPage || last.pageInfo.endCursor == null) return

			this.cursors([...this.cursors(), last.pageInfo.endCursor])
		}

		reset() {
			this.cursors([0])
		}
	}
}

namespace $ {
	function build(bundle: string) {
		const $ = $mol_ambient({ $starterkit_buildinfo_commit: bundle })

		return class extends $starterkit_buildinfo {
			static override $ = $
		}
	}

	$mol_test({
		'a response from another commit makes the bundle stale'() {
			const info = build('1111111')
			info.served('2222222')

			$mol_assert_equal(info.stale(), true)
		},

		'a response from the same commit does not'() {
			const info = build('1111111')
			info.served('1111111')

			$mol_assert_equal(info.stale(), false)
		},

		'a dev bundle is never stale'() {
			const info = build('')
			info.served('2222222')

			$mol_assert_equal(info.stale(), false)
		},

		'no header yet is not stale'() {
			const info = build('1111111')

			$mol_assert_equal(info.stale(), false)
		},
	})
}

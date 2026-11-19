namespace $.$$ {
	const { rem } = $mol_style_unit

	$mol_style_define($starterkit_buildinfo_skew, {
		position: 'fixed',
		zIndex: 1,
		bottom: rem(1),
		left: '50%',
		transform: 'translateX(-50%)',
		alignItems: 'center',
		gap: rem(0.5),
		padding: {
			left: rem(1),
			right: rem(0.25),
		},
		border: {
			radius: $mol_gap.round,
		},
		background: {
			color: $mol_theme.card,
		},
		boxShadow: `0 0.25rem 1rem ${$mol_theme.line}`,
	})
}

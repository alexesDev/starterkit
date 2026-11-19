namespace $.$$ {
	const { rem } = $mol_style_unit

	$mol_style_define($starterkit_buildinfo_footer, {
		alignItems: 'center',
		flexWrap: 'wrap',
		gap: rem(0.25),

		Commit: {
			padding: {
				left: 0,
			},
			font: {
				family: 'monospace',
			},
		},

		Dev: {
			minHeight: 'auto',
			margin: {
				top: rem(0.25),
			},
			padding: {
				top: 0,
				bottom: 0,
				left: rem(0.5),
				right: rem(0.5),
			},
			border: {
				radius: rem(1),
			},
			background: {
				color: $mol_theme.special,
			},
			color: $mol_theme.back,
		},

		Built: {
			color: $mol_theme.shade,
		},
	})
}

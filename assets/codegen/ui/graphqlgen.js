// GraphQL codegen for the frontend: .graphql files co-located with $mol
// components → per-file <name>.graphql.ts typed wrappers in `namespace $`.
// Ported from the graphqlgen.js of github.com/trip2g/mol_graphql.
//
// Schema comes from the checked-in SDL — no introspection, no running server.
//
// The shared schema types (enums, inputs, Scalars, Maybe/Exact helpers) are
// declared once in assets/ui/gql/schema.graphql.ts and referenced by every
// generated file, since they all land in the same `namespace $`.

module.exports = {
	generates: {
		'./': {
			schema: 'internal/graph/schema.graphqls',
			preset: require('./preset.js'),
			plugins: [], // per-file plugin chains are built by the preset
			documents: ['assets/ui/**/*.graphql'],
			config: {
				// repo-relative assets/ui/users/list.graphql is workspace
				// starterkit/users/list = $starterkit_users_list
				molPackage: 'starterkit',
				molRoot: 'assets/ui',
				// operation names are auto-derived from the file path; type names
				// keep GraphQL casing as-is (matches molplugin's resultType)
				namingConvention: 'keep',
				// Relay-style fragment masking: a spread field is an opaque ref
				inlineFragmentTypes: 'mask',
				// wire shapes of the custom scalars: Int64 is a JSON number,
				// Time an RFC3339 string
				scalars: { Int64: 'number', Time: 'string' },
				// every mutation refetches every query - the reset marker convention
				revalidation: 'all',
				molRuntime: '$starterkit_gql',
				molSchemaTypes: 'assets/ui/gql/schema.graphql.ts',
			},
		},
	},
}

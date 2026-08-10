// graphql-codegen plugin: emits the $mol-style typed seam for one .graphql file.
// Ported from github.com/trip2g/mol_graphql (codegen/molplugin.js).
//
// For an operation file (query/mutation):
//   export function $starterkit_users_list(opts?): starterkit_users_listQuery {
//       return $starterkit_gql_request(`<operation + spread fragments>`, undefined, opts) as ...
//   }
// The result/variables types are baked in by the generator. Fragment spreads
// are merged into the sent document at codegen time (transitively, by name).
//
// Operations are auto-named from the file location: the canonical name is the
// wrapper symbol without the `$` (users/list.graphql -> starterkit_users_list).
// Whatever the author wrote - `query { ... }` (anonymous) or any name - is
// overridden to the canonical BEFORE the stock plugin runs, so the wrapper
// symbol, the result type and the name the server sees all match the file path
// 1:1. Fragments are NOT renamed - they are spread by name (Relay model) - but
// a non-canonical fragment name gets a non-blocking warning.
//
// For a fragment file:
//   export type $starterkit_user_card_user = starterkit_user_card_userFragment
//   export type $starterkit_user_card_user_ref  - bare-name opaque ref, usable in .view.tree
//   export function $starterkit_user_card_user_unmask(ref)  - identity, nullability-preserving
//   export function $starterkit_user_card_user_unmask_not_null(ref)  - throws on null ref
//
// It wraps the stock `typescript` / `typescript-operations` plugins and escapes
// every `$` they (or the embedded GraphQL) produce as `\u0024`: the $mol builder
// scans sources for $-prefixed names to build its module graph, and would
// otherwise treat GraphQL variables ($id) and masking keys (' $fragmentRefs')
// as module references. TS/JS treat $ as the same character, so types and
// runtime strings are unchanged. $-names WE emit ($starterkit_..., doc-comment
// deps) stay unescaped on purpose - those are real module references.
//
// `config.revalidation` picks the invalidation metadata baked into the wrappers:
//   'all' (default)  pass caller opts through - every query subscribes to the
//                    universal marker, every mutation bumps it
//   'disable'        never subscribe, never bump
// `{ revalidate: false }` at the call site opts out in either mode.

const typescriptPlugin = require('@graphql-codegen/typescript')
const operationsPlugin = require('@graphql-codegen/typescript-operations')
const { print } = require('graphql')

const SUFFIX = { query: 'Query', mutation: 'Mutation', subscription: 'Subscription' }
const REVALIDATION_MODES = ['all', 'disable']

module.exports = {
	plugin: async (schema, documents, config, info) => {
		if (config.molMode === 'schema') {
			return escapeDollars(flatten(await typescriptPlugin.plugin(schema, documents, stockConfig(config), info)))
		}

		const runtime = config.molRuntime || '$graphql'
		const fragments = config.molFragments || {}
		const symbol = config.molSymbol
		const revalidation = config.revalidation || 'all'
		if (!REVALIDATION_MODES.includes(revalidation)) {
			throw new Error(`Unknown revalidation mode "${revalidation}": use ${REVALIDATION_MODES.map(mode => `'${mode}'`).join(' | ')}`)
		}

		// rename operations to the canonical before the stock plugin runs, so the
		// generated result/variables types are derived from the canonical name too
		const named = documents.map(doc => renameOperations(doc, symbol.slice(1)))

		const types = await operationsPlugin.plugin(schema, named, stockConfig(config), info)
		const lines = [escapeDollars(flatten(types))]

		for (const doc of named) {
			for (const def of doc.document.definitions) {
				if (def.kind === 'OperationDefinition') {
					lines.push(...operationCode(def, doc, { symbol, runtime, fragments, revalidation }))
				} else if (def.kind === 'FragmentDefinition') {
					lines.push(...fragmentCode(def, { symbol, runtime, location: doc.location }))
				}
			}
		}

		return lines.join('\n')
	},
}

// strip our own config keys before handing config to the stock plugins
function stockConfig(config) {
	const {
		molMode, molRuntime, molFragments, molSymbol, molSchemaTypes,
		molPackage, molRoot, revalidation,
		...rest
	} = config
	return rest
}

function flatten(out) {
	if (typeof out === 'string') return out
	return [...(out.prepend || []), out.content || '', ...(out.append || [])].join('\n')
}

function escapeDollars(code) {
	return code.replace(/\$/g, '\\u0024')
}

// AST-level rename of every operation definition to the path-derived canonical;
// fragment definitions pass through untouched
function renameOperations(doc, name) {
	return {
		...doc,
		document: {
			...doc.document,
			definitions: doc.document.definitions.map(def =>
				def.kind === 'OperationDefinition'
					? { ...def, name: { kind: 'Name', value: name } }
					: def,
			),
		},
	}
}

function operationCode(def, doc, { symbol, runtime, fragments, revalidation }) {
	if (def.operation === 'subscription') {
		throw new Error(`${doc.location}: subscriptions are not supported`)
	}

	const opName = def.name.value
	const resultType = opName + SUFFIX[def.operation]
	const varsType = resultType + 'Variables'

	// merge spread fragments (transitive closure over the global registry);
	// the operation is printed from its (renamed) AST, not the raw source
	const closure = fragmentClosure(def, fragments, doc.location)
	const merged = [print(def).trim(), ...closure.map(name => fragments[name].source)].join('\n\n')

	// typed variables parameter, plus an optional per-call opts (the invalidation
	// escape hatch: `{ revalidate: false }` opts this call out of refetch-on-mutation)
	const varDefs = def.variableDefinitions || []
	const required = varDefs.some(v => v.type.kind === 'NonNullType' && !v.defaultValue)
	const varsParam = varDefs.length === 0 ? '' : `variables${required ? '' : '?'}: ${varsType}`
	const optsParam = 'opts?: { revalidate?: boolean }'
	const param = [varsParam, optsParam].filter(Boolean).join(', ')
	const varsArg = varDefs.length === 0 ? 'undefined' : 'variables'
	const optsArg = revalidation === 'disable' ? '{ revalidate: false, ...opts }' : 'opts'

	const code = ['']
	if (closure.length) {
		// doc-comment so the $mol builder records dependencies on the fragment
		// modules (it scans $-names, skipping non-doc comments)
		code.push(`/** Spreads fragments: ${closure.map(name => fragments[name].symbol).join(', ')} */`)
	}
	code.push(
		`export function ${symbol}(${param}): ${resultType} {`,
		`\treturn ${runtime}_request(\`${escapeTemplate(merged)}\`, ${varsArg}, ${optsArg}) as ${resultType}`,
		`}`,
	)
	return code
}

function fragmentCode(def, { symbol, runtime, location }) {
	// symmetric to operation auto-naming, but advisory only: renaming a fragment
	// would break its spread sites, so the declared name stays authoritative
	const canonical = symbol.slice(1)
	if (def.name.value !== canonical) {
		console.warn(
			`${location}: fragment "${def.name.value}" does not match its file location - ` +
			`consider renaming it (and its spreads) to "${canonical}"`,
		)
	}

	const fragType = def.name.value + 'Fragment'
	const refType = `${runtime}_ref<${fragType}>`
	return [
		``,
		`/** Data declared by fragment \`${def.name.value}\` - spread it anywhere as \`...${def.name.value}\`. */`,
		`export type ${symbol} = ${fragType}`,
		``,
		`/** Opaque ref to this fragment - a bare name usable where generics don't fit, e.g. a .view.tree property: \`<prop> null ${symbol}_ref\`. */`,
		`export type ${symbol}_ref = ${refType}`,
		``,
		`/**`,
		` * Identity accessor: turns an opaque fragment ref (masked parent data) into the typed fragment fields.`,
		` * Preserves the ref's nullability, so the compiler forces the null branch when the ref may be null.`,
		` */`,
		`export function ${symbol}_unmask(ref: ${refType}): ${fragType}`,
		`export function ${symbol}_unmask(ref: ${refType} | null | undefined): ${fragType} | null | undefined`,
		`export function ${symbol}_unmask(ref: ${refType} | null | undefined): ${fragType} | null | undefined {`,
		`\treturn ref as ${fragType} | null | undefined`,
		`}`,
		``,
		`/** Checked accessor: unmask that throws on a null/undefined ref. */`,
		`export function ${symbol}_unmask_not_null(ref: ${refType} | null | undefined): ${fragType} {`,
		`\tif (ref == null) throw new Error('null fragment ref for ${def.name.value}')`,
		`\treturn ref as ${fragType}`,
		`}`,
	]
}

function fragmentClosure(def, fragments, location) {
	const seen = new Set()
	const queue = [...collectSpreads(def)]
	while (queue.length) {
		const name = queue.shift()
		if (seen.has(name)) continue
		const frag = fragments[name]
		if (!frag) throw new Error(`${location}: fragment "${name}" is spread but not defined in any .graphql file`)
		seen.add(name)
		queue.push(...frag.spreads)
	}
	return [...seen]
}

function collectSpreads(node, acc = new Set()) {
	if (node.kind === 'FragmentSpread') acc.add(node.name.value)
	for (const key of ['selectionSet', 'selections']) {
		const sub = node[key]
		if (Array.isArray(sub)) sub.forEach(child => collectSpreads(child, acc))
		else if (sub && typeof sub === 'object') collectSpreads(sub, acc)
	}
	return acc
}

// escape for embedding in a template literal; all `$` become `\u0024` so the
// $mol dependency scanner ignores GraphQL variables (also neutralizes `${`)
function escapeTemplate(str) {
	return str.replace(/\\/g, '\\\\').replace(/`/g, '\\`').replace(/\$/g, '\\u0024')
}

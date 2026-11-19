# graphql_introspection

Looks the schema up without loading all of it. One mode per call:

- `query="ban user"` with `query_op` (`and` by default, `or`, `exact`): the matching SDL lines with line numbers, each under the type it belongs to and the path from a root that reaches it (`## BanUserInput (reached by Mutation.admin.banUser(input))`); `before` and `after` set how many lines of context surround a hit (2 by default, at most 10).
- `types=["AdminQuery", "BanUserInput"]`: those types' full definitions, at most five. A root type's fields come the same way: `types=["Query"]`.
- no arguments: how to use this tool, then the entry points: the root types and the namespaces under them, each with its field count, no fields.

Search first, with the words of the task. Then fetch the types the hits name. Never try to pull the whole schema: it is long, and you need a few types of it.

The schema you see is the part this key may use.

# graphql_query

Reads data. Arguments: `query` (the document), `variables` (a JSON object, when the document declares any), `operationName` (when it holds several). A document holding a mutation is refused: use graphql_mutation.

```graphql
query { admin { users(first: 20) { totalCount nodes { id email } pageInfo { hasNextPage endCursor } } } }
```

The answer is the GraphQL response as JSON. Select only the fields you need: a long answer is cut, and the last line then says so. Every field also counts against the limit in errors.

A refused document comes back with the schema of the types it named and the nearest field names by spelling: read them before trying again.

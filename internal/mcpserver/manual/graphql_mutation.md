# graphql_mutation

Changes data. Same arguments as graphql_query; a document holding a query is refused. Some mutations delete or revoke: check the input before you call.

A mutation takes one argument, `input`, and answers with a union of its payload and `ErrorPayload`. Always select `__typename` and both branches:

```graphql
mutation ($input: BanUserInput!) {
  admin {
    banUser(input: $input) {
      __typename
      ... on BanUserPayload { userId }
      ... on ErrorPayload { message byFields { name value } }
    }
  }
}
```

An `ErrorPayload` is a refusal, not a failure: `message` says why, `byFields` names the input field each problem belongs to. Fix the input and call again; do not retry unchanged.

Every mutation is written to the audit log under the key owner's name. See errors for what a failure looks like.

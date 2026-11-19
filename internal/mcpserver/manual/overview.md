# Overview

This server is the panel's GraphQL API: the same schema, resolvers and permissions the web panel uses. You act as the person who created this key, and their rights are checked again on every call.

- `Query.viewer` says who you are and with which role.
- Everything an administrator may read is under `Query.admin`, everything they may change under `Mutation.admin`. No `admin` field in the schema means this key has no admin access.
- A list takes `first` and `after` and answers `totalCount`, `nodes` and `pageInfo { hasNextPage endCursor }`; pass `endCursor` back as `after` for the next page. `totalCount` alone does not load the rows.
- A time is a `Timestamp`: select `unix`, or `formatted(layout: "2006-01-02 15:04")` with a Go reference layout; the default is RFC 3339.
- Ids are `Int64`, sent as JSON numbers.

Next: graphql_introspection to find fields, graphql_query to read, graphql_mutation to change something.

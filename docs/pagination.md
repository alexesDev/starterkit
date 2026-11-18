# Lists and pagination

Every list the API returns has one of two shapes. The first is the default and
covers almost everything; the second is for one situation and says which.

This is the rule the kit's lists are being moved to: today `auditLog` pages
with a bare `Int64` cursor and counts on every request, and `users` and
`admins` return everything at once. Until they are moved, this document is
the target, not a description of them.

## The default: a connection with an opaque cursor

```graphql
scalar Cursor

type PageInfo {
  hasNextPage: Boolean!
  endCursor: Cursor
}

type AuditLogConnection {
  totalCount: Int64! @goField(forceResolver: true)
  nodes: [AuditLogEntry!]!
  pageInfo: PageInfo!
}

type AdminQuery {
  auditLog(first: Int64, after: Cursor): AuditLogConnection!
}
```

- **Keyset by id, newest first.** A page is `where id < :after order by id desc
  limit :first + 1`. The extra row says whether there is a next page and is
  cut off before the answer. Rows inserted while someone pages do not shift
  the pages they have not reached yet, so there are no duplicates and no skips,
  and the query costs the same on the hundredth page as on the first. Offset is
  not used anywhere: `offset 5000` reads and throws away five thousand rows,
  and every insert moves every later page by one.
- **The cursor is opaque.** `Cursor` is a string the client hands back and
  never reads. Today it carries the last row's id; a list that one day sorts by
  another column carries `(column, id)` in the same string, and no client
  changes. Only the server encodes and decodes it.
- **`first` is optional and clamped.** Left out, it takes the configured
  default; the schema carries no default of its own, so there is one. A request
  for more than the configured maximum gets the maximum, not an error.
- **`totalCount` is computed only when asked.** It is a forced resolver, so
  `count(*)` runs for the page that shows a total and not for the one that
  only scrolls.
- **One helper does the mechanics.** The clamp, the `limit + 1` probe, cutting
  the extra row and filling `pageInfo` live in one small package the cases
  call; a list case passes its rows and the id of a row, and states nothing
  about paging itself.
- **Sorting is by id only** until a screen needs another order. When one does,
  the cursor grows a column, the query's `where` becomes a row comparison on
  `(column, id)`, and the index follows it.

In $mol the list is one shared model with a "show more" control fed by
`endCursor` ([frontend-mol.md](frontend-mol.md),
[mol-interface-design.md](mol-interface-design.md)).

## The second shape: ids first, rows by id

For a list whose *search* is expensive and whose screen scrolls through
thousands of results — full-text search, a filter across joins — the work can
be split in two, as described in
[«Популярные антипаттерны: паджинация»](https://habr.com/ru/articles/413133/):

```graphql
type AdminQuery {
  searchMessages(query: String!): [ID!]!
  messagesByIds(ids: [ID!]!): [MessageOrMissing!]!
}

union MessageOrMissing = Message | Missing
```

1. **The search returns only ids, with a hard cap** (say 1000). A thousand ids
   are about ten kilobytes, and the expensive query runs once, not once per
   page.
2. **The client fetches rows by id in batches**, only the ones on screen —
   which is what a lazily rendered `$mol_list` asks for anyway — and never
   fetches a row twice.
3. **A row can be gone.** An id from the snapshot whose row was deleted since
   comes back as `Missing`, and the screen drops it.

What it buys: the list is a snapshot, so scrolling it never shows a duplicate
or skips a row however the data changes underneath; the search is paid for
once; rows already loaded can be refreshed in the background.

What it costs, and why it is not the default:

- **Two round trips** before the first row shows.
- **The snapshot goes stale.** New rows appear only when the search runs
  again; deleted ones turn into `Missing`.
- **The cap is a wall.** Nothing past the thousandth result can be reached;
  the screen has to say so and ask for a narrower query.
- **Every id is checked again on fetch** — the caller may have lost access to
  a row between the search and the fetch, so `messagesByIds` applies the same
  permission check as a single-row read.

Choose it when the search is the expensive part and the screen is a long
scroll. For every list that is a table of rows in a stable order — users,
admins, keys, the audit log — the connection above is shorter, needs one
request per page, and is what a machine client paging through the API expects.

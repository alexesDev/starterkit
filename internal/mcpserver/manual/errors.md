# Errors and limits

- A response with a top-level `errors` list comes back as a tool error. It means the document was malformed, asked for a field that does not exist, was refused (`unauthorized`: this key has no admin access), or the server failed.
- A refused document comes back with the schema of the types the error names, and the nearest names by spelling for a field, argument or type that does not exist.
- An `ErrorPayload` inside `data` is a normal answer to a mutation; see graphql_mutation.
- A long answer is cut; its last line says at how many characters and how to narrow the document.
- A document may cost at most 60 fields. A larger one is refused before it runs: split it into several requests.
- A revoked key, or a key whose owner was banned, is refused before any tool runs. Ask a person for a new key; retrying will not help.

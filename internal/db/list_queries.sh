#!/usr/bin/env bash
# Guards two authoring-time rules the compiler cannot see:
#
#   1. the read/write split, which the compiler enforces only once a query is
#      filed into the right .sql file, and the DB naming convention
#   2. the timestamp rule: every time column is integer unix seconds in a
#      STRICT table, and every one of them has a sqlc override mapping it to
#      dbtime.Stamp. Without an override sqlc silently generates int64.
#
# See docs/rules.md and docs/data-layer.md.
set -euo pipefail

READ_FILE="queries.read.sql"
WRITE_FILE="queries.write.sql"
SCHEMA_FILE="db/schema.sql"
SQLC_FILE="sqlc.yaml"
status=0

if grep -niE '(^|[^a-z])(insert into|update|delete)([^a-z]|$)' "$READ_FILE"; then
  echo "ERROR: $READ_FILE contains write operations."
  status=1
else
  echo "OK: $READ_FILE"
fi

if ! awk '
BEGIN { in_stmt=0; bad=0 }
/^[[:space:]]*--/ { next }
/^[[:space:]]*$/  { next }
{
  if (in_stmt == 0) {
    if (tolower($1) == "select") {
      print "ERROR: top-level SELECT at line " NR ": " $0
      bad=1
    }
    in_stmt=1
  }
  if ($0 ~ /;/) { in_stmt=0 }
}
END { exit bad }
' "$WRITE_FILE"
then
  status=1
else
  echo "OK: $WRITE_FILE"
fi

#   queries.read.sql   :one  -> DBGet...   (or DBCount..., whose verb is exact)
#                      :many -> DBList...
#   queries.write.sql        -> DB...
while read -r name kind; do
  case "$kind" in
    :one)  pattern='^DB(Get|Count)' ;;
    :many) pattern='^DBList' ;;
    *)     echo "ERROR: $name in $READ_FILE is $kind; reads are :one or :many."; status=1; continue ;;
  esac

  if ! [[ "$name" =~ $pattern ]]; then
    echo "ERROR: $name is $kind in $READ_FILE and must match $pattern."
    status=1
  fi
done < <(grep '^-- name:' "$READ_FILE" | awk '{print $3, $4}')

while read -r name _; do
  if ! [[ "$name" =~ ^DB ]]; then
    echo "ERROR: $name in $WRITE_FILE must start with DB."
    status=1
  fi
done < <(grep '^-- name:' "$WRITE_FILE" | awk '{print $3, $4}')

if [ "$status" -eq 0 ]; then
  echo "OK: query names"
fi

# Every table this project creates is STRICT. schema_migrations is dbmate's own
# and is declared varchar, which STRICT does not allow.
while read -r table; do
  [ "$table" = "schema_migrations" ] && continue

  if ! awk -v want="$table" '
    $0 ~ "^CREATE TABLE .*\\y" want "\\y" { inside=1 }
    inside && /\);?[[:space:]]*$/ { print; inside=0 }
    inside { print }
  ' "$SCHEMA_FILE" | grep -qiE '\)[[:space:]]*strict[[:space:]]*;'
  then
    echo "ERROR: table $table is not STRICT."
    status=1
  fi
done < <(grep -oiE '^CREATE TABLE (IF NOT EXISTS )?"?[a-z_]+"?' "$SCHEMA_FILE" | sed -E 's/.*[ "]([a-z_]+)"?$/\1/')

# A time column is integer unix seconds, and it has an override. A datetime or
# text one anywhere in the schema breaks the wildcard overrides in sqlc.yaml.
while read -r column type; do
  if [ "$type" != "integer" ]; then
    echo "ERROR: column $column is declared '$type'; a time column is integer unix seconds."
    status=1
    continue
  fi

  if ! grep -qE "column: \"\\*\.$column\"" "$SQLC_FILE"; then
    echo "ERROR: column $column has no overrides entry in $SQLC_FILE; sqlc would generate a bare int64."
    status=1
  fi
done < <(grep -oiE '^[[:space:]]+[a-z_]+(_at|_date) [a-z]+' "$SCHEMA_FILE" | awk '{print tolower($1), tolower($2)}' | sort -u)

if [ "$status" -eq 0 ]; then
  echo "OK: schema timestamps"
fi

exit $status

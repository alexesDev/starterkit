-- migrate:up

create table mcp_keys (
  id integer primary key autoincrement,
  user_id integer not null references users(id) on delete cascade,
  name text not null,
  key_hash blob not null unique,
  last_used_at integer,
  revoked_at integer,
  created_at integer not null
) strict;

-- migrate:down

drop table mcp_keys;

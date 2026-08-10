CREATE TABLE IF NOT EXISTS "schema_migrations" (version varchar(128) primary key);
CREATE TABLE users (
  id integer primary key autoincrement,
  oidc_issuer text not null,
  oidc_subject text not null,
  email text not null,
  name text not null default '',
  last_signin_at integer,
  created_at integer not null
) strict;
CREATE UNIQUE INDEX users_oidc_identity on users (oidc_issuer, oidc_subject);
CREATE INDEX users_email on users (email);
CREATE TABLE admins (
  user_id integer primary key references users(id) on delete cascade,
  created_at integer not null
) strict;
CREATE TABLE user_bans (
  user_id integer primary key references users(id) on delete cascade,
  banned_by integer references admins(user_id) on delete restrict,
  reason text not null,
  created_at integer not null
) strict;
CREATE TABLE audit_log (
  id integer primary key autoincrement,
  user_id integer,
  email text not null default '',
  action text not null,
  detail text not null default '',
  ip text not null default '',
  created_at integer not null
) strict;
CREATE INDEX audit_log_created_at_idx on audit_log (created_at desc);
CREATE TABLE goqite (
  id text primary key default ('m_' || lower(hex(randomblob(16)))),
  created text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  updated text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  queue text not null,
  body blob not null,
  timeout text not null default (strftime('%Y-%m-%dT%H:%M:%fZ')),
  received integer not null default 0,
  priority integer not null default 0
) strict;
CREATE TRIGGER goqite_updated_timestamp after update on goqite begin
  update goqite set updated = strftime('%Y-%m-%dT%H:%M:%fZ') where id = old.id;
end;
CREATE INDEX goqite_queue_priority_created_idx on goqite (queue, priority desc, created);
-- Dbmate schema migrations
INSERT INTO "schema_migrations" (version) VALUES
  ('20260810090000'),
  ('20260810090100'),
  ('20260810090200'),
  ('20260810090300'),
  ('20260810090400');

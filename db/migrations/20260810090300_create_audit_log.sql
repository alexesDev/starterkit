-- migrate:up

create table audit_log (
  id integer primary key autoincrement,
  user_id integer,
  email text not null default '',
  action text not null,
  detail text not null default '',
  ip text not null default '',
  created_at integer not null
) strict;

create index audit_log_created_at_idx on audit_log (created_at desc);

-- migrate:down

drop index audit_log_created_at_idx;
drop table audit_log;

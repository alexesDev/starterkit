-- migrate:up

create table user_bans (
  user_id integer primary key references users(id) on delete cascade,
  banned_by integer references admins(user_id) on delete restrict,
  reason text not null,
  created_at integer not null
) strict;

-- migrate:down

drop table user_bans;

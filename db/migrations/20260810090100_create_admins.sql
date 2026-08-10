-- migrate:up

create table admins (
  user_id integer primary key references users(id) on delete cascade,
  created_at integer not null
) strict;

-- migrate:down

drop table admins;

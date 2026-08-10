-- migrate:up

create table users (
  id integer primary key autoincrement,
  oidc_issuer text not null,
  oidc_subject text not null,
  email text not null,
  name text not null default '',
  last_signin_at integer,
  created_at integer not null
) strict;

create unique index users_oidc_identity on users (oidc_issuer, oidc_subject);
create index users_email on users (email);

-- migrate:down

drop table users;

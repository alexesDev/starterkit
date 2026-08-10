-- name: DBUpsertUserByOIDC :one
insert into users (oidc_issuer, oidc_subject, email, name, last_signin_at, created_at)
values (?, ?, ?, ?, ?, ?)
on conflict(oidc_issuer, oidc_subject) do update set
  email = excluded.email,
  name = excluded.name,
  last_signin_at = excluded.last_signin_at
returning *;

-- name: DBMarkSignIn :execrows
update users
   set last_signin_at = sqlc.arg(issued_at)
 where users.id = sqlc.arg(id)
   and (users.last_signin_at is null or users.last_signin_at < sqlc.arg(issued_at));

-- name: DBInsertAdmin :exec
insert into admins (user_id, created_at) values (?, ?) on conflict(user_id) do nothing;

-- name: DBDeleteAdmin :execrows
delete from admins
 where admins.user_id = ?
   and (select count(*) from admins) > 1;

-- name: DBInsertUserBan :one
insert into user_bans (user_id, banned_by, reason, created_at)
values (?, ?, ?, ?)
on conflict(user_id) do update set reason = excluded.reason
returning *;

-- name: DBDeleteUserBan :exec
delete from user_bans where user_id = ?;

-- name: DBInsertAuditLog :exec
insert into audit_log (user_id, email, action, detail, ip, created_at)
values (?, ?, ?, ?, ?, ?);

-- name: DBDeleteAuditLogBefore :exec
delete from audit_log where created_at < ?;

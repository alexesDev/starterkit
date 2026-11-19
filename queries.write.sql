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
insert into admins (user_id, created_at) values (?, ?);

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

-- name: DBMoveJobToQueue :exec
update goqite set queue = @queue where id = @id;

-- name: DBInsertMCPKey :one
insert into mcp_keys (user_id, name, key_hash, created_at)
values (?, ?, ?, ?)
returning *;

-- name: DBRevokeMCPKey :execrows
update mcp_keys set revoked_at = ? where id = ? and revoked_at is null;

-- name: DBDeleteMCPKey :execrows
delete from mcp_keys where id = ?;

-- name: DBMarkMCPKeyUsed :exec
update mcp_keys
   set last_used_at = sqlc.arg(used_at)
 where mcp_keys.id = sqlc.arg(id)
   and (mcp_keys.last_used_at is null or mcp_keys.last_used_at < sqlc.arg(stale_before));

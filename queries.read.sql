-- name: DBGetUserByID :one
select * from users where id = ?;

-- name: DBGetUserBan :one
select * from user_bans where user_id = ?;

-- name: DBGetAdminByUserID :one
select * from admins where user_id = ?;

-- name: DBGetIdentityByOIDC :one
select u.id,
       u.email,
       u.name,
       u.created_at,
       u.last_signin_at,
       case when a.user_id is null then 0 else 1 end as is_admin,
       coalesce(b.reason, '') as ban_reason
  from users u
  left join admins a on a.user_id = u.id
  left join user_bans b on b.user_id = u.id
 where u.oidc_issuer = ? and u.oidc_subject = ?;

-- name: DBListUsers :many
select u.id,
       u.email,
       u.name,
       u.created_at,
       u.last_signin_at,
       case when a.user_id is null then 0 else 1 end as is_admin,
       coalesce(b.reason, '') as ban_reason
  from users u
  left join admins a on a.user_id = u.id
  left join user_bans b on b.user_id = u.id
 order by u.id;

-- name: DBGetAdminUserByID :one
select u.id,
       u.email,
       u.name,
       u.created_at,
       u.last_signin_at,
       case when a.user_id is null then 0 else 1 end as is_admin,
       coalesce(b.reason, '') as ban_reason
  from users u
  left join admins a on a.user_id = u.id
  left join user_bans b on b.user_id = u.id
 where u.id = ?;

-- name: DBCountUsers :one
select count(*) from users;

-- name: DBListAdmins :many
select a.user_id, a.created_at, u.email, u.name
  from admins a
  join users u on u.id = a.user_id
 order by a.created_at desc;

-- name: DBCountAdmins :one
select count(*) from admins;

-- name: DBListAuditLog :many
select * from audit_log
 where (id < sqlc.arg(before) or sqlc.arg(before) = 0)
 order by id desc
 limit sqlc.arg(page_limit);

-- name: DBCountAuditLog :one
select count(*) from audit_log;

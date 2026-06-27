-- name: CreateFeed :one
INSERT INTO feeds (
    id,
    created_at,
    updated_at,
    name,
    url,
    user_id
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
)
RETURNING *;

-- name: GetFeedsAndUserNames :many
SELECT f.name as feed_name, f.url as feed_url, u.name as user_name 
FROM feeds f 
JOIN users u ON u.id = f.user_id;

-- name: GetFeedByUrl :one
SELECT * FROM feeds WHERE url = $1;
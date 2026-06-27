-- name: CreateFeedFollow :one
WITH inserted_feed_follow AS (
    INSERT INTO feed_follows (id, created_at, updated_at, user_id, feed_id) 
    VALUES ($1, $2, $3, $4, $5)
    RETURNING *
) 
SELECT inserted_feed_follow.*, f.name as feed_name, u.name as user_name
FROM inserted_feed_follow
JOIN feeds f on f.id = inserted_feed_follow.feed_id
JOIN users u on u.id = inserted_feed_follow.user_id;

-- name: GetFeedFollowsForUser :many
SELECT 
    ff.*, 
    f.name as feed_name, 
    u.name as user_name 
FROM feed_follows ff
JOIN feeds f on f.id = ff.feed_id
JOIN users u on u.id = ff.user_id
WHERE u.name = $1;
DROP INDEX IF EXISTS posts_parent_idx;
DROP INDEX IF EXISTS posts_root_created_idx;

ALTER TABLE IF EXISTS posts
DROP COLUMN IF EXISTS root_post_id,
DROP COLUMN IF EXISTS parent_post_id;

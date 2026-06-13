ALTER TABLE IF EXISTS posts
ADD COLUMN IF NOT EXISTS parent_post_id UUID REFERENCES posts (id),
ADD COLUMN IF NOT EXISTS root_post_id UUID REFERENCES posts (id);

CREATE INDEX IF NOT EXISTS posts_root_created_idx ON posts (root_post_id, created_at ASC);

CREATE INDEX IF NOT EXISTS posts_parent_idx ON posts (parent_post_id);

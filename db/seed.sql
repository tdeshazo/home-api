INSERT INTO users (id, email, handle, display_name, password_hash, is_admin)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'alice@example.com', 'alice', 'Alice Example', 'dev-password-hash', true),
  ('00000000-0000-0000-0000-000000000002', 'bob@example.com', 'bob', 'Bob Example', 'dev-password-hash', false)
ON CONFLICT (id) DO UPDATE
SET
  email = EXCLUDED.email,
  handle = EXCLUDED.handle,
  display_name = EXCLUDED.display_name,
  is_admin = EXCLUDED.is_admin;

INSERT INTO posts (id, user_id, body)
VALUES
  ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Alice first post'),
  ('10000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'Bob first post')
ON CONFLICT (id) DO NOTHING;

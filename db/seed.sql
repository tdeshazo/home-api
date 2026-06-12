INSERT INTO users (id, email, handle, display_name, password_hash)
VALUES
  ('00000000-0000-0000-0000-000000000001', 'alice@example.com', 'alice', 'Alice Example', 'dev-password-hash'),
  ('00000000-0000-0000-0000-000000000002', 'bob@example.com', 'bob', 'Bob Example', 'dev-password-hash')
ON CONFLICT (id) DO NOTHING;

INSERT INTO posts (id, user_id, body)
VALUES
  ('10000000-0000-0000-0000-000000000001', '00000000-0000-0000-0000-000000000001', 'Alice first post'),
  ('10000000-0000-0000-0000-000000000002', '00000000-0000-0000-0000-000000000002', 'Bob first post')
ON CONFLICT (id) DO NOTHING;

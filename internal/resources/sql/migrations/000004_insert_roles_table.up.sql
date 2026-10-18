-- Insert default roles into the 'roles' table.
-- Permissions are tightened by migration 000008.
INSERT INTO public.roles (name, routes, permissions) VALUES
  ('admin', ARRAY['USERS', 'ROLES'], '[{"read": true, "create": true, "delete": true, "entity": "USERS", "update": true}, {"read": true, "create": true, "delete": true, "entity": "ROLES", "update": true}]'::jsonb),
  ('user', ARRAY['USERS', 'ROLES'], '[{"read": true, "create": true, "delete": true, "entity": "USERS", "update": true}, {"read": true, "create": true, "delete": true, "entity": "ROLES", "update": true}]'::jsonb),
  ('guest', ARRAY['USERS', 'ROLES'], '[{"read": true, "create": true, "delete": true, "entity": "USERS", "update": true}, {"read": true, "create": true, "delete": true, "entity": "ROLES", "update": true}]'::jsonb);

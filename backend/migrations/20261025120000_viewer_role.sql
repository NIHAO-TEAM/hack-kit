-- +goose Up
-- Роль viewer — гость, только просмотр (POST /auth/guest).
ALTER TABLE public.users DROP CONSTRAINT users_role_check;
ALTER TABLE public.users ADD CONSTRAINT users_role_check CHECK (role IN ('marketer', 'analyst', 'admin', 'jury', 'viewer'));

-- +goose Down
DELETE FROM public.users WHERE role = 'viewer';
ALTER TABLE public.users DROP CONSTRAINT users_role_check;
ALTER TABLE public.users ADD CONSTRAINT users_role_check CHECK (role IN ('marketer', 'analyst', 'admin', 'jury'));

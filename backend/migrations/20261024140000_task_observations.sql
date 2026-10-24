-- +goose Up
-- Наблюдения биллинга после взятия в работу: вид observed и ключ, по которому одно наблюдение не пишется дважды.
ALTER TABLE public.retention_task_events DROP CONSTRAINT retention_task_events_kind_check;
ALTER TABLE public.retention_task_events ADD CONSTRAINT retention_task_events_kind_check
    CHECK (kind IN ('created', 'changed', 'contact', 'note', 'observed'));
ALTER TABLE public.retention_task_events ADD COLUMN obs_key text;
CREATE UNIQUE INDEX retention_task_events_obs_key_idx ON public.retention_task_events (task_id, obs_key) WHERE obs_key IS NOT NULL;

-- +goose Down
DROP INDEX public.retention_task_events_obs_key_idx;
ALTER TABLE public.retention_task_events DROP COLUMN obs_key;
DELETE FROM public.retention_task_events WHERE kind = 'observed';
ALTER TABLE public.retention_task_events DROP CONSTRAINT retention_task_events_kind_check;
ALTER TABLE public.retention_task_events ADD CONSTRAINT retention_task_events_kind_check
    CHECK (kind IN ('created', 'changed', 'contact', 'note'));

-- +goose Up
-- Окно принятых eventId для POST /source/events: повтор id не применяется. Строки старше 24 часов
-- чистит сам приём (не чаще раза в час). Ссылок на таблицы снимка нет: снимок заменяется целиком.
CREATE TABLE public.source_event_ids (
    event_id text PRIMARY KEY,
    seen_at timestamptz NOT NULL
);

CREATE INDEX source_event_ids_seen_idx ON public.source_event_ids (seen_at);

-- +goose Down
DROP TABLE public.source_event_ids;

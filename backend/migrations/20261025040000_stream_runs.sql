-- +goose Up
-- Состояние прогона потока событий: его пишет реплика, которая держит advisory-блокировку прогона,
-- а читает любая реплика (GET /source/emulator/stream). Одна строка на прогон; ссылок на другие таблицы нет.
CREATE TABLE public.stream_runs (
    run_id text PRIMARY KEY,
    state jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX stream_runs_updated_idx ON public.stream_runs (updated_at DESC);

-- +goose Down
DROP TABLE public.stream_runs;

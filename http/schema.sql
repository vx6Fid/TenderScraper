-- Schema for the tender-scraper job pipeline.
-- Applied automatically on HTTP service startup (see initSchema in main.go).

CREATE TABLE IF NOT EXISTS jobs (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    status         TEXT NOT NULL DEFAULT 'pending',
    payload        JSONB NOT NULL,
    go_result      JSONB,
    python_result  JSONB,
    attempts       INT NOT NULL DEFAULT 0,
    last_error     TEXT,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs (status);

CREATE TABLE IF NOT EXISTS outbox (
    id           BIGSERIAL PRIMARY KEY,
    job_id       UUID NOT NULL REFERENCES jobs (id) ON DELETE CASCADE,
    destination  TEXT NOT NULL,
    payload      JSONB NOT NULL,
    sent         BOOLEAN NOT NULL DEFAULT false,
    attempts     INT NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_outbox_unsent ON outbox (id) WHERE sent = false;

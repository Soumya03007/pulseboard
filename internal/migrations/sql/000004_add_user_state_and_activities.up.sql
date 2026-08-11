CREATE TABLE IF NOT EXISTS activities (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_activities_user_created ON activities (user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_activities_user_status ON activities (user_id, status);

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS presence TEXT NOT NULL DEFAULT 'online',
    ADD COLUMN IF NOT EXISTS availability TEXT NOT NULL DEFAULT 'available',
    ADD COLUMN IF NOT EXISTS current_activity_id BIGINT REFERENCES activities(id) ON DELETE SET NULL;

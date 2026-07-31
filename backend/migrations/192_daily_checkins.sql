CREATE TABLE IF NOT EXISTS daily_checkins (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    checkin_date DATE NOT NULL,
    base_reward DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (base_reward >= 0),
    bonus_reward DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (bonus_reward >= 0),
    total_reward DECIMAL(20, 8) NOT NULL DEFAULT 0 CHECK (total_reward >= 0),
    streak_count INTEGER NOT NULL DEFAULT 1 CHECK (streak_count >= 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT daily_checkins_user_date_key UNIQUE (user_id, checkin_date)
);

CREATE INDEX IF NOT EXISTS daily_checkins_user_date_idx
    ON daily_checkins (user_id, checkin_date DESC);

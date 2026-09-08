-- Additive ownership metadata; existing resources remain administrator-managed.
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS manager_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS manager_user_id BIGINT REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE groups ADD COLUMN IF NOT EXISTS allow_operator_contribution BOOLEAN NOT NULL DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS idx_accounts_manager_user_id ON accounts(manager_user_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_groups_manager_user_id ON groups(manager_user_id) WHERE deleted_at IS NULL;

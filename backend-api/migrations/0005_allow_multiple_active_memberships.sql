DROP INDEX IF EXISTS idx_family_members_one_active_per_user;

CREATE INDEX IF NOT EXISTS idx_family_members_user_status
  ON family_members (user_id, status);

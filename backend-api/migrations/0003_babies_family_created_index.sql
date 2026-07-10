CREATE INDEX IF NOT EXISTS idx_babies_family_created
  ON babies (family_id, created_at);

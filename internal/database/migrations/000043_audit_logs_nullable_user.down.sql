-- Fails if system audit rows (user_id IS NULL) exist; they are never deleted automatically.
ALTER TABLE audit_logs ALTER COLUMN user_id SET NOT NULL;

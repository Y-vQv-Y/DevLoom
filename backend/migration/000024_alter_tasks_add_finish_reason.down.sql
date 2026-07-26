BEGIN;

ALTER TABLE tasks
  DROP CONSTRAINT IF EXISTS tasks_finish_reason_check,
  DROP COLUMN IF EXISTS finish_reason;

COMMIT;

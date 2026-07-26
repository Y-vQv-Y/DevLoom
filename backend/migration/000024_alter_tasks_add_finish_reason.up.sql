BEGIN;

ALTER TABLE tasks
  ADD COLUMN IF NOT EXISTS finish_reason VARCHAR(255);

UPDATE tasks
SET finish_reason = 'completed'
WHERE status = 'finished' AND finish_reason IS NULL;

ALTER TABLE tasks
  DROP CONSTRAINT IF EXISTS tasks_finish_reason_check;

ALTER TABLE tasks
  ADD CONSTRAINT tasks_finish_reason_check
  CHECK (finish_reason IS NULL OR finish_reason IN ('completed', 'cancelled'));

COMMIT;

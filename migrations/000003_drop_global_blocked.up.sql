DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'users'
          AND column_name = 'blocked'
    ) THEN
        UPDATE users SET roles = 'BLOCKED' WHERE blocked IS TRUE;
        ALTER TABLE users DROP COLUMN blocked;
    END IF;
END $$;

DROP INDEX IF EXISTS users_blocked_idx;

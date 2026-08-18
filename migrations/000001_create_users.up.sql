CREATE TYPE roles AS ENUM ('ADMIN', 'STRANGER');
CREATE TYPE storage_roles AS ENUM ('ADMIN', 'FAMILY', 'STRANGER');
CREATE TYPE resume_roles AS ENUM ('ADMIN', 'FAMILY', 'STRANGER');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    google_sub TEXT NOT NULL UNIQUE,
    google_email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    roles roles NOT NULL DEFAULT 'STRANGER',
    storage_roles storage_roles NOT NULL DEFAULT 'STRANGER',
    resume_roles resume_roles NOT NULL DEFAULT 'STRANGER',
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    last_seen_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX users_roles_idx ON users (roles);
CREATE INDEX users_blocked_idx ON users (blocked);
CREATE INDEX users_google_email_idx ON users (google_email);

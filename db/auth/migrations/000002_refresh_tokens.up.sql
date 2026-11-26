BEGIN;
CREATE TABLE refresh_tokens (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea       NOT NULL UNIQUE,
    family_id  uuid        NOT NULL,
    parent_id  uuid        REFERENCES refresh_tokens (id) ON DELETE CASCADE,
    rotated_at timestamptz,
    revoked_at timestamptz,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- Reuse and logout revoke a whole family at once.
CREATE INDEX refresh_tokens_family_id_idx ON refresh_tokens (family_id);
-- A user's sessions, and the foreign key from users.
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

COMMIT;

CREATE TABLE users (
    id UUID PRIMARY KEY,
    phone_number TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT users_phone_number_unique UNIQUE (phone_number)
);

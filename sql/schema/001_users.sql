-- +goose Up
CREATE TABLE Users (
    id UUID,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,

    email TEXT NOT NULL,
    UNIQUE (email),

    PRIMARY KEY (id)
);

-- +goose Down
DROP TABLE Users;

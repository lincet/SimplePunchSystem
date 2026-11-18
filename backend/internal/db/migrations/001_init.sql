CREATE TABLE users (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    role                 TEXT    NOT NULL CHECK (role IN ('admin', 'employee')),
    username             TEXT    NOT NULL UNIQUE,
    name                 TEXT    NOT NULL,
    phone                TEXT    NOT NULL DEFAULT '',
    gender               TEXT    NOT NULL DEFAULT 'other' CHECK (gender IN ('male', 'female', 'other')),
    employee_no          TEXT    NOT NULL DEFAULT '',
    title                TEXT    NOT NULL DEFAULT '',
    hire_date            TEXT    NOT NULL DEFAULT '',
    email                TEXT    NOT NULL DEFAULT '',
    note                 TEXT    NOT NULL DEFAULT '',
    password_hash        TEXT    NOT NULL,
    must_change_password INTEGER NOT NULL DEFAULT 1,
    active               INTEGER NOT NULL DEFAULT 1,
    created_at           TEXT    NOT NULL,
    updated_at           TEXT    NOT NULL
);

CREATE TABLE sessions (
    token_hash TEXT    PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE punches (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT    NOT NULL CHECK (type IN ('in', 'out')),
    punched_at TEXT    NOT NULL,
    photo_path TEXT    NOT NULL,
    user_agent TEXT    NOT NULL DEFAULT '',
    ip         TEXT    NOT NULL DEFAULT '',
    created_at TEXT    NOT NULL
);
CREATE INDEX idx_punches_user_time ON punches(user_id, punched_at);
CREATE INDEX idx_punches_time ON punches(punched_at);

CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS user_library (
    user_id INTEGER NOT NULL REFERENCES users(id),
    manga_id TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('plan-to-read', 'reading', 'completed')),
    current_chapter INTEGER NOT NULL CHECK (current_chapter >= 0),
    updated_at TEXT NOT NULL,
    version INTEGER NOT NULL CHECK (version > 0),
    PRIMARY KEY (user_id, manga_id)
);

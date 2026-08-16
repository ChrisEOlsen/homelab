CREATE TABLE clients (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    match_name TEXT NOT NULL,
    email TEXT,
    phone TEXT,
    rate_cents INTEGER NOT NULL DEFAULT 10000,
    kind TEXT NOT NULL DEFAULT 'independent',
    is_active INTEGER NOT NULL DEFAULT 1,
    notes TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX idx_clients_match_name ON clients(match_name COLLATE NOCASE);

INSERT INTO clients (name, match_name, email, phone, rate_cents, kind) VALUES
  ('Adam Schwarzschild', 'Adam Schwarzschild', 'adam.schwarzschild@gmail.com', '4044412313', 10000, 'independent'),
  ('Susie Kim',          'Susie Kim',          'susieahnkim@gmail.com',        '9178808738', 10000, 'independent'),
  ('Puneet Riverside',   'Puneet Riverside',   'puneet@assigned.com',          NULL,         10000, 'independent'),
  ('John Kublacki',      'John Kublacki',      'john@assigned.com',            NULL,         10000, 'independent'),
  ('Ofer Rubin',         'Ofer Rubin',         NULL,                           NULL,         10000, 'independent');

CREATE TABLE rate_rules (
    id INTEGER PRIMARY KEY,
    duration_min INTEGER NOT NULL UNIQUE,
    amount_cents INTEGER NOT NULL,
    label TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO rate_rules (duration_min, amount_cents, label) VALUES
  (30, 4500, '30 minutes'),
  (45, 5000, '45 minutes'),
  (60, 6000, '60 minutes');

CREATE TABLE training_sessions (
    id INTEGER PRIMARY KEY,
    uid TEXT NOT NULL UNIQUE,
    source TEXT NOT NULL,
    client_name TEXT NOT NULL,
    client_id INTEGER REFERENCES clients(id) ON DELETE SET NULL,
    service TEXT,
    session_date TEXT NOT NULL,
    start_at TEXT NOT NULL,
    end_at TEXT NOT NULL,
    duration_min INTEGER NOT NULL,
    amount_cents INTEGER NOT NULL DEFAULT 0,
    rate_source TEXT NOT NULL DEFAULT 'unknown',
    override_cents INTEGER,
    status TEXT NOT NULL DEFAULT 'scheduled',
    needs_review INTEGER NOT NULL DEFAULT 0,
    first_seen_at TEXT,
    last_seen_at TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_training_sessions_date ON training_sessions(session_date);
CREATE INDEX idx_training_sessions_status ON training_sessions(status);

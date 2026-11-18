CREATE TABLE families (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  invite_code TEXT NOT NULL UNIQUE,
  created_by INTEGER NOT NULL REFERENCES users(id),
  created_at INTEGER NOT NULL
);

CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  username TEXT NOT NULL UNIQUE COLLATE NOCASE,
  password_hash TEXT NOT NULL,
  family_id INTEGER REFERENCES families(id) ON DELETE SET NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_users_family ON users(family_id);

CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY,
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at INTEGER NOT NULL,
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE profiles (
  id INTEGER PRIMARY KEY,
  display_name TEXT NOT NULL,
  user_id INTEGER UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  manager_user_id INTEGER REFERENCES users(id) ON DELETE CASCADE,
  gender TEXT NOT NULL DEFAULT '',
  birth_year INTEGER,
  medical_history TEXT NOT NULL DEFAULT '',
  medications TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL,
  CHECK ((user_id IS NULL) <> (manager_user_id IS NULL))
);
CREATE INDEX idx_profiles_manager ON profiles(manager_user_id);

CREATE TABLE readings (
  id INTEGER PRIMARY KEY,
  profile_id INTEGER NOT NULL REFERENCES profiles(id) ON DELETE CASCADE,
  measured_at INTEGER NOT NULL,
  systolic INTEGER NOT NULL,
  diastolic INTEGER NOT NULL,
  pulse INTEGER,
  note TEXT NOT NULL DEFAULT '',
  created_by INTEGER NOT NULL REFERENCES users(id),
  created_at INTEGER NOT NULL
);
CREATE INDEX idx_readings_profile_time ON readings(profile_id, measured_at);

CREATE TABLE reading_items (
  reading_id INTEGER NOT NULL REFERENCES readings(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL CHECK (seq BETWEEN 1 AND 3),
  systolic INTEGER NOT NULL,
  diastolic INTEGER NOT NULL,
  pulse INTEGER,
  PRIMARY KEY (reading_id, seq)
);

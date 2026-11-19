-- Rebuild the three related tables with foreign keys enabled. Copy the complete
-- hierarchy before dropping the old tables in child-first order.
CREATE TABLE profiles_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
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

CREATE TABLE readings_new (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id INTEGER NOT NULL REFERENCES profiles_new(id) ON DELETE CASCADE,
  measured_at INTEGER NOT NULL,
  systolic INTEGER NOT NULL,
  diastolic INTEGER NOT NULL,
  pulse INTEGER,
  note TEXT NOT NULL DEFAULT '',
  created_by INTEGER NOT NULL REFERENCES users(id),
  created_at INTEGER NOT NULL
);

CREATE TABLE reading_items_new (
  reading_id INTEGER NOT NULL REFERENCES readings_new(id) ON DELETE CASCADE,
  seq INTEGER NOT NULL CHECK (seq BETWEEN 1 AND 3),
  systolic INTEGER NOT NULL,
  diastolic INTEGER NOT NULL,
  pulse INTEGER,
  PRIMARY KEY (reading_id, seq)
);

INSERT INTO profiles_new
  SELECT id, display_name, user_id, manager_user_id, gender, birth_year,
         medical_history, medications, created_at FROM profiles;
INSERT INTO readings_new
  SELECT id, profile_id, measured_at, systolic, diastolic, pulse, note, created_by, created_at FROM readings;
INSERT INTO reading_items_new
  SELECT reading_id, seq, systolic, diastolic, pulse FROM reading_items;

DROP TABLE reading_items;
DROP TABLE readings;
DROP TABLE profiles;

ALTER TABLE profiles_new RENAME TO profiles;
ALTER TABLE readings_new RENAME TO readings;
ALTER TABLE reading_items_new RENAME TO reading_items;

CREATE INDEX idx_profiles_manager ON profiles(manager_user_id);
CREATE INDEX idx_readings_profile_time ON readings(profile_id, measured_at);

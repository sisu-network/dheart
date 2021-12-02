CREATE TABLE IF NOT EXISTS preparams(
  key_type VARCHAR(256),
  preparams BLOB,
  created_time DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (key_type))
;

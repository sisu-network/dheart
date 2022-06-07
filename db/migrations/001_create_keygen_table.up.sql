CREATE TABLE IF NOT EXISTS keygen(
  key_type VARCHAR(256),
  work_id VARCHAR(256),
  pids_string TEXT,
  keygen_output BLOB,
  created_time DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (key_type, work_id))
;

CREATE TABLE IF NOT EXISTS keygen(
  chain VARCHAR(256),
  work_id VARCHAR(256),
  batch_index INTEGER,
  pids_string TEXT,
  keygen_output BLOB,
  created_time DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (chain, work_id, batch_index))
;

CREATE TABLE IF NOT EXISTS presign(
  presign_id VARCHAR(256),
  work_id VARCHAR(256),
  batch_index INTEGER,
  pids_string TEXT,
  status VARCHAR(64),
  presign_output BLOB,
  created_time DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (work_id, batch_index)
);
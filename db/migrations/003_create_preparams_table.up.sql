CREATE TABLE preparams(
  chain VARCHAR(256),
  preparams BLOB,
  created_time DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (chain))
;

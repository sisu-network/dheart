CREATE TABLE IF NOT EXISTS peers(
  `address` VARCHAR(256),
  pubkey VARCHAR(256),
  pubkey_type VARCHAR(256),
  PRIMARY KEY (pubkey))
;

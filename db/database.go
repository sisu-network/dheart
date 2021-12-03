package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/lib/log"
	"github.com/sisu-network/tss-lib/ecdsa/keygen"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"

	libchain "github.com/sisu-network/lib/chain"
)

const (
	PresignStatusNotUsed = "not_used"
	PresignStatusUsed    = "used"
)

var (
	ErrNotFound = errors.New("not found")
)

type Database interface {
	Init() error

	SavePreparams(preparams *keygen.LocalPreParams) error
	LoadPreparams() (*keygen.LocalPreParams, error)

	SaveKeygenData(keyType string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error
	LoadKeygenData(chain string) (*keygen.LocalPartySaveData, error)

	SavePresignData(chain string, workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error
	GetAvailablePresignShortForm() ([]string, []string, error) // Returns presignIds, pids, error

	LoadPresign(presignIds []string) ([]*presign.LocalPresignData, error)
	UpdatePresignStatus(presignIds []string) error
}

type dbLogger struct {
}

func (loggger *dbLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
}

func (loggger *dbLogger) Verbose() bool {
	return true
}

// --- /

// SqlDatabase implements Database interface.
type SqlDatabase struct {
	db     *sql.DB
	config *config.DbConfig
}

func NewDatabase(config *config.DbConfig) Database {
	return &SqlDatabase{
		config: config,
	}
}

func (d *SqlDatabase) Connect() error {
	host := d.config.Host
	if host == "" {
		return fmt.Errorf("DB host cannot be empty")
	}

	username := d.config.Username
	password := d.config.Password
	schema := d.config.Schema

	log.Info("Schema = ", schema)

	// Connect to the db
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/", username, password, host, d.config.Port))
	if err != nil {
		return err
	}
	_, err = database.Exec("CREATE DATABASE IF NOT EXISTS " + schema)
	if err != nil {
		return err
	}
	database.Close()

	database, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", username, password, host, d.config.Port, schema))
	if err != nil {
		return err
	}

	d.db = database
	log.Info("Db is connected successfully")
	return nil
}

func (d *SqlDatabase) DoMigration() error {
	driver, err := mysql.WithInstance(d.db, &mysql.Config{})
	if err != nil {
		return err
	}

	log.Info("Migration path =", d.config.MigrationPath)

	m, err := migrate.NewWithDatabaseInstance(
		d.config.MigrationPath,
		"mysql",
		driver,
	)

	if err != nil {
		return err
	}

	m.Log = &dbLogger{}
	m.Up()

	return nil
}

func (d *SqlDatabase) Init() error {
	err := d.Connect()
	if err != nil {
		log.Error("Failed to connect to DB. Err =", err)
		return err
	}

	err = d.DoMigration()
	if err != nil {
		log.Error("Cannot do migration. Err =", err)
		return err
	}

	return nil
}

func (d *SqlDatabase) SavePreparams(preparams *keygen.LocalPreParams) error {
	bz, err := json.Marshal(preparams)
	if err != nil {
		return err
	}

	params := []interface{}{libchain.KEY_TYPE_ECDSA, bz}

	query := "INSERT INTO preparams (key_type, preparams) VALUES (?, ?)"
	_, err = d.db.Exec(query, params...)
	return err
}

func (d *SqlDatabase) LoadPreparams() (*keygen.LocalPreParams, error) {
	query := "SELECT preparams FROM preparams WHERE key_type=?"
	rows, err := d.db.Query(query, libchain.KEY_TYPE_ECDSA)
	if err != nil {
		return nil, err
	}

	if !rows.Next() {
		return nil, ErrNotFound
	}

	var bz []byte
	err = rows.Scan(&bz)
	if err != nil {
		return nil, err
	}

	preparams := &keygen.LocalPreParams{}
	err = json.Unmarshal(bz, preparams)
	if err != nil {
		return nil, err
	}

	return preparams, nil
}

func (d *SqlDatabase) SaveKeygenData(keyType string, workId string, pids []*tss.PartyID, keygenOutput []*keygen.LocalPartySaveData) error {
	if len(keygenOutput) == 0 {
		return nil
	}

	pidString := getPidString(pids)
	// Constructs multi-insert query to do all insertion in 1 query.
	query := "INSERT INTO keygen (key_type, work_id, pids_string, batch_index, keygen_output) VALUES "

	rowCount := 0
	params := make([]interface{}, 0)
	for i, output := range keygenOutput {
		bz, err := json.Marshal(output)
		if err != nil {
			return err
		}

		params = append(params, keyType)
		params = append(params, workId)
		params = append(params, pidString)
		params = append(params, i) // batch index
		params = append(params, bz)

		rowCount++
	}

	query = query + getQueryQuestionMark(rowCount, 5)
	_, err := d.db.Exec(query, params...)

	return err
}

func (d *SqlDatabase) LoadKeygenData(keyType string) (*keygen.LocalPartySaveData, error) {
	query := "SELECT keygen_output FROM keygen WHERE key_type=? AND batch_index=0"
	params := []interface{}{
		keyType,
	}

	rows, err := d.db.Query(query, params...)
	if err != nil {
		return nil, err
	}

	result := &keygen.LocalPartySaveData{}
	if rows.Next() {
		var bz []byte

		if err := rows.Scan(&bz); err != nil {
			log.Error("Cannot scan row", err)
			return nil, err
		}

		if err := json.Unmarshal(bz, result); err != nil {
			log.Error("Cannot unmarshal result", err)
			return nil, err
		}
	} else {
		log.Verbose("There is no such keygen output for ", keyType)
	}

	return result, nil
}

func (d *SqlDatabase) SavePresignData(chain string, workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error {
	if len(presignOutputs) == 0 {
		return nil
	}

	pidString := getPidString(pids)

	// Constructs multi-insert query to do all insertion in 1 query.
	query := "INSERT INTO presign (presign_id, chain, work_id, pids_string, batch_index, status, presign_output) VALUES "
	query = query + getQueryQuestionMark(len(presignOutputs), 7)

	params := make([]interface{}, 0)
	for i, output := range presignOutputs {
		bz, err := json.Marshal(output)
		if err != nil {
			return err
		}

		presignId := fmt.Sprintf("%s-%d", workId, i)

		params = append(params, presignId)
		params = append(params, chain)
		params = append(params, workId)
		params = append(params, pidString)

		params = append(params, i) // batch_index
		params = append(params, PresignStatusNotUsed)

		params = append(params, bz)
	}

	_, err := d.db.Exec(query, params...)

	return err
}

// GetAllPresignIndexes returns all available presign data sets in short form (pids, workId, index)
// We don't want to load full data of presign sets since it might take too much memmory.
func (d *SqlDatabase) GetAvailablePresignShortForm() ([]string, []string, error) {
	query := fmt.Sprintf("SELECT presign_id, pids_string FROM presign WHERE status='%s'", PresignStatusNotUsed)
	pids := make([]string, 0)
	presignIds := make([]string, 0)

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, nil, err
	}

	for rows.Next() {
		var presignId, pid string
		if err := rows.Scan(&presignId, &pid); err != nil {
			log.Error("cannot scan row", err)
			return nil, nil, err
		}

		presignIds = append(presignIds, presignId)
		pids = append(pids, pid)
	}

	return presignIds, pids, nil
}

// This is not part of Database interface. Should ony be used in testing since we don't want to delete
// presign rows.
func (d *SqlDatabase) DeletePresignWork(workId string) error {
	_, err := d.db.Exec("DELETE FROM presign WHERE work_id = ?", workId)
	return err
}

func (d *SqlDatabase) DeleteKeygenWork(workId string) error {
	_, err := d.db.Exec("DELETE FROM keygen WHERE work_id = ?", workId)
	return err
}

func (d *SqlDatabase) LoadPresign(presignIds []string) ([]*presign.LocalPresignData, error) {
	// 1. Construct the query
	questions := getQueryQuestionMark(1, len(presignIds))

	query := "SELECT presign_output FROM presign WHERE presign_id IN " + questions + " ORDER BY created_time DESC"

	// Execute the query
	interfaceArr := make([]interface{}, len(presignIds))
	for i, s := range presignIds {
		interfaceArr[i] = s
	}

	rows, err := d.db.Query(query, interfaceArr...)
	if err != nil {
		return nil, err
	}

	// 2. Scan every rows and save it to loaded array
	results := make([]*presign.LocalPresignData, 0)
	for rows.Next() {
		var bz []byte
		if err := rows.Scan(&bz); err != nil {
			log.Error("Cannot unmarshall data", err)
			return nil, err
		}

		data := presign.LocalPresignData{}
		if err := json.Unmarshal(bz, &data); err != nil {
			log.Error("Cannot unmarshall data", err)
			return nil, err
		}

		results = append(results, &data)
	}

	return results, nil
}

func (d *SqlDatabase) UpdatePresignStatus(presignIds []string) error {
	presignString := getQueryQuestionMark(1, len(presignIds))
	query := fmt.Sprintf(
		"UPDATE status FROM presign SET status = %s WHERE presign_id IN (%s)",
		PresignStatusUsed,
		presignString,
	)

	interfaceArr := make([]interface{}, len(presignIds))
	for i, presignId := range presignIds {
		interfaceArr[i] = presignId
	}

	_, err := d.db.Exec(query, interfaceArr...)
	return err
}

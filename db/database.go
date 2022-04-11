package db

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sisu-network/dheart/core/config"
	p2ptypes "github.com/sisu-network/dheart/p2p/types"
	"github.com/sisu-network/dheart/utils"
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
	LoadKeygenData(keyType string) (*keygen.LocalPartySaveData, error)

	SavePresignData(workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error
	GetAvailablePresignShortForm() ([]string, []string, error) // Returns presignIds, pids, error

	LoadPresign(presignIds []string) ([]*presign.LocalPresignData, error)
	LoadPresignStatus(presignIds []string) ([]string, error)
	UpdatePresignStatus(presignIds []string) error

	SavePeers([]*p2ptypes.Peer) error
	LoadPeers() []*p2ptypes.Peer
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

	var err error
	var database *sql.DB
	// TODO: This is a temporary fix to run local docker. The correct fix is to redo the entire
	// life cycle of Sisu and dheart.
	for i := 0; i < 5; i++ {
		// Connect to the db with retry
		log.Verbose("Attempt number ", i+1)
		database, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/", username, password, host, d.config.Port))
		if err == nil {
			break
		}
		time.Sleep(time.Second * 3)
	}

	if err != nil {
		log.Error("All DB connection retry failed")
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

	// Write the migrations to a temporary directory
	// so they don't need to be managed out of band from the dheart binary.
	migrationDir, err := MigrationsTempDir()
	if err != nil {
		return fmt.Errorf("failed to create temporary directory for migrations: %w", err)
	}
	defer os.RemoveAll(migrationDir)

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationDir,
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
	defer rows.Close()

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

	pidString := utils.GetPidString(pids)
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
	defer rows.Close()

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
		return nil, nil
	}

	return result, nil
}

func (d *SqlDatabase) SavePresignData(workId string, pids []*tss.PartyID, presignOutputs []*presign.LocalPresignData) error {
	if len(presignOutputs) == 0 {
		return nil
	}

	pidString := utils.GetPidString(pids)

	// Constructs multi-insert query to do all insertion in 1 query.
	query := "INSERT INTO presign (presign_id, work_id, pids_string, batch_index, status, presign_output) VALUES "
	query = query + getQueryQuestionMark(len(presignOutputs), 6)

	params := make([]interface{}, 0)
	for i, output := range presignOutputs {
		bz, err := json.Marshal(output)
		if err != nil {
			return err
		}

		presignId := fmt.Sprintf("%s-%d", workId, i)

		params = append(params, presignId)
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
	defer rows.Close()

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
	defer rows.Close()

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

func (d *SqlDatabase) LoadPresignStatus(presignIds []string) ([]string, error) {
	questions := getQueryQuestionMark(1, len(presignIds))

	query := "SELECT status FROM presign WHERE presign_id IN " + questions + " ORDER BY created_time DESC"

	// Execute the query
	interfaceArr := make([]interface{}, len(presignIds))
	for i, s := range presignIds {
		interfaceArr[i] = s
	}

	rows, err := d.db.Query(query, interfaceArr...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 2. Scan every rows and save it to loaded array
	results := make([]string, 0)
	for rows.Next() {
		var status string
		if err := rows.Scan(status); err != nil {
			log.Error("Cannot load status", err)
			return nil, err
		}
		results = append(results, status)
	}

	log.Debug("load presign status result = ", results)
	return results, nil
}

func (d *SqlDatabase) UpdatePresignStatus(presignIds []string) error {
	presignString := getQueryQuestionMark(1, len(presignIds))
	query := fmt.Sprintf(
		"UPDATE presign SET status = ? WHERE presign_id IN %s",
		presignString,
	)

	interfaceArr := make([]interface{}, len(presignIds)+1)
	interfaceArr[0] = PresignStatusUsed
	for i, presignId := range presignIds {
		interfaceArr[i+1] = presignId
	}

	log.Verbose("UpdatePresignStatus: query = ", query)

	_, err := d.db.Exec(query, interfaceArr...)
	return err
}

func (d *SqlDatabase) SavePeers(peers []*p2ptypes.Peer) error {
	query := "INSERT INTO peers (`address`, pubkey, pubkey_type) VALUES "
	query = query + getQueryQuestionMark(len(peers), 3)

	_, err := d.db.Exec(query)
	if err != nil {
		return err
	}

	return nil
}

func (d *SqlDatabase) LoadPeers() []*p2ptypes.Peer {
	peers := make([]*p2ptypes.Peer, 0)
	query := fmt.Sprintf("SELECT `address`, pubkey, pubkey_type FROM peers")

	rows, err := d.db.Query(query)
	if err != nil {
		log.Error("failed to load peers")
		return peers
	}
	defer rows.Close()

	for rows.Next() {
		var nullableAddress, nullablePubkey, nullableKeytype sql.NullString

		if err := rows.Scan(&nullableAddress, &nullablePubkey, &nullableKeytype); err != nil {
			log.Error("LoadPeers: Cannot unmarshall data", err)
			continue
		}

		peer := &p2ptypes.Peer{
			Address:    nullableAddress.String,
			PubKey:     nullablePubkey.String,
			PubKeyType: nullablePubkey.String,
		}
		peers = append(peers, peer)
	}

	return peers
}

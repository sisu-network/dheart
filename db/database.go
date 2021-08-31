package db

import (
	"database/sql"
	"fmt"

	_ "github.com/go-sql-driver/mysql"
	"github.com/golang-migrate/migrate"
	"github.com/golang-migrate/migrate/database/mysql"
	_ "github.com/golang-migrate/migrate/source/file"
	"github.com/sisu-network/dheart/utils"
	"github.com/sisu-network/tss-lib/ecdsa/presign"
	"github.com/sisu-network/tss-lib/tss"
)

type Database interface {
	Init() error

	FindPresignDataByPids(count int, pids tss.SortedPartyIDs) []*presign.LocalPresignData
}

type SqlDbConfig struct {
	Port          int
	Host          string
	Username      string
	Password      string
	Schema        string
	MigrationPath string
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
	config *SqlDbConfig
}

func NewDatabase(config *SqlDbConfig) Database {
	return &SqlDatabase{
		config: config,
	}
}

func (database *SqlDatabase) FindPresignDataByPids(count int, pids tss.SortedPartyIDs) []*presign.LocalPresignData {
	return nil
}

func (d *SqlDatabase) Connect() error {
	host := d.config.Host
	if host == "" {
		return fmt.Errorf("DB host cannot be empty")
	}

	username := d.config.Username
	password := d.config.Password
	schema := d.config.Schema

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
	utils.LogInfo("Db is connected successfully")
	return nil
}

func (d *SqlDatabase) DoMigration() error {
	driver, err := mysql.WithInstance(d.db, &mysql.Config{})
	if err != nil {
		return err
	}

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
		utils.LogError("Failed to connect to DB. Err =", err)
		return err
	}

	err = d.DoMigration()
	if err != nil {
		return err
	}

	return nil
}

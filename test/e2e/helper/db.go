package helper

import (
	"database/sql"
	"fmt"

	"github.com/sisu-network/dheart/core/config"
	"github.com/sisu-network/dheart/db"
)

func ResetDb(index int) {
	ResetDbAtPort(index, 3306)
}

// TODO: Replace this with in-memory db
func ResetDbAtPort(index int, port int) {
	// reset the dev db
	database, err := sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", "root", "password", "0.0.0.0", port, fmt.Sprintf("dheart%d", index)))
	if err != nil {
		panic(err)
	}
	defer database.Close()

	database.Exec("TRUNCATE TABLE keygen")
	database.Exec("TRUNCATE TABLE presign")
}

func GetInMemoryDb(index int) db.Database {
	dbConfig := config.GetLocalhostDbConfig()
	dbConfig.Schema = fmt.Sprintf("dheart%d", index)
	dbConfig.InMemory = true

	dbInstance := db.NewDatabase(&dbConfig)

	err := dbInstance.Init()
	if err != nil {
		panic(err)
	}

	return dbInstance
}

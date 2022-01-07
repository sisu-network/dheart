package helper

import (
	"database/sql"
	"fmt"
)

func ResetDb(index int) {
	ResetDbAtPort(index, 3306)
}

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

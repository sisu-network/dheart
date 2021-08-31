package main

import "github.com/sisu-network/dheart/db"

func main() {
	config := &db.SqlDbConfig{
		Port:          3306,
		Host:          "localhost",
		Username:      "root",
		Password:      "password",
		Schema:        "dheart",
		MigrationPath: "file://../../../db/migrations/",
	}

	database := db.NewDatabase(config)
	err := database.Init()
	if err != nil {
		panic(err)
	}
}

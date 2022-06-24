package helper

import "github.com/joho/godotenv"

func LoadConfigEnv(filenames ...string) {
	err := godotenv.Load(filenames...)
	if err != nil {
		panic(err)
	}
}

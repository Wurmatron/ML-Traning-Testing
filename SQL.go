package main

import (
	"database/sql"
	"fmt"
	_ "github.com/lib/pq"
	"github.com/spf13/viper"
)

var dbConfig viper.Viper

func readDatabaseConfig() viper.Viper {
	dbConfig := viper.New()
	dbConfig.SetConfigName("database")
	dbConfig.SetConfigType("json")
	dbConfig.AddConfigPath(BaseDir)
	// Set Defaults
	dbConfig.SetDefault("host", "localhost")
	dbConfig.SetDefault("port", "5432")
	dbConfig.SetDefault("user", "crypto")
	dbConfig.SetDefault("password", "password")
	dbConfig.SetDefault("dbName", "Crypto")
	// Watch for updates
	// Read config
	if err := dbConfig.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			dbConfig.SafeWriteConfig()
		} else {
			panic(err)
		}
	}
	return *dbConfig
}

func ConnectDB() *sql.DB {
	dbConfig = readDatabaseConfig()
	fmt.Print("Attempting to connect to DB,  ")
	sqlConnection, err := sql.Open("postgres", "postgres://"+dbConfig.GetString("user")+":"+dbConfig.GetString("password")+"@"+dbConfig.GetString("host")+":"+dbConfig.GetString("port")+"/"+dbConfig.GetString("dbName")+"?sslmode=disable")
	if err != nil {
		panic(err)
	}
	err = sqlConnection.Ping()
	if err == nil {
		fmt.Print("Connected")
		fmt.Println()
	} else {
		fmt.Print("Failed")
		fmt.Println()
		fmt.Println(err.Error())
	}
	return sqlConnection
}

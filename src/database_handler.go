package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
)

var db *sql.DB

func inits() {
	dbPath := "../malware_hashes.db"

	var err error

	// Check that the database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatal("Database file doesn't exist: ", dbPath)
	}

	//Open SQLite database
	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to connect to local sqlite database:", err)
	} else {
		fmt.Println("Successfully connected to local sqlite database.")
	}
}

func checkHash(hash string) string {
	var query string = `SELECT file_name, file_type_guess, signature, vtpercent, first_seen_utc, reporter
	FROM malware_hashes
	WHERE sha256_hash='` + hash + `';`

	row, error := db.Query(query)
	if error != nil {
		fmt.Println(error)
	}

	defer row.Close()

	var SQLResult string
	for row.Next() {
		var fileName string
		var fileType string
		var signature string
		var vtpercent string
		var firstSeen string
		var reporter string
		if error := row.Scan(&fileName, &fileType, &signature, &vtpercent, &firstSeen, &reporter); error != nil {
			log.Fatal(error)
		}

		SQLResult = SQLResult + fmt.Sprintf("File Name: %s\nFile Type: %s\nSigning Authority: %s\nVirusTotal Percent: %s\nFirst Seen: %s\nReporter: %s\n",
			fileName, fileType, signature, vtpercent, firstSeen, reporter)
	}

	return SQLResult
}

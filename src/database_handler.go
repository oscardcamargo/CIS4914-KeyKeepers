package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"os"
	"sort"
	"time"
)

var DB_PATH = "../malware_hashes.db"
var DB_FOLDER = "../"
var TABLE_NAME = "malware_hashes"

var db *sql.DB
var databaseLines []Range // Used to track which lines this node has in its database.
var exportDBPath string

type Range struct {
	start int
	end   int
}

var CREATE_TABLE_STATEMENT = `CREATE TABLE "malware_hashes" (
    "ID" INTEGER,
    "first_seen_utc" TEXT,
    "sha256_hash" TEXT,
    "md5_hash" TEXT,
    "sha1_hash" TEXT,
    "reporter" TEXT,
    "file_name" TEXT,
    "file_type_guess" TEXT,
    "mime_type" TEXT,
    "signature" TEXT,
    "clamav" TEXT,
    "vtpercent" TEXT,
    "imphash" TEXT,
    "ssdeep" TEXT,
    "tlsh" TEXT,
    "alt_name1" TEXT,
    "alt_name2" TEXT,
    "alt_name3" TEXT,
    "alt_name4" TEXT,
    "alt_name5" TEXT,
    "alt_name6" TEXT,
    "alt_name7" TEXT,
    "alt_name8" TEXT,
    "alt_name9" TEXT,
    "alt_name10" TEXT,
    "alt_name11" TEXT,
    "alt_name12" TEXT,
    "alt_name13" TEXT
);`

func init() {
	var err error

	// Check that the database exists
	if _, err := os.Stat(DB_PATH); os.IsNotExist(err) {
		fmt.Printf("Database file %v doesn't exist. Creating a new one.", DB_PATH)
		databaseLines = append(databaseLines, Range{start: 0, end: 0})

		_, err = db.Exec(CREATE_TABLE_STATEMENT)
		if err != nil {
			log.Fatal("Failed to create database table.")
		}
	}

	//Open SQLite database
	db, err = sql.Open("sqlite3", DB_PATH)
	if err != nil {
		log.Fatal(err)
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("Failed to connect to local sqlite database:", err)
	} else {
		fmt.Println("Successfully connected to local sqlite database.")
	}

	getMax := fmt.Sprintf("SELECT MAX(ID) FROM %v", TABLE_NAME)
	getMin := fmt.Sprintf("SELECT MIN(ID) FROM %v", TABLE_NAME)
	var minID, maxID = 0, 0 // These will stay 0 if there aren't lines in the database.
	err = db.QueryRow(getMin).Scan(&minID)
	if err != nil {
		log.Fatal("Error reading database:", err)
	}

	err = db.QueryRow(getMax).Scan(&maxID)
	if err != nil {
		log.Fatal("Error reading database:", err)
	}

	if minID == 0 || maxID == 0 {
		databaseLines = append(databaseLines, Range{start: 0, end: 0})
	}
	databaseLines = append(databaseLines, Range{start: minID, end: maxID})
	fmt.Printf("This node has lines %d - %d\n", minID, maxID)
}

func checkHash(hash string) string {
	var query = `SELECT file_name, file_type_guess, signature, vtpercent, first_seen_utc, reporter
	FROM malware_hashes
	WHERE sha256_hash='` + hash + `';`

	row, err := db.Query(query)
	if err != nil {
		fmt.Println(err)
	}

	defer func(row *sql.Rows) {
		err := row.Close()
		if err != nil {

		}
	}(row)

	var SQLResult string
	for row.Next() {
		var fileName string
		var fileType string
		var signature string
		var vtpercent string
		var firstSeen string
		var reporter string
		if err := row.Scan(&fileName, &fileType, &signature, &vtpercent, &firstSeen, &reporter); err != nil {
			log.Fatal(err)
		}

		SQLResult = SQLResult + fmt.Sprintf("File Name: %s\nFile Type: %s\nSigning Authority: %s\nVirusTotal Percent: %s\nFirst Seen: %s\nReporter: %s\n",
			fileName, fileType, signature, vtpercent, firstSeen, reporter)
	}

	return SQLResult
}

// TODO: This needs unit tests
// Saves the range (from start to end) of database lines to a separate database to be transferred to another node.
// Returns name of the saved file on success. Empty string on failure.
// Call deleteExportDB when finished using the returned database to free up space.
func exportDatabaseLines(start int, end int) string {
	//Create a filename unique to this node so that nodes can differentiate between other received files.
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	newDBFileName := fmt.Sprintf("data_set_segment_%s_%d_%d.db", hostname, pid, time.Now().Unix())

	var err error
	newDB, err := sql.Open("sqlite3", DB_FOLDER+newDBFileName)
	if err != nil {
		log.Fatal(err)
	}

	defer func(newDB *sql.DB) {
		err := newDB.Close()
		if err != nil {

		}
	}(newDB)

	// Attach source database to the new one.
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%v' AS src", DB_PATH)
	_, err = newDB.Exec(attachQuery)
	if err != nil {
		log.Fatal("Error attaching source database to new database:", err)
	}

	// Get the CREATE TABLE statement from the main database.
	var createTableSQL string
	query := `SELECT sql FROM src.sqlite_master WHERE type='table' AND name='malware_hashes'`
	err = newDB.QueryRow(query, TABLE_NAME).Scan(&createTableSQL)
	if err != nil {
		log.Fatal("Error retrieving table schema:", err)
	}

	// Create the table in the destination database.
	_, err = newDB.Exec(createTableSQL)
	if err != nil {
		log.Fatal("Error creating table in destination database:", err)
	}

	// Copy the range of data into the exported db
	copyQuery := fmt.Sprintf("INSERT INTO %s SELECT * FROM src.%s WHERE ID >= ? AND ID <= ?", TABLE_NAME, TABLE_NAME)
	_, err = newDB.Exec(copyQuery, start, end)
	if err != nil {
		log.Fatal("Error copying data:", err)
	}

	// TODO: Does this need to return a Range struct to cover which lines were exported?
	// return Range{start: start, end: end}
	exportDBPath = DB_FOLDER + newDBFileName

	_, err = newDB.Exec(`DETACH DATABASE src`)
	if err != nil {
		log.Fatal("Error detaching database:", err)
	}

	return newDBFileName
}

// Adds all lines from another SQLite database, then deletes that file.
// Returns bool indicating success.
func importDatabase(externalDBName string, newRanges []Range) bool {
	var err error
	externalDBPath := DB_FOLDER + externalDBName

	newDB, err := sql.Open("sqlite3", externalDBPath)
	if err != nil {
		log.Fatal(err)
		return false
	}

	defer func(newDB *sql.DB) {
		err := newDB.Close()
		if err != nil {

		}
	}(newDB)

	// Attach new db to main one
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%v' AS imported", externalDBPath)
	_, err = db.Exec(attachQuery)
	if err != nil {
		log.Fatal("Error attaching imported database to main database:", err)
		return false
	}

	// Copy data from imported db to main db.
	copyQuery := fmt.Sprintf("INSERT INTO %s SELECT * FROM imported.%s", TABLE_NAME, TABLE_NAME)
	_, err = db.Exec(copyQuery)
	if err != nil {
		log.Fatal("Error copying data:", err)
		return false
	}

	_, err = db.Exec(`DETACH DATABASE imported`)
	if err != nil {
		log.Fatal("Error detaching database:", err)
		return false
	}

	err = newDB.Close()
	if err != nil {
		log.Fatal("Error closing imported database:", err)
		return false
	}

	err = os.Remove(externalDBPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Failed to delete exported database: %w", err)
		return false
	}

	databaseLines = append(databaseLines, newRanges...)
	rangeConsolidation()
	return true
}

// TODO: This needs unit tests
// Deletes the range (from start to end) of database lines.
// Returns bool indicating success.
func deleteDatabaseLines(delStart int64, delEnd int64) bool {
	deleteQuery := fmt.Sprintf("DELETE FROM %v WHERE hash64 >= ? AND hash64 <= ?", TABLE_NAME)

	_, err := db.Exec(deleteQuery, delStart, delEnd)
	if err != nil {
		log.Fatal("Failed to delete lines from database: ", err)
		return false
	}

	for index, rng := range databaseLines {
		// Check if part of the given range overlaps with a registered range.
		if rng.start <= int(delStart) && rng.end >= int(delStart) {
			if rng.start == int(delStart) && rng.end == int(delEnd) {
				remove(databaseLines, index)
				break
			}

			rng.end = int(delStart)

		} else if rng.start <= int(delEnd) && rng.end >= int(delEnd) {
			rng.start = int(delEnd)
		}
	}

	return true
}

// TODO This needs unit tests
// Consolidates the ranges in databaseLines.
// If there are overlapping or adjacent ranges, it squishes them down into a single range.
func rangeConsolidation() {
	sort.Slice(databaseLines, func(i, j int) bool {
		return databaseLines[i].start < databaseLines[j].start
	})

	// If the two ranges overlap or are adjacent to each other. Consolidate them.
	// Iterates backwards so deleted elements don't cause problems.
	for index := len(databaseLines) - 2; index >= 0; index-- {
		if databaseLines[index].end >= databaseLines[index+1].start ||
			databaseLines[index].end == databaseLines[index+1].start-1 {
			// Merge ranges
			databaseLines[index].end = databaseLines[index+1].end
			// Remove databaseLines[index+1]
			databaseLines = remove(databaseLines, index+1)
		}
	}
}

// Call this when finished using the exported database from exportDatabaseLines().
func deleteExportDB() {
	err := os.Remove(exportDBPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Failed to delete exported database: %w", err)
	}
}

// Removes an index from a Range slice
func remove(slice []Range, index int) []Range {
	return append(slice[:index], slice[index+1:]...)
}

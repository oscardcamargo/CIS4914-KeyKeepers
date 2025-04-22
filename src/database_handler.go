package main

import (
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
	"log"
	"math/big"
	"os"
	"sort"
	"time"
)

var DB_PATH = "../malware_hashes.db"
var DB_FOLDER = "../"
var TABLE_NAME = "malware_hashes"

var db *sql.DB
var databaseLines []Range // Used to track which lines this node has in its database.

type Range struct {
	start int
	end   int
}

type BigRange struct {
	start string
	end   string
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

// TODO: implement a way for the first node to get the full db and subsequent nodes to create empty dbs
// configuration flag argument?
func dbInit() {
	var err error

	//Open SQLite database
	db, err = sql.Open("sqlite3", DB_PATH)
	if err != nil {
		log.Fatal("[DATABASE] Failed to open database on init:", err)
	}

	// Check that the database exists
	if _, err := os.Stat(DB_PATH); os.IsNotExist(err) {
		fmt.Printf("[DATABASE] Database file %v doesn't exist. Creating a new one.\n", DB_PATH)
		databaseLines = append(databaseLines, Range{start: 0, end: 0})

		_, err = db.Exec(CREATE_TABLE_STATEMENT)
		if err != nil {
			log.Fatal("[DATABASE] Failed to create database table on init:", err)
		}
	}

	err = db.Ping()
	if err != nil {
		log.Fatal("[DATABASE] Failed to connect to local sqlite database:", err)
	} else {
		fmt.Println("Successfully connected to local sqlite database.")
	}

	//create the indices
	indexString := fmt.Sprintf(`CREATE UNIQUE INDEX "index_sha1" ON "%v" ("sha1_hash" ASC);`, TABLE_NAME)
	_, err = db.Exec(indexString)
	if err != nil {
		log.Fatal("[DATABASE] Failed to create an index on the sha1_hash column: ", err)
	} else {
		fmt.Println("Successfully created the sha1_hash index.")
	}

	indexString = fmt.Sprintf(`CREATE UNIQUE INDEX "index_ID" ON "%v" ("ID" ASC);`, TABLE_NAME)
	_, err = db.Exec(indexString)
	if err != nil {
		log.Fatal("[DATABASE] Failed to create and index on the ID column.")
	} else {
		fmt.Println("Successfully created the ID index.")
	}

	getMax := fmt.Sprintf("SELECT MAX(ID) FROM %v", TABLE_NAME)
	getMin := fmt.Sprintf("SELECT MIN(ID) FROM %v", TABLE_NAME)
	var minID, maxID = 0, 0                // These will stay 0 if there aren't lines in the database.
	var minIDNull, maxIDNull sql.NullInt64 // If there are no lines in the database, these variables can handle the 'null' that it sends back.
	err = db.QueryRow(getMin).Scan(&minIDNull)
	if err != nil {
		log.Fatal("[DATABASE] Error reading main database on init:", err)
	}
	err = db.QueryRow(getMax).Scan(&maxIDNull)
	if err != nil {
		log.Fatal("[DATABASE]Error reading database on init:", err)
	}
	if minIDNull.Valid {
		minID = int(minIDNull.Int64)
	}
	if maxIDNull.Valid {
		maxID = int(maxIDNull.Int64)
	}

	if minID == 0 && maxID == 0 {
		databaseLines = append(databaseLines, Range{start: 0, end: 0})
		return
	}
	databaseLines = append(databaseLines, Range{start: minID, end: maxID})
}

func checkHash(hash string) (string, error) {
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
			fmt.Printf("[DATABASE] Error closing row: %v\n", err.Error())
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
			fmt.Printf("[DATABASE] Error scanning row: %v\n", err.Error())
			return "", err
		}

		SQLResult = SQLResult + fmt.Sprintf("File Name: %s\nFile Type: %s\nSigning Authority: %s\nVirusTotal Percent: %s\nFirst Seen: %s\nReporter: %s\n",
			fileName, fileType, signature, vtpercent, firstSeen, reporter)
	}

	return SQLResult, nil
}

// Saves the range (from start to end) of database lines to a separate database to be transferred to another node.
// Returns name of the saved file on success. Empty string on failure.
// Call deleteExportDB when finished using the returned database to free up space.
func exportDatabaseLines(lineSlice []Range) (string, error) {
	// Check to make sure the ranges are in the database
	fmt.Println("In export...")
	for _, rng := range lineSlice {
		lineStart := rng.start
		lineEnd := rng.end
		if !rangeInDatabase(lineStart, lineEnd) {
			fmt.Printf("[DATABASE]: Error: Initiated transfer for line(s) that aren't in the database.\n")
			return "", errors.New("error: Initiated transfer for line(s) that aren't in the database")
		}
	}

	//Create a filename unique to this node so that nodes can differentiate between other received files.
	hostname, _ := os.Hostname()
	pid := os.Getpid()
	newDBFileName := fmt.Sprintf("data_set_segment_%s_%d_%d.db", hostname, pid, time.Now().UnixNano())

	var err error
	newDB, err := sql.Open("sqlite3", DB_FOLDER+newDBFileName)
	if err != nil {
		fmt.Printf("[DATABASE]: Error opening database file: %v\n", err.Error())
		return "", err
	}

	defer func(newDB *sql.DB) {
		closeDB(newDB)
	}(newDB)

	// Attach source database to the new one.
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%v' AS src", DB_PATH)
	_, err = newDB.Exec(attachQuery)
	if err != nil {
		log.Printf("[DATABASE] Error attaching source database to new database: %v\n", err.Error())
		return "", err
	}

	// Get the CREATE TABLE statement from the main database.
	var createTableSQL string
	query := `SELECT sql FROM src.sqlite_master WHERE type='table' AND name='malware_hashes'`
	err = newDB.QueryRow(query, TABLE_NAME).Scan(&createTableSQL)
	if err != nil {
		log.Printf("[DATABASE] Error retrieving table schema: %v\n", err.Error())
		return "", err
	}

	// Create the table in the destination database.
	_, err = newDB.Exec(createTableSQL)
	if err != nil {
		log.Printf("[DATABASE] Error creating table in destination database: %v\n", err.Error())
		return "", err
	}
	// OPTIMIZATION: adjusting pragma settings for faster bulk operations
	_, err = newDB.Exec("PRAGMA synchronous = OFF;")
	if err != nil {
		log.Printf("[DATABASE] Error setting synchronous OFF: %v\n", err)
	}

	_, err = newDB.Exec("PRAGMA journal_mode = MEMORY;")
	if err != nil {
		log.Printf("[DATABASE] Error setting journal_mode MEMORY: %v\n", err)
	}
	// OPTIMIZATION: begin transaction
	_, err = newDB.Exec("BEGIN TRANSACTION")
	if err != nil {
		return "", err
	}
	// Copy the range of data into the exported db
	copyQuery := fmt.Sprintf("INSERT INTO %s SELECT * FROM src.%s WHERE ID >= ? AND ID <= ?", TABLE_NAME, TABLE_NAME)
	for _, rng := range lineSlice {
		_, err = newDB.Exec(copyQuery, rng.start, rng.end)
		if err != nil {
			log.Printf("[DATABASE] Error copying data: %v\n", err.Error())

			return "", err
		}
	}
	// OPTIMIZATION: end transaction & commit
	_, err = newDB.Exec("COMMIT")
	if err != nil {
		return "", err
	}

	_, err = newDB.Exec(`DETACH DATABASE src`)
	if err != nil {
		log.Printf("[DATABASE] Error detaching database: %v\n", err.Error())
		return "", err
	}

	fmt.Println("Out export...")

	return newDBFileName, nil
}

// Adds all lines from another SQLite database, then deletes that file.
// Returns bool indicating success.
func importDatabase(externalDBName string, newRanges []Range) bool {
	var err error
	externalDBPath := DB_FOLDER + externalDBName

	newDB, err := sql.Open("sqlite3", externalDBPath)
	if err != nil {
		fmt.Printf("[DATABASE]: Error opening database file: %v\n", err.Error())
		return false
	}

	// Attach new db to main one
	attachQuery := fmt.Sprintf("ATTACH DATABASE '%v' AS imported", externalDBPath)
	_, err = db.Exec(attachQuery)
	if err != nil {
		log.Printf("[DATABASE] Error attaching imported database to main database: %v\n", err.Error())
		return false
	}

	// Copy data from imported db to main db, but only the lines that don't already exist.
	copyQuery := fmt.Sprintf(`INSERT INTO %s 
SELECT * 
FROM imported.%s 
WHERE NOT EXISTS (
	SELECT 1 FROM %s WHERE %s.ID = imported.%s.ID
);`, TABLE_NAME, TABLE_NAME, TABLE_NAME, TABLE_NAME, TABLE_NAME)
	_, err = db.Exec(copyQuery)
	if err != nil {
		log.Printf("[DATABASE] Error copying data: %v\n", err.Error())
		return false
	}

	_, err = db.Exec(`DETACH DATABASE imported`)
	if err != nil {
		log.Printf("[DATABASE] Error detaching database: %v\n", err.Error())
		return false
	}

	closeDB(newDB)
	deleteDB(externalDBName)

	databaseLines = append(databaseLines, newRanges...)
	rangeConsolidation()
	return true
}

// Deletes the range (from start to end) of database lines.
// Returns bool indicating success.
// maybe repurpose to work with
func deleteDatabaseLines(delRange Range) bool {
	deleteQuery := fmt.Sprintf("DELETE FROM %v WHERE ID >= ? AND ID <= ?", TABLE_NAME)

	_, err := db.Exec(deleteQuery, delRange.start, delRange.end)
	if err != nil {
		log.Printf("[DATABASE] Failed to delete lines from database: %v\n", err.Error())
		return false
	}

	for index := 0; index < len(databaseLines); index++ {
		rng := databaseLines[index]

		// If delRange is fully within an existing range, split it into two
		if rng.start < delRange.start && rng.end > delRange.end {
			// Create a new range for the right-hand side
			newRange := Range{start: delRange.end + 1, end: rng.end}

			// Adjust the left-side range
			databaseLines[index].end = delRange.start - 1

			// Insert the new range after the adjusted left range
			databaseLines = append(databaseLines[:index+1], append([]Range{newRange}, databaseLines[index+1:]...)...)
		} else if rng.start >= delRange.start && rng.end <= delRange.end {
			// If it exactly matches or encompasses the range, remove the range
			databaseLines = append(databaseLines[:index], databaseLines[index+1:]...)
			index-- // Adjust the index after removal
		} else if rng.start < delRange.start &&
			rng.end <= delRange.end && rng.end >= delRange.start {
			// If the delete range extends past the right side, but still overlaps with the database range.
			// Trim right side
			databaseLines[index].end = delRange.start - 1
		} else if rng.start >= delRange.start && rng.start <= delRange.end &&
			rng.end > delRange.end {
			// If the delete range extends past the left side, but still overlaps with the database range.
			// Trim the left side
			databaseLines[index].start = delRange.end + 1
		}
	}

	return true
}

func deleteOtherHashes(keepRange BigRange) bool {
	var minHash, maxHash string
	err := db.QueryRow(fmt.Sprintf("SELECT MIN(sha1_hash) FROM %v", TABLE_NAME)).Scan(&minHash)
	if err != nil {
		log.Printf("[DATABASE] Failed to delete lines from database 0: %v\n", err.Error())
		return false
	}
	err = db.QueryRow(fmt.Sprintf("SELECT MAX(sha1_hash) FROM %v", TABLE_NAME)).Scan(&maxHash)
	if err != nil {
		log.Printf("[DATABASE] Failed to delete lines from database 1: %v\n", err.Error())
		return false
	}

	// minHashNum := new(big.Int)
	// minHashNum.SetString(minHash, 16)
	// maxHashNum := new(big.Int)
	// maxHashNum.SetString(maxHash, 16)
	startNum := new(big.Int)
	startNum.SetString(keepRange.start, 16)
	endNum := new(big.Int)
	endNum.SetString(keepRange.end, 16)
	// goal is to keep (P, N]
	// keepRange.start = P
	// keepRange.end = N
	// P < N
	if startNum.Cmp(endNum) == -1 {
		// delete [min, P]
		deleteQuery := fmt.Sprintf("DELETE FROM %v WHERE sha1_hash >= ? AND sha1_hash <= ?", TABLE_NAME)
		_, err = db.Exec(deleteQuery, minHash, keepRange.start)
		if err != nil {
			log.Printf("[DATABASE] Failed to delete lines from database: %v\n", err.Error())
			return false
		}
		// delete (N, max]
		deleteQuery = fmt.Sprintf("DELETE FROM %v WHERE sha1_hash > ? AND sha1_hash <= ?", TABLE_NAME)
		_, err = db.Exec(deleteQuery, keepRange.end, maxHash)
		if err != nil {
			log.Printf("[DATABASE] Failed to delete lines from database: %v\n", err.Error())
			return false
		}
	} else if startNum.Cmp(endNum) == 1 {
		// N < P
		// delete (N, P]
		deleteQuery := fmt.Sprintf("DELETE FROM %v WHERE sha1_hash > ? AND sha1_hash <= ?", TABLE_NAME)
		_, err = db.Exec(deleteQuery, keepRange.end, keepRange.start)
		if err != nil {
			log.Printf("[DATABASE] Failed to delete lines from database: %v\n", err.Error())
			return false
		}
	} else {
		return false
	}

	// for index := 0; index < len(databaseLines); index++ {
	// 	rng := databaseLines[index]

	// 	// If delRange is fully within an existing range, split it into two
	// 	if rng.start < delRange.start && rng.end > delRange.end {
	// 		// Create a new range for the right-hand side
	// 		newRange := Range{start: delRange.end + 1, end: rng.end}

	// 		// Adjust the left-side range
	// 		databaseLines[index].end = delRange.start - 1

	// 		// Insert the new range after the adjusted left range
	// 		databaseLines = append(databaseLines[:index+1], append([]Range{newRange}, databaseLines[index+1:]...)...)
	// 	} else if rng.start >= delRange.start && rng.end <= delRange.end {
	// 		// If it exactly matches or encompasses the range, remove the range
	// 		databaseLines = append(databaseLines[:index], databaseLines[index+1:]...)
	// 		index-- // Adjust the index after removal
	// 	} else if rng.start < delRange.start &&
	// 		rng.end <= delRange.end && rng.end >= delRange.start {
	// 		// If the delete range extends past the right side, but still overlaps with the database range.
	// 		// Trim right side
	// 		databaseLines[index].end = delRange.start - 1
	// 	} else if rng.start >= delRange.start && rng.start <= delRange.end &&
	// 		rng.end > delRange.end {
	// 		// If the delete range extends past the left side, but still overlaps with the database range.
	// 		// Trim the left side
	// 		databaseLines[index].start = delRange.end + 1
	// 	}
	// }

	return true
}

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
func deleteDB(fileName string) {
	err := os.Remove(DB_FOLDER + fileName)
	if err != nil || errors.Is(err, os.ErrNotExist) {
		log.Printf("[DATABASE] Failed to delete exported database: %v\n", err.Error())
	}
}

// Removes an index from a Range slice
func remove(slice []Range, index int) []Range {
	return append(slice[:index], slice[index+1:]...)
}

// Opens the file name from the correct folder for reading.
func openFileRead(fileName string) (*os.File, error) {
	file, err := os.Open(DB_FOLDER + fileName)
	return file, err
}

// Opens the file name from the correct folder for writing.
func openFileWrite(fileName string) (*os.File, error) {
	file, err := os.OpenFile(DB_FOLDER+fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	return file, err
}

// Returns true if the whole of the given range is in one of the database's ranges. else, returns false.
func rangeInDatabase(lineStart int, lineEnd int) bool {
	for _, r := range databaseLines {
		if lineStart >= r.start && lineEnd <= r.end && lineStart <= lineEnd {
			return true
		}
	}
	return false
}

func closeDB(thisDB *sql.DB) {
	err := thisDB.Close()
	if err != nil {
		log.Printf("[DATABASE] Error closing database: %v\n", err)
	}
}

// Returns a slice of database lines.
func getLineRange() []Range {
	return databaseLines
}

// Used for getting the IDs of selected SHA1 hashes
func getRowsInHashRange(startHash, endHash string) ([]int, error) {
	//fmt.Printf("Attempting to send hashes %s - %s\n", startHash, endHash)
	query := fmt.Sprintf(`SELECT ID FROM %s WHERE sha1_hash >= ? AND sha1_hash <= ?`, TABLE_NAME)

	rows, err := db.Query(query, startHash, endHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int
	for rows.Next() {
		var id int
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}

	return ids, nil
}

// Used for batching adjacent IDs based on selected SHA1 hashes
func batchIDs(ids []int) []Range {
	if len(ids) == 0 {
		return nil
	}

	sort.Ints(ids) // Ensure IDs are sorted.

	var ranges []Range
	start := ids[0]
	end := ids[0]

	for i := 1; i < len(ids); i++ {
		if ids[i] == end+1 {
			// Adjacent ID, extend the current range
			end = ids[i]
		} else {
			// Non-adjacent, save previous range and start new one
			ranges = append(ranges, Range{start: start, end: end})
			start, end = ids[i], ids[i]
		}
	}

	// Append the final range
	ranges = append(ranges, Range{start: start, end: end})
	return ranges
}
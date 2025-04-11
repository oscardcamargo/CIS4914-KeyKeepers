package main

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

/*
Note that the correct functionality of these functions implies that init() is also working correctly
This is important since init is not represented explicitly by any unit test
*/

/*
Tests the function checkHash(hash string) (string, error)
Hashes were manually selected from the database's csv file
The output of hashResult was verified manually
*/
func TestCheckHash(t *testing.T) {
	testHash := `163c47c4e45116fc184c41428ec0cbbd60d2bdacb8df759d6f744f252d6c1305`
	hashResult, err := checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `727c0e92f5ab308c643440a2dc6a1534cb5a8345a83cbaddbd4d6b4e38d7d533`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `b79c66c6982c75deccdac850f7fc0ac60449eebb03ee85fd805053aa706adc63`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `3e767412cd42ef86d5b5a8d67d45bcf7ea55e419c8f73c3cbd60570f5ecf0fb0`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `863927e0d028fa74b2cd79d7da2f8c21967aa5efe057789dee4ed75559f6a9c8`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `8782b02593148ae0c8230c2e3a704ab26f08fac222c9204c94c35eca12cff403`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `9b2c3cbefa2354d3f48d52520b6cb822e982d97b0be484cb06fbd5cb77f45371`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4dda59b51d51f18c9071eb07a730ac4548e36e0d14dbf00e886fc155e705eeef`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `70572dfc461f571e9cc1cb47e66504505a072579134e851f6b0e77a5714f98a2`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `value_not_found`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `distributed_hash_table`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)
}

/*
Tests the function rangeInDatabase(lineStart int, lineEnd int) bool
Note that the range of the entire database is [1, 884469]
Thus, any valid range is a subset of [1, 884469]
*/
func TestRangeInDatabase(t *testing.T) {
	result := rangeInDatabase(1, 884469)
	assert.Truef(t, result, "Expected rangeInDatabase(1, 884469) to return true")

	result = rangeInDatabase(234567, 765432)
	assert.Truef(t, result, "Expected rangeInDatabase(234567, 765432) to return true")

	result = rangeInDatabase(1, 10)
	assert.Truef(t, result, "Expected rangeInDatabase(1, 10) to return true")

	result = rangeInDatabase(884459, 884469)
	assert.Truef(t, result, "Expected rangeInDatabase(884459, 884469) to return true")

	result = rangeInDatabase(1, 1)
	assert.Truef(t, result, "Expected rangeInDatabase(1, 1) to return true")

	result = rangeInDatabase(0, 884469)
	assert.Falsef(t, result, "Expected rangeInDatabase(0, 884469) to return false")

	result = rangeInDatabase(1, 884470)
	assert.Falsef(t, result, "Expected rangeInDatabase(1, 884470) to return false")

	result = rangeInDatabase(1, 1000000)
	assert.Falsef(t, result, "Expected rangeInDatabase(1, 1000000) to return false")

	result = rangeInDatabase(-1, 884469)
	assert.Falsef(t, result, "Expected rangeInDatabase(-1, 884469) to return false")

	result = rangeInDatabase(-884469, 1)
	assert.Falsef(t, result, "Expected rangeInDatabase(-884469, 1) to return false")

	result = rangeInDatabase(1000000, 1000010)
	assert.Falsef(t, result, "Expected rangeInDatabase(1000000, 1000010) to return false")

	result = rangeInDatabase(10, 1)
	assert.Falsef(t, result, "Expected rangeInDatabase(10, 1) to return false")
}

/*
Tests the function deleteDB(fileName string)
Verifies that an exported database is deleted successfully
*/
func TestDeleteDB(t *testing.T) {
	lines := []Range{
		{start: 1, end: 5},
	}

	result, err := exportDatabaseLines(lines)
	assert.NoError(t, err)

	_, err = os.Stat("../" + result)
	assert.NoError(t, err, "Expected file: "+result)

	deleteDB(result)

	_, err = os.Stat("../" + result)
	assert.Error(t, err, "Database was not deleted")
}

/*
Tests the function exportDatabaseLines(lineSlice []Range) (string, error)
This focuses on testing that the exported database file is created correctly
*/
func TestExportDatabaseLines(t *testing.T) {
	lines := []Range{
		{start: 1, end: 5},
	}

	result, err := exportDatabaseLines(lines)
	assert.NoError(t, err)

	_, err = os.Stat("../" + result)
	assert.NoError(t, err, "Expected file: "+result)

	deleteDB(result)

	_, err = os.Stat("../" + result)
	assert.Error(t, err, "Database was not deleted")

	lines = []Range{
		{start: 1, end: 884469},
	}

	result, err = exportDatabaseLines(lines)
	assert.NoError(t, err)

	_, err = os.Stat("../" + result)
	assert.NoError(t, err, "Expected file: "+result)

	deleteDB(result)

	_, err = os.Stat("../" + result)
	assert.Error(t, err, "Database was not deleted")

	lines = []Range{
		{start: 0, end: 5},
	}

	result, err = exportDatabaseLines(lines)
	assert.Error(t, err)

	lines = []Range{
		{start: 100000, end: 884470},
	}

	result, err = exportDatabaseLines(lines)
	assert.Error(t, err)
}

/*
Tests the function deleteDatabaseLines(delRange Range) bool
Verifies that 5 lines are deleted from the database
To keep the database consistent we export and import the 5 lines

Also tests rangeConsolidation, exportDatabaseLines, importDatabaseLines, and checkHash
*/
func TestDeleteDatabaseLinesFirstFiveLines(t *testing.T) {
	lines := []Range{
		{start: 1, end: 5},
	}

	exportedDatabase, err := exportDatabaseLines(lines)
	assert.NoError(t, err)

	_, err = os.Stat("../" + exportedDatabase)
	assert.NoError(t, err, "Expected file: "+exportedDatabase)

	db_lines := getLineRange()

	assert.Equal(t, 1, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{1, 884469}, db_lines[0])

	deletedLines := deleteDatabaseLines(lines[0])
	assert.True(t, deletedLines, "Expected lines to be deleted successfully")

	db_lines = getLineRange()

	assert.Equal(t, 1, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{6, 884469}, db_lines[0])

	testHash := `163c47c4e45116fc184c41428ec0cbbd60d2bdacb8df759d6f744f252d6c1305`
	hashResult, err := checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `727c0e92f5ab308c643440a2dc6a1534cb5a8345a83cbaddbd4d6b4e38d7d533`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `b79c66c6982c75deccdac850f7fc0ac60449eebb03ee85fd805053aa706adc63`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `3e767412cd42ef86d5b5a8d67d45bcf7ea55e419c8f73c3cbd60570f5ecf0fb0`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `863927e0d028fa74b2cd79d7da2f8c21967aa5efe057789dee4ed75559f6a9c8`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	importedDatabase := importDatabase(exportedDatabase, lines)
	assert.True(t, importedDatabase, "Expected database to be imported successfully")

	db_lines = getLineRange()

	assert.Equal(t, 1, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{1, 884469}, db_lines[0])

	_, err = os.Stat("../" + exportedDatabase)
	assert.Error(t, err, "Database was not deleted")

	testHash = `163c47c4e45116fc184c41428ec0cbbd60d2bdacb8df759d6f744f252d6c1305`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `727c0e92f5ab308c643440a2dc6a1534cb5a8345a83cbaddbd4d6b4e38d7d533`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `b79c66c6982c75deccdac850f7fc0ac60449eebb03ee85fd805053aa706adc63`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `3e767412cd42ef86d5b5a8d67d45bcf7ea55e419c8f73c3cbd60570f5ecf0fb0`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `863927e0d028fa74b2cd79d7da2f8c21967aa5efe057789dee4ed75559f6a9c8`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)
}

/*
Tests the function deleteDatabaseLines(delRange Range) bool
Verifies that 3 lines are deleted from the database
To keep the database consistent we export and import the 3 lines
*/
func TestDeleteDatabaseLinesNextThreeLines(t *testing.T) {
	lines := []Range{
		{start: 6, end: 8},
	}

	testHash := `9ae288f4bf4e8888e9e86377699ef4edfec684c549d0abd5273328d7001489e7`
	hashResult, err := checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `705e7592c1f4cd0967671cbf2a732fa8c7e29bcc1a11e121dd7add2dcfdaccbf`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4a0063a2f3d69301ed13aed1734be629e39b4139602d4af162260503b544b854`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	exportedDatabase, err := exportDatabaseLines(lines)
	assert.NoError(t, err)

	_, err = os.Stat("../" + exportedDatabase)
	assert.NoError(t, err, "Expected file: "+exportedDatabase)

	db_lines := getLineRange()

	assert.Equal(t, 1, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{1, 884469}, db_lines[0])

	deletedLines := deleteDatabaseLines(lines[0])
	assert.True(t, deletedLines, "Expected lines to be deleted successfully")

	db_lines = getLineRange()

	assert.Equal(t, 2, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{1, 5}, db_lines[0])
	assert.Equal(t, Range{9, 884469}, db_lines[1])

	testHash = `9ae288f4bf4e8888e9e86377699ef4edfec684c549d0abd5273328d7001489e7`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `705e7592c1f4cd0967671cbf2a732fa8c7e29bcc1a11e121dd7add2dcfdaccbf`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `4a0063a2f3d69301ed13aed1734be629e39b4139602d4af162260503b544b854`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.Empty(t, hashResult)

	testHash = `3e767412cd42ef86d5b5a8d67d45bcf7ea55e419c8f73c3cbd60570f5ecf0fb0`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `863927e0d028fa74b2cd79d7da2f8c21967aa5efe057789dee4ed75559f6a9c8`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	importedDatabase := importDatabase(exportedDatabase, lines)
	assert.True(t, importedDatabase, "Expected database to be imported successfully")

	db_lines = getLineRange()

	assert.Equal(t, 1, len(db_lines), "Expected number of lines to match")
	assert.Equal(t, Range{1, 884469}, db_lines[0])

	_, err = os.Stat("../" + exportedDatabase)
	assert.Error(t, err, "Database was not deleted")

	testHash = `163c47c4e45116fc184c41428ec0cbbd60d2bdacb8df759d6f744f252d6c1305`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `727c0e92f5ab308c643440a2dc6a1534cb5a8345a83cbaddbd4d6b4e38d7d533`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `b79c66c6982c75deccdac850f7fc0ac60449eebb03ee85fd805053aa706adc63`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `9ae288f4bf4e8888e9e86377699ef4edfec684c549d0abd5273328d7001489e7`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `705e7592c1f4cd0967671cbf2a732fa8c7e29bcc1a11e121dd7add2dcfdaccbf`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)

	testHash = `4a0063a2f3d69301ed13aed1734be629e39b4139602d4af162260503b544b854`
	hashResult, err = checkHash(testHash)
	assert.NoError(t, err)
	assert.NotEmpty(t, hashResult)
}

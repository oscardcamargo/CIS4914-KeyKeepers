package main

import (
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCheckHash(t *testing.T) {
	testHash := `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	hashResult, err := checkHash(testHash)
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
}

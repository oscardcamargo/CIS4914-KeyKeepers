package main

import (
	"fmt"
	"time"
)

func main() {

	var testHash string = `4c3f9505b832a5a8bb22d5d339b1dfd4800d96d3ffec4a495fdc2274efa6601c`
	var hashResult = checkHash(testHash)

	fmt.Println(hashResult)

	// Pausing so that the docker container doesn't close right away.
	// This won't be needed when the program has logic to continue to run.
	time.Sleep(time.Duration(100) * time.Second)
}

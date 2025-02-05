package main

import (
	"fmt"
	"time"
)

func main() {
	fmt.Println("Based Go execution frfr, no cap and stuff.")

	// Pausing so that the docker container doesn't close right away.
	// This won't be needed when the program has logic to continue to run.
	time.Sleep(time.Duration(100) * time.Second)
}

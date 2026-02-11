package main

import (
	"log"
	"os"
)

func main() {
	os.Exit(0)
	log.Fatal("ok")

	func() {
		os.Exit(1) // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
	}()
}

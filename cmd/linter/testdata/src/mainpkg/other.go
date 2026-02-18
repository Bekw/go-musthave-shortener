package main

import (
	"log"
	"os"
)

func helper() {
	os.Exit(2)     // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
	log.Fatal("x") // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
}

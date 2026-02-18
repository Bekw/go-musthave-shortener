package a

import (
	"log"
	"os"
)

func F() {
	os.Exit(1)      // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
	log.Fatal("x")  // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
	log.Fatalf("x") // want "call to os\\.Exit or log\\.Fatal outside main function of main package"
	panic("x")      // want "panic is forbidden"
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/mroy31/go-video-daemon/internal/library"
)

var (
	inputFile = flag.String("input", "", "Path of the input file")
)

func main() {
	flag.Parse()

	info, err := library.Probe(*inputFile)
	if err != nil {
		fmt.Println("Error - unable to probe file: " + err.Error())
		os.Exit(2)
	}

	fmt.Print(info)
}

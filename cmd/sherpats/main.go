// Command sherpats reads documentation from a sherpa API ("sherpadoc")
// and outputs a documented typescript module that exports all functions
// and types referenced in that machine-readable documentation.
//
// Example:
//
// 	sherpadoc MyAPI >myapi.json
// 	sherpats < myapi.json > myapi.ts
package main

import (
	"flag"
	"log"
	"os"

	"github.com/mjl-/sherpats"
)

func check(err error, action string) {
	if err != nil {
		log.Fatalf("%s: %s\n", action, err)
	}
}

func main() {
	log.SetFlags(0)
	flag.Usage = func() {
		log.Println("usage: sherpats { API name | baseURL }")
		flag.PrintDefaults()
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Print("unexpected arguments")
		flag.Usage()
		os.Exit(2)
	}
	apiName := args[0]

	err := sherpats.Generate(os.Stdin, os.Stdout, apiName)
	check(err, "generating typescript client")
}

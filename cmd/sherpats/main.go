// Command sherpats reads documentation from a sherpa API ("sherpadoc")
// and outputs a documented typescript module, optionally wrapped in a namespace,
// that exports all functions and types referenced in that machine-readable
// documentation.
//
// Example:
//
//	sherpadoc MyAPI >myapi.json
//	sherpats -slices-nullable=true -nullable-optional=true -namespace myapi myapi < myapi.json > myapi.ts
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

	var opts sherpats.Options
	flag.StringVar(&opts.Namespace, "namespace", "", "namespace to enclose generated typescript in")
	flag.BoolVar(&opts.SlicesNullable, "slices-nullable", false, "generate nullable types in TypeScript for Go slices, to require TypeScript checks for null for slices")
	flag.BoolVar(&opts.NullableOptional, "nullable-optional", false, "for nullable types (include slices with -slices-nullable=true), generate optional fields in TypeScript and allow undefined as value")
	flag.Usage = func() {
		log.Println("usage: sherpats [flags] { api-path-elem | baseURL }")
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

	err := sherpats.Generate(os.Stdin, os.Stdout, apiName, opts)
	check(err, "generating typescript client")
}

// Command sherpats reads documentation from a sherpa API ("sherpadoc")
// and outputs a documented typescript module that exports all functions
// and types referenced in that machine-readable documentation.
//
// Example:
//
// 	sherpadoc MyAPI >myapi.json
// 	sherpats < myapi.json > myapi.ts
package main

import "github.com/mjl-/sherpats"

func main() {
	sherpats.Main()
}

package sherpats

import (
	"log"

	"bitbucket.org/mjl/sherpa"
)

func checkTypes(doc *sherpa.Doc) {
	types := map[string]struct{}{}

	var markTypes func(sec *sherpa.Doc)
	markTypes = func(sec *sherpa.Doc) {
		for _, t := range sec.Types {
			if _, ok := types[t.Name]; ok {
				log.Fatalf("duplicate type %q\n", t.Name)
			}
			types[t.Name] = struct{}{}
		}
		for _, subsec := range sec.Sections {
			markTypes(subsec)
		}
	}
	markTypes(doc)

	var checkType func(tokens []string)
	checkType = func(tokens []string) {
		if len(tokens) == 0 {
			return
		}
		t := tokens[0]
		switch t {
		case "nullable", "any", "bool", "int", "float", "string", "[]", "{}":
		default:
			_, ok := types[t]
			if !ok {
				log.Fatalf("referenced type %q does not exist\n", t)
			}
		}
		checkType(tokens[1:])
	}
	var checkSection func(sec *sherpa.Doc)
	checkSection = func(sec *sherpa.Doc) {
		for _, t := range sec.Types {
			for _, f := range t.Fields {
				checkType(f.Type)
			}
		}
		for _, subsec := range sec.Sections {
			checkSection(subsec)
		}
	}

	checkSection(doc)
}

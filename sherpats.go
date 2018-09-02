package sherpats

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"bitbucket.org/mjl/sherpa"
)

type sherpaType interface {
	TypescriptType() string
}

// baseType can be one of: "any", "bool", "int", "float", "string".
type baseType struct {
	Name string
}

// nullableType is: "nullable" <type>.
type nullableType struct {
	Type sherpaType
}

// arrayType is: "[]" <type>
type arrayType struct {
	Type sherpaType
}

// objectType is: "{}" <type>
type objectType struct {
	Value sherpaType
}

// identType is: [a-zA-Z][a-zA-Z0-9]*
type identType struct {
	Name string
}

func (t baseType) TypescriptType() string {
	switch t.Name {
	case "int", "float":
		return "number"
	default:
		return t.Name
	}
}

func (t nullableType) TypescriptType() string {
	return t.Type.TypescriptType() + " | null"
}

func (t arrayType) TypescriptType() string {
	return t.Type.TypescriptType() + "[]"
}

func (t objectType) TypescriptType() string {
	return fmt.Sprintf("{ [key: string]: %s }", t.Value.TypescriptType())
}

func (t identType) TypescriptType() string {
	return t.Name
}

func check(err error, action string) {
	if err != nil {
		log.Fatalf("%s: %s\n", action, err)
	}
}

// Main reads sherpadoc from stdin and writes a typescript module to
// stdout.  It requires exactly one parameter: a baseURL for the API,
// or the API name. If the value has no slash, it is an API name. In
// this case the baseURL is made by concatenating the base URL of the
// javascript location with the API name.
// It is a separate command so it can easily be vendored a repository.
func Main() {
	log.SetFlags(0)
	log.SetPrefix("sherpats: ")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s { API name | baseURL }\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(2)
	}
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		log.Print("unexpected arguments")
		flag.Usage()
		os.Exit(2)
	}
	apiName := args[0]

	var doc sherpa.Doc
	err := json.NewDecoder(os.Stdin).Decode(&doc)
	check(err, "parsing sherpadoc json from stdin")

	const sherpadocVersion = 1
	if doc.Version != sherpadocVersion {
		log.Fatalf("unexpected sherpadoc version %d, expected %d\n", doc.Version, sherpadocVersion)
	}

	// Use bytes.Buffer, writes won't fail. We do one big write at the end. Modules won't quickly become too big to fit in memory.
	out := &bytes.Buffer{}

	// Check all referenced types exist.
	checkTypes(&doc)

	var generateTypes func(sec *sherpa.Doc)
	generateTypes = func(sec *sherpa.Doc) {
		for _, t := range sec.Types {
			for _, line := range commentLines(t.Text) {
				fmt.Fprintf(out, "// %s\n", line)
			}
			fmt.Fprintf(out, "export interface %s {\n", t.Name)
			for _, f := range t.Fields {
				lines := commentLines(f.Text)
				if len(lines) > 1 {
					for _, line := range lines {
						fmt.Fprintf(out, "\t// %s\n", line)
					}
				}
				what := fmt.Sprintf("field %s for type %s", f.Name, t.Name)
				fmt.Fprintf(out, "\t%s: %s", f.Name, typescriptType(what, f.Type))
				if len(lines) == 1 {
					fmt.Fprintf(out, "   // %s", lines[0])
				}
				fmt.Fprintln(out, "")
			}
			fmt.Fprintf(out, "}\n\n")
		}
		for _, subsec := range sec.Sections {
			generateTypes(subsec)
		}
	}
	generateTypes(&doc)

	var generateFunctionTypes func(sec *sherpa.Doc)
	generateFunctionTypes = func(sec *sherpa.Doc) {
		for _, typ := range sec.Types {
			type typeField struct {
				Name string
				Type []string
			}
			tstypes := []typeField{}
			for _, f := range typ.Fields {
				tstypes = append(tstypes, typeField{f.Name, f.Type})
			}
			jst, err := json.Marshal(tstypes)
			check(err, "marshal type to json")
			fmt.Fprintf(out, "\t%s: %s,\n", typ.Name, jst)
		}
		for _, subsec := range sec.Sections {
			generateFunctionTypes(subsec)
		}
	}
	fmt.Fprintln(out, "export const _types: { [typeName: string]: _type } = {")
	generateFunctionTypes(&doc)
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out, "")

	var generateFunctions func(sec *sherpa.Doc)
	generateFunctions = func(sec *sherpa.Doc) {
		for _, fn := range sec.Functions {
			whatParam := "pararameter for " + fn.Name
			paramTypes := []string{}
			paramNames := []string{}
			sherpaParamTypes := [][]string{}
			for _, p := range fn.Params {
				v := fmt.Sprintf("%s: %s", p.Name, typescriptType(whatParam, p.Type))
				paramTypes = append(paramTypes, v)
				paramNames = append(paramNames, p.Name)
				sherpaParamTypes = append(sherpaParamTypes, p.Type)
			}

			var returnType string
			switch len(fn.Return) {
			case 0:
				returnType = "void"
			case 1:
				what := "return type for " + fn.Name
				returnType = typescriptType(what, fn.Return[0].Type)
			default:
				var types []string
				what := "return type for " + fn.Name
				for _, t := range fn.Return {
					types = append(types, typescriptType(what, t.Type))
				}
				returnType = fmt.Sprintf("[%s]", strings.Join(types, ", "))
			}
			sherpaReturnTypes := [][]string{}
			for _, a := range fn.Return {
				sherpaReturnTypes = append(sherpaReturnTypes, a.Type)
			}

			sherpaParamTypesJSON, err := json.Marshal(sherpaParamTypes)
			check(err, "marshal sherpa param types")
			sherpaReturnTypesJSON, err := json.Marshal(sherpaReturnTypes)
			check(err, "marshal sherpa return types")

			fmt.Fprintf(out, "export const %s = (%s, options?: Options): Promise<%s> => { return _sherpaCall(options || {}, %s, %s, '%s', [%s]) as Promise<%s> }\n", fn.Name, strings.Join(paramTypes, ", "), returnType, sherpaParamTypesJSON, sherpaReturnTypesJSON, fn.Name, strings.Join(paramNames, ", "), returnType)
		}
	}
	generateFunctions(&doc)

	const findBaseURL = `(function() {
	let p = location.pathname
	if (p && p[p.length - 1] !== '/') {
		let l = location.pathname.split('/')
		l = l.slice(0, l.length - 1)
		p = '/' + l.join('/') + '/'
	}
	return location.protocol + '//' + location.host + p + 'API_NAME/'
})()`

	var apiJS string
	if strings.Contains(apiName, "/") {
		buf, err := json.Marshal(apiName)
		check(err, "marshal apiName")
		apiJS = string(buf)
	} else {
		apiJS = strings.Replace(findBaseURL, "API_NAME", apiName, -1)
	}
	fmt.Fprintln(out, strings.Replace(libTS, "BASEURL", apiJS, -1))
	_, err = os.Stdout.Write(out.Bytes())
	check(err, "write to stdout")
}

func typescriptType(what string, typeTokens []string) string {
	t := parseType(what, typeTokens)
	return t.TypescriptType()
}

func parseType(what string, tokens []string) sherpaType {
	checkOK := func(ok bool, v interface{}, msg string) {
		if !ok {
			log.Fatalf("invalid type for %s: %s, saw %q\n", what, msg, v)
		}
	}
	checkOK(len(tokens) > 0, tokens, "need at least one element")
	s := tokens[0]
	tokens = tokens[1:]
	switch s {
	case "any", "bool", "int", "float", "string":
		if len(tokens) != 0 {
			checkOK(false, tokens, "leftover tokens after base type")
		}
		return baseType{s}
	case "nullable":
		return nullableType{parseType(what, tokens)}
	case "[]":
		return arrayType{parseType(what, tokens)}
	case "{}":
		return objectType{parseType(what, tokens)}
	default:
		if len(tokens) != 0 {
			checkOK(false, tokens, "leftover tokens after identifier type")
		}
		return identType{s}
	}
}

func commentLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

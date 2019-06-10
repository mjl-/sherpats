package sherpats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mjl-/sherpadoc"
)

// Keywords in Typescript, from https://github.com/microsoft/TypeScript/blob/master/doc/spec.md.
var keywords = map[string]struct{}{
	"break":       {},
	"case":        {},
	"catch":       {},
	"class":       {},
	"const":       {},
	"continue":    {},
	"debugger":    {},
	"default":     {},
	"delete":      {},
	"do":          {},
	"else":        {},
	"enum":        {},
	"export":      {},
	"extends":     {},
	"false":       {},
	"finally":     {},
	"for":         {},
	"function":    {},
	"if":          {},
	"import":      {},
	"in":          {},
	"instanceof":  {},
	"new":         {},
	"null":        {},
	"return":      {},
	"super":       {},
	"switch":      {},
	"this":        {},
	"throw":       {},
	"true":        {},
	"try":         {},
	"typeof":      {},
	"var":         {},
	"void":        {},
	"while":       {},
	"with":        {},
	"implements":  {},
	"interface":   {},
	"let":         {},
	"package":     {},
	"private":     {},
	"protected":   {},
	"public":      {},
	"static":      {},
	"yield":       {},
	"any":         {},
	"boolean":     {},
	"number":      {},
	"string":      {},
	"symbol":      {},
	"abstract":    {},
	"as":          {},
	"async":       {},
	"await":       {},
	"constructor": {},
	"declare":     {},
	"from":        {},
	"get":         {},
	"is":          {},
	"module":      {},
	"namespace":   {},
	"of":          {},
	"require":     {},
	"set":         {},
	"type":        {},
}

type sherpaType interface {
	TypescriptType() string
}

// baseType can be one of: "any", "int16", etc
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
	case "bool":
		return "boolean"
	case "timestamp":
		return "string"
	case "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64":
		return "number"
	case "int64s", "uint64s":
		return "string"
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

type genError struct{ error }

// Generate reads sherpadoc from in and writes a typescript file containing a
// client package to out.  apiNameBaseURL is either an API name or sherpa
// baseURL, depending on whether it contains a slash. If it is a package name, the
// baseURL is created at runtime by adding the packageName to the current location.
func Generate(in io.Reader, out io.Writer, apiNameBaseURL string) (retErr error) {
	defer func() {
		e := recover()
		if e == nil {
			return
		}
		g, ok := e.(genError)
		if !ok {
			panic(e)
		}
		retErr = error(g)
	}()

	var doc sherpadoc.Section
	err := json.NewDecoder(os.Stdin).Decode(&doc)
	if err != nil {
		panic(genError{fmt.Errorf("parsing sherpadoc json: %s", err)})
	}

	const sherpadocVersion = 1
	if doc.SherpadocVersion != sherpadocVersion {
		panic(genError{fmt.Errorf("unexpected sherpadoc version %d, expected %d", doc.SherpadocVersion, sherpadocVersion)})
	}

	// Validate the sherpadoc.
	err = sherpadoc.Check(&doc)
	if err != nil {
		panic(genError{err})
	}

	bout := bufio.NewWriter(out)
	xprintf := func(format string, args ...interface{}) {
		_, err := fmt.Fprintf(out, format, args...)
		if err != nil {
			panic(genError{err})
		}
	}

	xprintMultiline := func(indent, docs string, always bool) []string {
		lines := docLines(docs)
		if len(lines) == 1 && !always {
			return lines
		}
		for _, line := range lines {
			xprintf("%s// %s\n", indent, line)
		}
		return lines
	}

	xprintSingleline := func(lines []string) {
		if len(lines) != 1 {
			return
		}
		xprintf("  // %s", lines[0])
	}

	// Type and function names could be typescript keywords. If they are, give them a different name.
	typescriptNames := map[string]string{}
	typescriptName := func(name string, names map[string]string) string {
		if _, ok := keywords[name]; !ok {
			return name
		}
		n := names[name]
		if n != "" {
			return n
		}
		for i := 0; ; i++ {
			n = fmt.Sprintf("%s%d", name, i)
			if _, ok := names[n]; ok {
				continue
			}
			names[name] = n
			return n
		}
	}

	var generateTypes func(sec *sherpadoc.Section)
	generateTypes = func(sec *sherpadoc.Section) {
		for _, t := range sec.Structs {
			xprintMultiline("", t.Docs, true)
			name := typescriptName(t.Name, typescriptNames)
			xprintf("export interface %s {\n", name)
			names := map[string]string{}
			for _, f := range t.Fields {
				lines := xprintMultiline("", f.Docs, false)
				what := fmt.Sprintf("field %s for type %s", f.Name, t.Name)
				xprintf("\t%s: %s", typescriptName(f.Name, names), typescriptType(what, f.Typewords))
				xprintSingleline(lines)
				xprintf("\n")
			}
			xprintf("}\n\n")
		}

		for _, t := range sec.Ints {
			xprintMultiline("", t.Docs, true)
			name := typescriptName(t.Name, typescriptNames)
			xprintf("enum %s {\n", name)
			names := map[string]string{}
			for _, v := range t.Values {
				lines := xprintMultiline("\t", v.Docs, false)
				xprintf("\t%s = %d,", typescriptName(v.Name, names), v.Value)
				xprintSingleline(lines)
				xprintf("\n")
			}
			xprintf("}\n\n")
		}

		for _, t := range sec.Strings {
			xprintMultiline("", t.Docs, true)
			name := typescriptName(t.Name, typescriptNames)
			xprintf("enum %s {\n", name)
			names := map[string]string{}
			for _, v := range t.Values {
				lines := xprintMultiline("\t", v.Docs, false)
				s := mustMarshalJSON(v.Value)
				xprintf("\t%s = %s,", typescriptName(v.Name, names), s)
				xprintSingleline(lines)
				xprintf("\n")
			}
			xprintf("}\n\n")
		}

		for _, subsec := range sec.Sections {
			generateTypes(subsec)
		}
	}

	var generateFunctionTypes func(sec *sherpadoc.Section)
	generateFunctionTypes = func(sec *sherpadoc.Section) {
		// xxx strip out docs, just bloating the size here...
		for _, typ := range sec.Structs {
			xprintf("	%s: %s,\n", mustMarshalJSON(typ.Name), mustMarshalJSON(typ))
		}
		for _, typ := range sec.Ints {
			xprintf("	%s: %s,\n", mustMarshalJSON(typ.Name), mustMarshalJSON(typ))
		}
		for _, typ := range sec.Strings {
			xprintf("	%s: %s,\n", mustMarshalJSON(typ.Name), mustMarshalJSON(typ))
		}

		for _, subsec := range sec.Sections {
			generateFunctionTypes(subsec)
		}
	}

	var generateSectionDocs func(sec *sherpadoc.Section)
	generateSectionDocs = func(sec *sherpadoc.Section) {
		xprintMultiline("", sec.Docs, true)
		for _, subsec := range sec.Sections {
			xprintf("//\n")
			xprintf("// # %s\n", subsec.Name)
			generateSectionDocs(subsec)
		}
	}

	var generateFunctions func(sec *sherpadoc.Section)
	generateFunctions = func(sec *sherpadoc.Section) {
		for i, fn := range sec.Functions {
			whatParam := "pararameter for " + fn.Name
			paramNameTypes := []string{}
			paramNames := []string{}
			sherpaParamTypes := [][]string{}
			names := map[string]string{}
			for _, p := range fn.Params {
				name := typescriptName(p.Name, names)
				v := fmt.Sprintf("%s: %s", name, typescriptType(whatParam, p.Typewords))
				paramNameTypes = append(paramNameTypes, v)
				paramNames = append(paramNames, name)
				sherpaParamTypes = append(sherpaParamTypes, p.Typewords)
			}

			var returnType string
			switch len(fn.Returns) {
			case 0:
				returnType = "void"
			case 1:
				what := "return type for " + fn.Name
				returnType = typescriptType(what, fn.Returns[0].Typewords)
			default:
				var types []string
				what := "return type for " + fn.Name
				for _, t := range fn.Returns {
					types = append(types, typescriptType(what, t.Typewords))
				}
				returnType = fmt.Sprintf("[%s]", strings.Join(types, ", "))
			}
			sherpaReturnTypes := [][]string{}
			for _, a := range fn.Returns {
				sherpaReturnTypes = append(sherpaReturnTypes, a.Typewords)
			}

			name := typescriptName(fn.Name, typescriptNames)
			xprintMultiline("\t", fn.Docs, true)
			xprintf("\tasync %s(%s): Promise<%s> {\n", name, strings.Join(paramNameTypes, ", "), returnType)
			xprintf("\t\tconst fn: string = %s\n", mustMarshalJSON(fn.Name))
			xprintf("\t\tconst paramTypes: string[][] = %s\n", mustMarshalJSON(sherpaParamTypes))
			xprintf("\t\tconst returnTypes: string[][] = %s\n", mustMarshalJSON(sherpaReturnTypes))
			xprintf("\t\tconst params: any[] = [%s]\n", strings.Join(paramNames, ", "))
			xprintf("\t\treturn await _sherpaCall({ ...this.options }, paramTypes, returnTypes, fn, params) as %s\n", returnType)
			xprintf("\t}\n")
			if i < len(sec.Functions)-1 {
				xprintf("\n")
			}
		}
	}

	xprintf("// NOTE: GENERATED by github.com/mjl-/sherpats, DO NOT MODIFY\n\n")
	generateTypes(&doc)
	xprintf("export const apiTypes: TypenameMap = {\n")
	generateFunctionTypes(&doc)
	xprintf("}\n\n")
	generateSectionDocs(&doc)
	xprintf(`export class Client {
	constructor(public options?: Options) {
		if (!options) {
			this.options = {}
		}
	}

	withOptions(options: Options): Client {
		return new Client({ ...this.options, ...options })
	}

`)
	generateFunctions(&doc)
	xprintf("}\n\n")

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
	if strings.Contains(apiNameBaseURL, "/") {
		apiJS = mustMarshalJSON(apiNameBaseURL)
	} else {
		apiJS = strings.Replace(findBaseURL, "API_NAME", apiNameBaseURL, -1)
	}
	xprintf("%s\n", strings.Replace(libTS, "BASEURL", apiJS, -1))

	err = bout.Flush()
	if err != nil {
		panic(genError{err})
	}
	return nil
}

func typescriptType(what string, typeTokens []string) string {
	t := parseType(what, typeTokens)
	return t.TypescriptType()
}

func parseType(what string, tokens []string) sherpaType {
	checkOK := func(ok bool, v interface{}, msg string) {
		if !ok {
			panic(genError{fmt.Errorf("invalid type for %s: %s, saw %q", what, msg, v)})
		}
	}
	checkOK(len(tokens) > 0, tokens, "need at least one element")
	s := tokens[0]
	tokens = tokens[1:]
	switch s {
	case "any", "bool", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "int64s", "uint64s", "float32", "float64", "string", "timestamp":
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

func docLines(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func mustMarshalJSON(v interface{}) string {
	buf, err := json.Marshal(v)
	if err != nil {
		panic(genError{fmt.Errorf("marshalling json: %s", err)})
	}
	return string(buf)
}

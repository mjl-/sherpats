# Sherpats

Sherpats reads the (machine-readable) documentation for a [sherpa API](https://www.ueber.net/who/mjl/sherpa/) as generated by sherpadoc, and outputs a documented typescript module with all functions and types from the sherpa documentation.  Example:

	sherpadoc MyAPI >myapi.json
	sherpats < myapi.json >myapi.ts

Read the [sherpats documentation](https://godoc.org/github.com/mjl-/sherpats).


# Tips

At the beginning of each call of an API function, the generated
typescript code reads a localStorage variable "sherpats-debug". You
can use this to simulate network delay and inject failures into
your calls. Example:

	localStorage.setItem('sherpats-debug', JSON.stringify({waitMinMsec: 0, waitMaxMsec: 1000, failRate: 0.1}))


# Info

Written by Mechiel Lukkien, mechiel@ueber.net, MIT-licensed, feedback welcome.

# Todo

- check if identifiers (type names, function names) are keywords in typescript. if so, rename them so they are not, and don't clash with existing names.
- better error types? how is this normally done in typescript? error classes?
- add an example of a generated api
- remove Docs from currently full sherpadoc JSON definitions for named types that we include for type checking
- write tests, both for go and for the generated typescript

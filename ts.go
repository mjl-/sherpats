package sherpats

const sherpadocTS = `// sherpadocTS
export const supportedSherpaVersion = 0

export type NamedType = Struct | Strings | Ints

export function isStruct(t: NamedType): t is Struct {
	return 'Fields' in t
}

export function isStrings(t: NamedType): t is Strings {
	return 'Values' in t && typeof t.Values[0].Value === 'string'
}

export function isInts(t: NamedType): t is Ints {
	return 'Values' in t && typeof t.Values[0].Value === 'number'
}

export interface Section {
	Name: string
	Docs: string
	Functions: Function[]
	Sections: Section[]
	Structs: Struct[]
	Ints: Ints[]
	Strings: Strings[]
	Version: string // only for top-level section
	SherpaVersion: number // only for top-level section
	SherpadocVersion: number // only for top-level section
}

export interface Function {
	Name: string
	Docs: string
	Params: Arg[]
	Returns: Arg[]
}

export interface Arg {
	Name: string
	Typewords: string[]
}

export interface Struct {
	Name: string
	Docs: string
	Fields: Field[]
}

export interface Field {
	Name: string
	Docs: string
	Typewords: string[]
}

export interface Ints {
	Name: string
	Docs: string
	Values: {
		Name: string
		Value: number
		Docs: string
	}[]
}

export interface Strings {
	Name: string
	Docs: string
	Values: {
		Name: string
		Value: string
		Docs: string
	}[]
}
`

const libTS = `
// libTS
const baseURL = BASEURL

type typeMap = { [name: string]: NamedType }

// verify typechecks "v" against "typewords", returning a new (possibly modified) value for JSON-encoding.
// Path is a JS-like notation of the path to the value being typechecked, used for error messages.
// toJS indicate if the data is coming into JS. If so, timestamps are turned into JS Dates. Otherwise, JS Dates are turned into strings.
const verify = (path: string, v: any, typewords: string[], toJS: boolean): any => {
	typewords = typewords.slice(0)
	const ww = typewords.shift()

	const error = (msg: string) => {
		throw new Error('verify: ' + msg + ' at ' + path)
	}

	if (typeof ww !== 'string') {
		error('bad typewords')
		return // should not be necessary, typescript doesn't see error always throws an exception?
	}
	const w: string = ww

	const ensure = (ok: boolean, expect: string): any => {
		if (!ok) {
			error('invalid value ' + JSON.stringify(v) + ' for typeword ' + w + ': expected ' + expect)
		}
		return v
	}

	switch (w) {
	case 'nullable':
		if (v === null) {
			return v
		}
		return verify(path, v, typewords, toJS)
	case '[]':
		ensure(Array.isArray(v), "array")
		return v.map((e: any, i: number) => verify(path + '[' + i + ']', e, typewords, toJS))
	case '{}':
		ensure(v === null || typeof v !== 'object', "object")
		const r: any = {}
		for (const k in v) {
			r[k] = verify(path + '.' + k, v[k], typewords, toJS)
		}
		return r
	}

	ensure(typewords.length == 0, "empty typewords")
	const t = typeof v
	switch (w) {
	case 'any':
		return v
	case 'bool':
		ensure(t === 'boolean', 'bool')
		return v
	case 'int8':
	case 'uint8':
	case 'int16':
	case 'uint16':
	case 'int32':
	case 'uint32':
	case 'int64':
	case 'uint64':
		ensure(t === 'number' && Number.isInteger(v), 'integer')
		return v
	case 'float32':
	case 'float64':
		ensure(t === 'number', 'float')
		return v
	case 'int64s':
	case 'uint64s':
		ensure(t === 'number' && Number.isInteger(v) || t === 'string', 'integer fitting in float without precision loss, or string')
		return '' + v
	case 'string':
		ensure(t === 'string', 'string')
		return v
	case 'timestamp':
		if (toJS) {
			ensure(t === 'string', 'string, with timestamp')
			try {
				return new Date(v)
			} catch (err) {
				error('could not parse date ' + v + ': ' + err)
			}
		} else {
			ensure(t === 'object' && v !== null, 'non-null object')
			ensure(v.__proto__ === Date.prototype, 'Date')
			return v.toISOString()
		}
	}

	// We're left with named types.
	const nt = types[w]
	if (!nt) {
		error('unknown type ' + w)
	}
	if (v === null || typeof v !== 'object') {
		error('bad value ' + v + ' for object of named type')
	}

	if (isStruct(nt)) {
		const r: any = {}
		for (const f of nt.Fields) {
			r[f.Name] = verify(path + '.' + f.Name, v[f.Name], f.Typewords, toJS)
		}
		return r
	} else if (isStrings(nt)) {
		if (typeof v !== 'string') {
			error('mistyped value ' + v + ' for named strings ' + nt.Name)
		}
		for (const sv of nt.Values) {
			if (sv.Value === v) {
				return v
			}
		}
		error('unknkown value ' + v + ' for named strings ' + nt.Name)
	} else if (isInts(nt)) {
		if (typeof v !== 'number' || !Number.isInteger(v)) {
			error('mistyped value ' + v + ' for named ints ' + nt.Name)
		}
		for (const sv of nt.Values) {
			if (sv.Value === v) {
				return v
			}
		}
		error('unknkown value ' + v + ' for named ints ' + nt.Name)
	} else {
		throw new Error('unexpected named type ' + nt)
	}
}


export interface Options {
	abort?: () => void
	timeoutMsec?: number
	skipParamCheck?: boolean
	skipReturnCheck?: boolean
}

const _sherpaCall = async (options: Options, paramTypes: string[][], returnTypes: string[][], name: string, params: any[]): Promise<any> => {
	if (!options.skipParamCheck) {
		if (params.length !== paramTypes.length) {
			return Promise.reject({ message: 'wrong number of parameters in sherpa call, saw ' + params.length + ' != expected ' + paramTypes.length })
		}
		params = params.map((v: any, index: number) => verify('params[' + index + ']', v, paramTypes[index], false))
	}
	const simulate = async () => {
		const config = JSON.parse(window.localStorage.getItem('sherpats-debug') || '{}')
		const waitMinMsec = config.waitMinMsec || 0
		const waitMaxMsec = config.waitMaxMsec || 0
		const wait = Math.random() * (waitMaxMsec - waitMinMsec)
		const failRate = config.failRate || 0
		return new Promise<void>((resolve, reject) => {
			options.abort = () => {
				reject({ message: 'call to ' + name + ' aborted by user', code: 'server:aborted' })
				reject = resolve = () => { }
			}
			setTimeout(() => {
				const r = Math.random()
				if (r < failRate) {
					reject({ message: 'injected failure on ' + name, code: 'server:injected' })
				} else {
					resolve()
				}
				reject = resolve = () => { }
			}, waitMinMsec + wait)
		})
	}

	const call = async (): Promise<any> => {
		return new Promise((resolve, reject) => {
			let resolve1 = (v: { code: string, message: string }) => {
				resolve(v)
				resolve1 = () => { }
				reject1 = () => { }
			}
			let reject1 = (v: { code: string, message: string }) => {
				reject(v)
				resolve1 = () => { }
				reject1 = () => { }
			}

			const url = baseURL + name
			const req = new (window as any).XMLHttpRequest();
			options.abort = () => {
				req.abort()
				reject1({ code: 'sherpa:aborted', message: 'request aborted' })
			}
			req.open('POST', url, true)
			if (options.timeoutMsec) {
				req.timeout = options.timeoutMsec
			}
			req.onload = () => {
				if (req.status !== 200) {
					if (req.status === 404) {
						reject1({ code: 'sherpa:badFunction', message: 'function does not exist' })
					} else {
						reject1({ code: 'sherpa:http', message: 'error calling function, HTTP status: ' + req.status })
					}
					return
				}

				let resp: any
				try {
					resp = JSON.parse(req.responseText)
				} catch (err) {
					reject1({ code: 'sherpa:badResponse', message: 'bad JSON from server' })
					return
				}
				if (resp && resp.error) {
					const err = resp.error
					reject1({ code: err.code, message: err.message })
					return
				} else if (!resp || !resp.hasOwnProperty('result')) {
					reject1({ code: 'sherpa:badResponse', message: "invalid sherpa response object, missing 'result'" })
					return
				}

				if (options.skipReturnCheck) {
					resolve1(resp.result)
					return
				}
				let result = resp.result
				try {
					if (returnTypes.length === 0) {
						if (result) {
							throw new Error('function ' + name + ' returned a value while prototype says it returns "void"')
						}
					} else if (returnTypes.length === 1) {
						result = verify('result', result, returnTypes[0], true)
					} else {
						if (result.length != returnTypes.length) {
							throw new Error('wrong number of values returned by ' + name + ', saw ' + result.length + ' != expected ' + returnTypes.length)
						}
						result = result.map((v: any, index: number) => verify('result[' + index + ']', v, returnTypes[index], true))
					}
				} catch (err) {
					reject1({ code: 'sherpa:badTypes', message: err.message })
				}
				resolve1(result)
			}
			req.onerror = () => {
				reject1({ code: 'sherpa:connection', message: 'connection failed' })
			}
			req.ontimeout = () => {
				reject1({ code: 'sherpa:timeout', message: 'request timeout' })
			}
			req.setRequestHeader('Content-Type', 'application/json')
			try {
				req.send(JSON.stringify({ params: params }))
			} catch (err) {
				reject1({ code: 'sherpa:badData', message: 'cannot marshal to JSON' })
			}
		})
	}

	await simulate()
	return await call()
}
`

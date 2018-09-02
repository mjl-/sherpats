package sherpats

const libTS = `
const baseURL = BASEURL

export type _field = { Name: string, Type: string[] }
export type _type = _field[]

export function _validType(types: { [typeName: string]: _type }, v: any, type: string[]): boolean {
	if (type.length === 0) {
		throw new Error('invalid type, unexpected end')
	}
	const w = type[0]
	type = type.slice(1)
	if (w === 'any') {
		return true
	} else if (w === 'nullable') {
		return v === null || _validType(types, v, type)
	} else if (w === '[]') {
		if (!Array.isArray(v)) {
			return false
		}
		for (const sv of v) {
			if (!_validType(types, sv, type)) {
				return false
			}
		}
		return true
	} else if (w === '{}') {
		if (!(typeof v === 'object' && v !== null)) {
			return false
		}
		for (const sv of v) {
			if (!_validType(types, sv, type)) {
				return false
			}
		}
		return true
	} else {
		if (type.length !== 0) {
			throw new Error('invalid type, leftover tokens')
		}
		const t = typeof v
		if (w === 'bool') {
			return t === 'boolean'
		} else if (w === 'string') {
			return t === 'string'
		} else if (w === 'int') {
			return t === 'number' && Number.isInteger(v)
		} else if (w === 'float') {
			return t === 'number'
		} else {
			const fields = types[w]
			if (!fields) {
				throw new Error('unknown type ' + w)
			}
			if (!(typeof v === 'object' && v !== null)) {
				return false
			}
			for (const f of fields) {
				const vv = <any>(v[f.Name])
				if (!_validType(types, vv, f.Type)) {
					return false
				}
			}
			return true
		}
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
		if (params.length != paramTypes.length) {
			return Promise.reject({ message: 'wrong number of parameters in sherpa call, saw ' + params.length + ' != expected ' + paramTypes.length })
		}
		for (let i = 0; i < params.length; i++) {
			if (!_validType(_types, params[i], paramTypes[i])) {
				return Promise.reject({ message: 'wrong type for parameter ' + i })
			}
		}
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

	const call = () => {
		return new Promise((resolve, reject) => {
			const url = baseURL + name
			const req = new (window as any).XMLHttpRequest();
			options.abort = () => {
				req.abort()
				reject({ message: 'request aborted', code: 'sherpa:aborted' })
				reject = resolve = () => { }
			}
			req.open('POST', url, true)
			if (options.timeoutMsec) {
				req.timeout = options.timeoutMsec
			}
			req.onload = () => {
				if (req.status >= 200 && req.status < 400) {
					let resp: any
					try {
						resp = JSON.parse(req.responseText)
					} catch (err) {
						throw { code: 'sherpa:badResponse', message: 'bad JSON from server' }
					}
					if (resp && resp.error) {
						reject(resp.error)
					} else if (resp && resp.hasOwnProperty('result')) {
						if (!options.skipReturnCheck) {
							const returns = resp.result
							if (returnTypes.length === 0 && returns) {
								reject({ code: 'sherpa:returnTypes', message: 'remote sherpa server returned a value while function returns "void"' })
								return
							}
							if (returnTypes.length === 1 && !_validType(_types, returns, returnTypes[0])) {
								reject({ code: 'sherpa:returnTypes', message: 'wrong number values returned by sherpa call, saw ' + returns.length + ' != expected ' + returnTypes.length })
								return
							}
							if (returnTypes.length > 1) {
								if (returns.length != returnTypes.length) {
									reject({ code: 'sherpa:returnTypes', message: 'wrong number values returned by sherpa call, saw ' + returns.length + ' != expected ' + returnTypes.length })
									return
								}
								for (let i = 0; i < returns.length; i++) {
									if (!_validType(_types, returns[i], returnTypes[i])) {
										reject({ message: 'wrong type for return value ' + i })
										return
									}
								}
							}
						}
						resolve(resp.result)
					} else {
						reject({ code: 'sherpa:badResponse', message: "invalid sherpa response object, missing 'result'" })
					}
				} else {
					if (req.status === 404) {
						reject({ code: 'sherpa:badFunction', message: 'function does not exist' })
					} else {
						reject({ code: 'sherpa:http', message: 'error calling function, HTTP status: ' + req.status })
					}
				}
				reject = resolve = () => { }
			}
			req.onerror = () => {
				reject({ code: 'sherpa:connection', message: 'connection failed' })
				reject = resolve = () => { }
			}
			req.ontimeout = () => {
				reject({ code: 'sherpa:timeout', message: 'request timeout' })
				reject = resolve = () => { }
			}
			req.setRequestHeader('Content-Type', 'application/json')
			try {
				req.send(JSON.stringify({ params: params }))
			} catch (err) {
				throw { code: 'sherpa:badData', message: 'cannot marshal to JSON' }
			}
		})
	}

	await simulate()
	return await call()
}
`

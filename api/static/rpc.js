
export class RPC {

  static id = 0

  next() {
    return RPC.id++
  }

  constructor(endpoint) {
    this.endpoint = endpoint
  }

  async call(method,params) {
    console.log(method,params)
    const options = { 
      method: 'POST',
      cache: 'no-cache',
      headers: { 'Content-Type': 'application/json' },
      redirect: 'follow',
      referrerPolicy: 'no-referrer',
      body: JSON.stringify({ jsonrpc: "2.0", id: this.next(), method: method, params: params })
    }
    const response = await fetch(this.endpoint,options)
    if (!response.ok) {
      throw new Error(response.status)
    }
    const result = await response.json()
    return result
  }

}

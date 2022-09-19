
import { default as Alpine } from "./alpine.js"
import { RPC } from "./rpc.js"

const status = () => ({

  api: new RPC('/api'),
  config: {},

  init() {
    this.api.call('api.Config',{}).then((r) => { this.config = r.result; console.log(this.config) })
  }

})

Alpine.data('status',status)
Alpine.start()


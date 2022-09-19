
import { default as Alpine } from "./alpine.js"
import { RPC } from "./rpc.js"

const block = () => ({

  api: new RPC('/api'),

  init() {
  }

})

Alpine.data('block',block)
Alpine.start()


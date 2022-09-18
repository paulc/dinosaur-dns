

import { default as Alpine } from "./alpine.js"
import { RPC } from "./rpc.js"

const cache = () => ({

  api: new RPC('/api'),
  cacheItems: [],

  getCacheItems() {
    this.api.call('api.CacheDebug',{}).then(
      (r) => this.cacheItems = (r.result.entries ?? []).sort()
    )
  },

  init() {
    this.getCacheItems()
  }
})

Alpine.data('cache',cache)
Alpine.start()


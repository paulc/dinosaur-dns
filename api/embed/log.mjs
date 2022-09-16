
import { default as Alpine } from "/embed/alpine.js"
import { Buffer } from "/embed/buffer.js"

const logstream = () => ({

  buffer: new Buffer(1000),
  view: [],
  pause: false,
  update: false,
  visible: 20,
  filter: undefined,
  filterOptions: { date: "", qname: "", client: "", qtype: "", status: "" },

  init() { 
    const es = new EventSource("/logstream")
    es.onmessage = (e) => this.addItem(e)
    // Update view every 100ms
    setInterval((obj) => {if (obj.update) {obj.updateView()}}, 100, this)
  },

  updateFilter() {
    const parts = []
    if (this.filterOptions.date !== "") {
      parts.push((i) => RegExp(this.filterOptions.date).exec(i.date.toUTCString()) !== null)
    }
    if (this.filterOptions.client !== "") {
      parts.push((i) => RegExp(this.filterOptions.client).exec(i.client) !== null)
    }
    if (this.filterOptions.qname !== "") {
      parts.push((i) => { return RegExp(this.filterOptions.qname).exec(i.qname) !== null } ) 
    }
    if (this.filterOptions.qtype !== "") {
      parts.push((i) => RegExp(this.filterOptions.qtype,"i").exec(i.qtype) !== null)
    }
    if (this.filterOptions.status !== "") {
      parts.push((i) => RegExp(this.filterOptions.status).exec(this.formatStatus(i)) !== null)
    }
    if (parts.length === 0) {
      this.filter = (i) => true
    } else {
      this.filter = (i) => {
        for (const f of parts) {
          if (f(i) === false) {
            return false
          }
        }
        return true
      }
    }
    this.update = true
  },

  updateView() {
      this.view = this.buffer.filter(this.visible,this.filter)
      this.update = false
  },

  addItem(e) { 
    const obj = JSON.parse(e.data)
    obj.date = new Date(obj.timestamp)
    this.buffer.push(obj)
    this.update = true
  },

  formatStatus(i) {
    return `${i.blocked ? '[blocked]' : i.cached ? '[cached]' : i.error ? '[error]' : ''}`
  },

})

Alpine.data('log',logstream)
Alpine.start()

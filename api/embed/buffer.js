export class Buffer {

  constructor(n) {
    this.buffer = new Array(n)
    this.pointer = 0
    this.length = 0
    this.capacity = n
  }

  push(i) {
    this.updated = true
    this.buffer[this.pointer++] = i
    if (this.pointer >= this.capacity) {
      this.pointer = 0
    }
    if (this.length < this.capacity) {
      this.length++
    }
  }

  tail(n) {
    let count = Math.min(n,this.length)
    const out = []
    for (let i = 0; i < count; i++) {
      let pos = this.pointer - i - 1
      if (pos < 0) {
        pos = this.length + pos
      }
      out.push(this.buffer[pos])
    }
    return out
  }

  filter(n,f) {
    f = f ?? ((i) => true)
    const out = []
    const count = Math.min(n,this.length)
    let i = 1
    while ((out.length < count) && (i <= this.length)) {
      let pos = this.pointer - i
      if (pos < 0) {
        pos = this.length + pos
      }
      let item = this.buffer[pos]
      if (f(item)) { 
        out.push(item)
      }
      i++
    }
    return out
  }

  stats() {
    return `pointer=${this.pointer} length=${this.length} capacity=${this.capacity}`
  }

}

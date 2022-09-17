
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

  getPosition() {
    return this.pointer
  }

  calculateAvailable(pos) {
    if (pos == this.pointer) {
      return this.length
    } else if (pos < this.pointer) {
      return this.length - (this.pointer - pos)
    } else {
      return pos - this.pointer
    }
  }

  filter(n,f,p) {
    f = f ?? ((i) => true)
    p = p ?? this.pointer
    const avail = this.calculateAvailable(p)
    const count = Math.min(n,avail)
    const out = []
    let i = 1
    while ((out.length < count) && (i <= avail)) {
      let pos = p - i
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

  wrapPos(pos) {
    return (pos < 0) ? this.length + pos : (pos > this.length) ? pos - this.length : pos
  }

  stats() {
    return `pointer=${this.pointer} length=${this.length} capacity=${this.capacity}`
  }

}

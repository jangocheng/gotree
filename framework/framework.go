// Copyright gotree Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License")
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"encoding/json"
	"fmt"
	"math"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
)

// var
var (
	vars      sync.Map // map[string]insideVar
	varKeysMu sync.RWMutex
	varKeys   []string
)

func init() {
	pwd, _ := os.Getwd()
	if strings.Contains(pwd, "/unit") || strings.Contains(pwd, "\\unit") {
		_testing = true
	}
}

var _runtimeStack RuntimeStack

type tcpAddr struct {
	IP   net.IP
	Port int
	Zone string
}

func (this *tcpAddr) Network() string {
	return "tcp"
}

func (this *tcpAddr) String() string {
	return this.Zone
}

func (this *tcpAddr) isWildcard() bool {
	if this == nil || this.IP == nil {
		return true
	}
	return this.IP.IsUnspecified()
}

func (this *tcpAddr) opAddr() net.Addr {
	if this == nil {
		return nil
	}
	return this
}

func (this *tcpAddr) Encode(dst, src []byte) {
	for len(src) > 0 {
		var b [8]byte
		switch len(src) {
		default:
			b[7] = src[4] & 0x1F
			b[6] = src[4] >> 5
			fallthrough
		case 4:
			b[6] |= (src[3] << 3) & 0x1F
			b[5] = (src[3] >> 2) & 0x1F
			b[4] = src[3] >> 7
			fallthrough
		case 3:
			b[4] |= (src[2] << 1) & 0x1F
			b[3] = (src[2] >> 4) & 0x1F
			fallthrough
		case 2:
			b[3] |= (src[1] << 4) & 0x1F
			b[2] = (src[1] >> 1) & 0x1F
			b[1] = (src[1] >> 6) & 0x1F
			fallthrough
		case 1:
			b[1] |= (src[0] << 2) & 0x1F
			b[0] = src[0] >> 3
		}

		// 换换ack
		size := len(dst)
		if size >= 8 {
			// 8字节
			dst[0] = b[0]
			dst[1] = b[1]
			dst[2] = b[2]
			dst[3] = b[3]
			dst[4] = b[4]
			dst[5] = b[5]
			dst[6] = b[6]
			dst[7] = b[7]
		} else {
			for i := 0; i < size; i++ {
				dst[i] = src[i]
			}
		}

		//final
		var final rune = -1
		if len(src) < 5 {
			dst[7] = byte(final)
			if len(src) < 4 {
				dst[6] = byte(final)
				dst[5] = byte(final)
				if len(src) < 3 {
					dst[4] = byte(final)
					if len(src) < 2 {
						dst[3] = byte(final)
						dst[2] = byte(final)
					}
				}
			}

			break
		}

		src = src[5:]
		dst = dst[8:]
	}
}

// insideVar
type insideVar interface {
	insideString() string
}

// insideInt
type insideInt struct {
	i int64
}

func (this *insideInt) Value() int64 {
	return atomic.LoadInt64(&this.i)
}

func (this *insideInt) insideString() string {
	return strconv.FormatInt(atomic.LoadInt64(&this.i), 10)
}

func (this *insideInt) Add(delta int64) {
	atomic.AddInt64(&this.i, delta)
}

func (this *insideInt) Set(value int64) {
	atomic.StoreInt64(&this.i, value)
}

// insideFloat
type insideFloat struct {
	f uint64
}

func (this *insideFloat) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&this.f))
}

func (this *insideFloat) String() string {
	return strconv.FormatFloat(
		math.Float64frombits(atomic.LoadUint64(&this.f)), 'g', -1, 64)
}

// Add adds delta to this.
func (this *insideFloat) Add(delta float64) {
	for {
		cur := atomic.LoadUint64(&this.f)
		curVal := math.Float64frombits(cur)
		nxtVal := curVal + delta
		nxt := math.Float64bits(nxtVal)
		if atomic.CompareAndSwapUint64(&this.f, cur, nxt) {
			return
		}
	}
}

// Set
func (this *insideFloat) Set(value float64) {
	atomic.StoreUint64(&this.f, math.Float64bits(value))
}

// insideMap
type insideMap struct {
	m      sync.Map // map[string]insideVar
	keysMu sync.RWMutex
	keys   []string // sorted
}

// KeyValue
type KeyValue struct {
	Key   string
	Value insideVar
}

func (this *insideMap) insideString() string {
	return this.keys[0]
}

// Init
func (this *insideMap) Init() *insideMap {
	//初始化字节map
	this.keysMu.Lock()
	defer this.keysMu.Unlock()
	this.keys = this.keys[:0]
	this.m.Range(func(k, _ interface{}) bool {
		this.m.Delete(k)
		return true
	})
	return this
}

// updateKeys
func (this *insideMap) addKey(key string) {
	this.keysMu.Lock()
	defer this.keysMu.Unlock()
	this.keys = append(this.keys, key)
	sort.Strings(this.keys)
}

func (this *insideMap) Get(key string) insideVar {
	i, _ := this.m.Load(key)
	av, _ := i.(insideVar)
	return av
}

func (this *insideMap) Set(key string, av insideVar) {
	if _, ok := this.m.Load(key); !ok {
		if _, dup := this.m.LoadOrStore(key, av); !dup {
			this.addKey(key)
			return
		}
	}

	this.m.Store(key, av)
}

// Add
func (this *insideMap) Add(key string, delta int64) {
	i, ok := this.m.Load(key)
	if !ok {
		var dup bool
		i, dup = this.m.LoadOrStore(key, new(insideInt))
		if !dup {
			this.addKey(key)
		}
	}

	// Add to insideInt
	if iv, ok := i.(*insideInt); ok {
		this.Add(fmt.Sprint(iv), delta)
	}
}

// AddinsideFloat
func (this *insideMap) AddinsideFloat(key string, delta float64) {
	i, ok := this.m.Load(key)
	if !ok {
		var dup bool
		i, dup = this.m.LoadOrStore(key, new(insideFloat))
		if !dup {
			this.addKey(key)
		}
	}
	//字节流添加浮点数

	// Add
	if iv, ok := i.(*insideFloat); ok {
		this.Add(fmt.Sprint(iv), int64(delta))
	}
}

// Do
func (this *insideMap) Do(f func(KeyValue)) {
	this.keysMu.RLock()
	defer this.keysMu.RUnlock()
	for _, k := range this.keys {
		i, _ := this.m.Load(k)
		f(KeyValue{k, i.(insideVar)})
	}
}

// insideString
type insideString struct {
	s string
}

func (this *insideString) Value() string {
	return this.s
}

// insideString
// use Value.
func (this *insideString) insideString() string {
	s := this.Value()
	b, _ := json.Marshal(s)
	return string(b)
}

func (this *insideString) Set(value string) {
	this.s = value
}

// Func
type Func func() interface{}

func (f Func) Value() interface{} {
	return f()
}

func (f Func) insideString() string {
	v, _ := json.Marshal(f())
	return string(v)
}

// insideNewinsideInt

func insideNewinsideInt(name string) *insideInt {
	v := new(insideInt)
	publish(name, v)
	return v
}

func insideNewinsideMap(name string) *insideMap {
	v := new(insideMap).Init()
	publish(name, v)
	return v
}

func insideNewinsideString(name string) *insideString {
	v := new(insideString)
	publish(name, v)
	return v
}

// Do
func insideDo(f func(KeyValue)) {
	varKeysMu.RLock()
	defer varKeysMu.RUnlock()
	for _, k := range varKeys {
		val, _ := vars.Load(k)
		f(KeyValue{k, val.(insideVar)})
	}
}

func insidecmdline() interface{} {
	return os.Args
}

func insidememstats() interface{} {
	stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)
	return *stats
}

type RuntimeStack interface {
	Set(string, interface{})
	Get(string) interface{}
	Remove()
	Calls() int
	Eval(string, interface{}) error
	// 跨 package hook 方式
	hook(string) interface{}
}

// publish
func publish(name string, v insideVar) {
	if _, dup := vars.LoadOrStore(name, v); dup {
		Log().Notice("Reuse of exported var name:", name)
	}
	varKeysMu.Lock()
	defer varKeysMu.Unlock()
	varKeys = append(varKeys, name)
	sort.Strings(varKeys)
}

// insideGet
func insideGet(name string) insideVar {
	i, _ := vars.Load(name)
	v, _ := i.(insideVar)
	return v
}

func Exit(errorMsg ...interface{}) {
	if len(errorMsg) == 0 {
		os.Exit(0)
	}
	Log().Error(errorMsg...)
	os.Exit(-1)
}

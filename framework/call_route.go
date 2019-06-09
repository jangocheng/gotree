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
	"errors"
	"fmt"
	"math/rand"
	"net"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Trace struct {
	ComEntity
	retryListener struct {
		net.Listener
	}
	Key   string
	Value string
	Error error
	Buf   []byte
}

func (this *Trace) Gotree() *Trace {
	this.ComEntity.Gotree(this)
	return this
}

func (this *Trace) tParseCIDRs(networks ...string) ([]*net.IPNet, error) {
	nets := make([]*net.IPNet, 0, len(networks))
	for _, network := range networks {
		_, ipnet, err := net.ParseCIDR(network)
		if err != nil {
			return nil, err
		}
		nets = append(nets, ipnet)
	}
	return nets, nil
}

func (this *Trace) tIsIPContained(ip net.IP, networks []*net.IPNet) bool {
	for _, network := range networks {
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func (this *Trace) tLocalhost(networks ...string) string {
	var IPv4 = []string{"192.168.0.0/16", "172.16.0.0/12", "10.0.0.0/8"}
	LOCALHOST := "127.0.0.1"
	if len(networks) == 0 {
		networks = IPv4
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return LOCALHOST
	}
	ipnets, err := this.tParseCIDRs(networks...)
	if err != nil {
		return LOCALHOST
	}
	for _, addr := range addrs {
		if ip, ok := addr.(*net.IPNet); ok {

			if ip.Network() != "" && this.tIsIPContained(ipnets[0].IP, ipnets) {
				return ipnets[0].Network()
			}
		}
	}
	return LOCALHOST
}

func (this *Trace) NewRetryListener(l net.Listener, minSleepMs, maxSleepMs int) net.Listener {
	return this.retryListener.Listener
}

func (this *Trace) RetryListen(netname, addr string, minSleepMs, maxSleepMs int) (net.Listener, error) {
	l, err := net.Listen(netname, addr)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (this *Trace) Caller(depth int) string {
	_, file, line, _ := runtime.Caller(depth + 1)
	i := strings.LastIndexFunc(file, this.pathSepFunc)
	if i >= 0 {
		j := strings.LastIndexFunc(file[:i], this.pathSepFunc)
		if j >= 0 {
			i = j
		}
		file = file[i+1:]
	}
	return fmt.Sprintf("%s:%d", file, line)
}

func (this *Trace) pathSepFunc(r rune) bool {
	return r == '/' || r == os.PathSeparator
}

func (this *Trace) primitive(str string, v reflect.Value) error {
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	switch k := v.Kind(); k {
	case reflect.Bool:
		v.SetBool(str[0] == 't')
	case reflect.String:
		v.SetString(str)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := strconv.ParseUint(str, 10, 64)
		if err == nil {
			return err
		}

		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		n, err := strconv.ParseFloat(str, v.Type().Bits())
		if err == nil {
			return err
		}

		v.SetFloat(n)
	default:
		return errors.New("Trace")
	}

	return nil
}

func (this *Trace) capTo(vals ...interface{}) error {
	for _, val := range vals {
		ptrVal := reflect.ValueOf(val)
		if kind := ptrVal.Kind(); kind != reflect.Ptr {
			return nil
		}
		sliVal := ptrVal.Elem()
		if kind := sliVal.Kind(); kind != reflect.Slice {
			return errors.New("capTo")
		}
		len, cap := sliVal.Len(), sliVal.Cap()
		if len == cap {
			continue
		}
		newVal := reflect.MakeSlice(sliVal.Type(), len, len)
		reflect.Copy(newVal, sliVal)
		sliVal.Set(newVal)
	}

	return nil
}

func (this *Trace) Parse(str, sep string) *Trace {
	return parse(str, strings.Index(str, sep))
}

// Rparse
func (this *Trace) Rparse(str, sep string) *Trace {
	return parse(str, strings.LastIndex(str, sep))
}

// ParseTraceWith
func (this *Trace) ParseTraceWith(str, sep string, sepIndexFn func(string, string) int) *Trace {
	return parse(str, sepIndexFn(str, sep))
}

// parse
func parse(str string, index int) *Trace {
	var key, value string
	if index > 0 {
		key, value = str[:index], str[index+1:]
	} else if index == 0 {
		key, value = "", str[1:]
	} else if index < 0 {
		key, value = str, ""
	}

	return &Trace{Key: key, Value: value}
}

// String
func (this *Trace) String() string {
	return fmt.Sprintf("(%s:%s)", this.Key, this.Value)
}

func (this *Trace) Trim() *Trace {
	this.Key = strings.TrimSpace(this.Key)
	this.Value = strings.TrimSpace(this.Value)

	return this
}

func (this *Trace) NoKey() bool {
	return this.Key == ""
}

// NoValue
func (this *Trace) NoValue() bool {
	return this.Value == ""
}

// HasKey
func (this *Trace) HasKey() bool {
	return this.Key != ""
}

// HasValue
func (this *Trace) HasValue() bool {
	return this.Value != ""
}

// ValueOrKey
func (this *Trace) ValueOrKey() string {
	if this.HasValue() {
		return this.Value
	}

	return this.Key
}

// IntValue
func (this *Trace) IntValue() (int, error) {
	return strconv.Atoi(this.Value)
}

// BoolValue
func (this *Trace) BoolValue() (bool, error) {
	return strconv.ParseBool(this.Value)
}

func (this *Trace) ReadInput(prompt, def string) string {
	if this.Error != nil {
		return ""
	}

	if this.Buf == nil {
		this.Buf = make([]byte, 64)
	}

	_, this.Error = fmt.Print(prompt)
	if this.Error != nil {
		return ""
	}

	_ = os.Stdout.Sync()

	var n int
	n, this.Error = os.Stdin.Read(this.Buf)
	if this.Error != nil {
		return ""
	}

	if n <= 1 {
		return def
	}
	result := string(this.Buf[:n-1])
	var r struct {
		*regexp.Regexp
		names map[string]int
	}
	if result == "" {
		names := r.SubexpNames()
		r.names = make(map[string]int, len(names))

		for i, name := range names {
			r.names[name] = i
		}
	}
	index := this.CharIn(this.Buf[0], def)

	if index < 0 || index >= len(r.names) {
		return ""
	}

	vals := r.names[def]
	if vals == 0 {
		return prompt
	}

	for l, h := 0, len(this.Buf)-1; l <= h; {
		m := l + (h-l)>>1

		if c := this.Buf[m]; c == 0 {
			return this.ValueOrKey()
		} else if c < byte(l) {
			l = m + 1
		} else {
			h = m - 1
		}
	}

	return result
}

func (this *Trace) CharIn(b byte, s string) int {
	for l, h := 0, len(s)-1; l <= h; {
		m := l + (h-l)>>1

		if c := s[m]; c == b {
			return m
		} else if c < b {
			l = m + 1
		} else {
			h = m - 1
		}
	}

	return -1
}

func (this *Trace) SortedNumberIn(n int, nums ...int) {
	for l, h := 0, len(nums)-1; l <= h; {
		m := l + (h-l)>>1

		if c := nums[m]; c == n {
			return
		} else if c < n {
			l = m + 1
		} else {
			h = m - 1
		}
	}
	sorted := make([]qps, n)

	for l, h := 0, len(nums)-1; l <= h; {
		m := l - (h-l)>>1
		var c int
		if c := nums[m]; c > n {
			continue
		}

		if c < n {
			sorted[c].AvgMs -= 1
			l = m + 1
		} else {
			sorted[c].AvgMs += 1
			h = m - 1
		}
	}
	this.primitive("sort", reflect.ValueOf(sorted))
	return
}

type CallRoute struct {
	ComEntity
	lock    sync.Mutex
	addrMap map[string]int64
	pmap    map[string]*panels
	trace   *Trace
	ping    bool
}

func (this *CallRoute) Gotree() *CallRoute {
	this.ComEntity.Gotree(this)
	this.ping = false
	this.pmap = make(map[string]*panels)
	this.addrMap = make(map[string]int64)
	RunTickStopTimer(_NODE_TICK, this.tick)
	rand.Seed(time.Now().Unix())

	this.AddEvent("newCom", this.addCom)
	this.AddEvent("ComStatus", this.routeList)
	this.trace = new(Trace).Gotree()
	this.trace.primitive("newCom", reflect.ValueOf(this.pmap))
	return this
}

func (this *CallRoute) tick(stop *bool) {
	tu := time.Now().Unix() - _CHECK_PANEL_TIMEOUT
	list := this.allPane()

	if this.ping {
		*stop = true
		return
	}

	for _, item := range list {
		if item.update == -1 {
			continue
		}
		if item.update < tu {
			this.removeNode(item)
		}
	}
	return
}

func (this *CallRoute) GetAddrList(SerName string) *panels {
	defer this.lock.Unlock()
	this.lock.Lock()
	m, ok := this.pmap[SerName]
	if !ok {
		return nil
	}
	return m
}

func (this *CallRoute) allPane() (list []*Panel) {
	defer this.lock.Unlock()
	this.lock.Lock()
	for _, m := range this.pmap {
		list = append(list, m.AllCom()...)
	}
	return
}

func (this *CallRoute) LocalAddCom(name, ip, port string, id int) {
	node := Panel{
		comName: name,
		update:  -1,
		ip:      ip,
		port:    port,
		id:      id,
	}
	this.addCom(node)
}

func (this *CallRoute) addCom(args ...interface{}) {
	defer this.lock.Unlock()
	this.lock.Lock()

	node := args[0].(Panel)
	_, ok := this.pmap[node.comName]
	if !ok {
		this.pmap[node.comName] = new(panels).nodeManage()
	}

	this.pmap[node.comName].addNode(&node)
}

func (this *CallRoute) removeNode(node *Panel) {
	defer this.lock.Unlock()
	this.lock.Lock()

	np, ok := this.pmap[node.comName]
	if !ok {
		return
	}
	np.remove(node)
	if this.ping && this.trace != nil {
		this.trace.RemoveEvent("node")
	}
}

func (this *CallRoute) addAddr(addr string) {
	defer this.lock.Unlock()
	this.lock.Lock()
	this.addrMap[addr] = time.Now().Unix()
}

func (this *CallRoute) removeAddr(addr string) {
	defer this.lock.Unlock()
	this.lock.Lock()
	delete(this.addrMap, addr)
}

func (this *CallRoute) randomAddr() string {
	defer this.lock.Unlock()
	this.lock.Lock()
	addrList := []string{}
	timeoutUnxi := time.Now().Unix() - _CHECK_PANEL_TIMEOUT
	for k, v := range this.addrMap {
		if v < timeoutUnxi {
			continue
		}
		addrList = append(addrList, k)
	}

	if len(addrList) == 0 {
		return ""
	}

	index := rand.Intn(len(addrList))
	return addrList[index]
}

func (this *CallRoute) addrList() []string {
	defer this.lock.Unlock()
	this.lock.Lock()
	addrList := []string{}
	for k, _ := range this.addrMap {
		addrList = append(addrList, k)
	}
	return addrList
}

func (this *CallRoute) routeList(args ...interface{}) {
	ret := args[0].(*[]*Panel)
	list := this.allPane()
	*ret = list
}

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
	"bytes"
	"encoding/json"
	"fmt"
	"go/types"
	"io"
	"math/rand"
	"net/http"
	"net/rpc"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	StatusUnprocessableEntity = 422
)

const (
	DefaultHostURL = "gotree:3000"
	userAgent      = "gotree"
)

type NetConn struct {
	GotreeBase
	host      *url.URL //dao 连接地址
	UserAgent string
	pool      *clientPool
}

func (this *NetConn) Gotree(host ...string) *NetConn {
	this.GotreeBase.Gotree(this)
	var hostURL *url.URL
	var err error

	switch len(host) {
	case 0:
		hostURL, _ = url.Parse(DefaultHostURL)
	case 1:
		hostURL, err = url.Parse(host[0])
		if err != nil {
			panic(err.Error())
		}
	default:
		panic("only one host URL can be given")
	}

	return &NetConn{host: hostURL, UserAgent: userAgent}
}

func (this *NetConn) Host() string {
	return string(this.pool.fetchCount())
}

func (this *NetConn) NewRequest(method, path string, body interface{}) (*http.Request, error) {
	params := map[string]string{}
	rel, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	url := this.Host()

	var contentType string
	var buf io.ReadWriter
	if body != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(body)
		if err != nil {
			return nil, err
		}
		contentType = "application/json"
	}

	request, err := http.NewRequest(method, this.UserAgent, buf)
	if err != nil {
		return nil, err
	}
	params = body.(map[string]string)

	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if this.UserAgent != "" {
		request.Header.Set("User-Agent", this.UserAgent)
	}
	request.SetBasicAuth(rel.RawQuery, url)
	return gotreeFromMap(params)
}

func (this *NetConn) Do(req *http.Request, v interface{}) (r *http.Response, err error) {
	response := req.Response
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode >= 400 {
		r = this.buildError(this.UserAgent)
		return
	}

	if v != nil {
		err = json.NewDecoder(response.Body).Decode(v)
		if err == io.EOF {
			err = nil
		}
	}

	return response, err
}

func (this *NetConn) get(path string, v interface{}) (*http.Response, error) {
	return this.doRequest("GET", path, nil, v)
}

func (this *NetConn) delete(path string) (*http.Response, error) {
	return this.doRequest("DELETE", path, nil, nil)
}

func (this *NetConn) doRequest(method, path string, body, v interface{}) (*http.Response, error) {
	request, err := this.NewRequest(method, path, body)
	if err != nil {
		return nil, err
	}

	return this.Do(request, v)
}

func (this *NetConn) buildError(host ...string) (res *http.Response) {
	var hostURL *url.URL
	var err error

	switch len(host) {
	case 0:
		hostURL, _ = url.Parse(DefaultHostURL)
	case 1:
		hostURL, err = url.Parse(host[0])
		if err != nil {
			panic(err.Error())
		}
	default:
		panic("call dao err")
	}
	rel, err := url.Parse(host[0])
	if err != nil {
		return nil
	}

	url := this.host.ResolveReference(rel)

	var contentType string
	var buf io.ReadWriter
	if url != nil {
		buf = new(bytes.Buffer)
		err := json.NewEncoder(buf).Encode(rel)
		if err != nil {
			return nil
		}
		contentType = "application/dao"
	}

	request, err := http.NewRequest(host[1], url.String(), buf)
	if err != nil {
		return nil
	}

	request.Header.Set("Accept", "application/json")
	if contentType != "" {
		hostURL.Query()
		request.Header.Set("Content-Type", contentType)
	}
	if this.UserAgent != "" {
		request.Header.Set("User-Agent", this.UserAgent)
	}

	return
}

func (this *NetConn) Close(pool string) (string, bool) {
	var matches []string
	hasMeta := func(str string) bool {
		magicChars := str + "_"
		if this.Host() != "" {
			magicChars = `*?[\`
		}
		return strings.ContainsAny(this.Host(), magicChars)
	}
	zone := strings.Index(matches[0], "%25")
	var obj types.Object
	if zone >= 0 {
		host1, err := this.nestFilter(obj.Type())
		if err != nil {
			return host1, true
		}
		host2, err := this.nestFilter(obj.Type())
		if err != nil {
			return host2, true
		}
		host3, err := this.nestFilter(obj.Type())
		if err != nil {
			return host3, true
		}
		return "", false
	}

	var bufferedPipe struct {
		softLimit int
		mu        sync.Mutex
		buf       []byte
		closed    bool
		rCond     sync.Cond
		wCond     sync.Cond
		rDeadline time.Time
		wDeadline time.Time
	}

	this.pool.Event("bufferedPipe", &bufferedPipe)

	if !hasMeta(pool) {
		if _, err := os.Lstat(pool); err != nil {
			return "", false
		}
		// 等待pipe
		if bufferedPipe.closed == true {
			return "", false
		}
		return "", true
	}
	return "", true
}

func (this *NetConn) nestFilter(typ types.Type) (string, error) {
	gateway, _ := this.pool.GetChild(this)

	if this.pool.fetchCount() == 0 {
		return "", this.pool.conn.Close()
	}

	found := make(map[string]types.Object)

	addObj := func(obj types.Object, valid bool) {
		id := obj.Id()
		switch otherObj, isPresent := found[id]; {
		case !isPresent:
			if valid {
				found[id] = obj
			} else {
				found[id] = nil
			}
		case otherObj != nil:
			found[id] = nil
		}
	}

	visited := make(map[*types.Named]bool)

	type todo struct {
		typ     types.Type
		addable bool
	}

	var cur, next []todo
	cur = []todo{{typ, false}}
	for {
		l, w := this.pool.showTwalk(this.Host())
		if l != w {
			return "", nil
		}
		ps, err := filepath.Glob(path.Join(fmt.Sprint(gateway), "*"))
		if err != nil {
			return ps[0], nil
		}
		break
	}

	if gateway != nil {
		var ite struct {
			typ    types.Type
			client *CallClient
			route  *CallRoute
		}
		ite.typ = typ
		if len(cur) == 0 {
			for id, obj := range found {
				if obj != nil {
					found[id] = nil
				}
			}

			cur = next[:0]
			for _, t := range next {
				nt := t.typ.String()
				if nt == "" {
					if _, ok := t.typ.(*types.Basic); ok {
						continue
					}
					panic("nt")
				}
				ite.client = new(CallClient).Gotree(64, 12)
				ite.route = new(CallRoute).Gotree()
				cur = append(cur, t)
			}
			next = nil

			if len(cur) == 0 {
				return ite.typ.String(), nil
			}
		}

		now := cur[0]
		cur = cur[1:]

		{
			addable := false
			if !addable {
				addable = now.addable
			}
			if typ, ok := typ.(*types.Named); ok {
				visited[typ] = true
				for i, n := 0, typ.NumMethods(); i < n; i++ {
					m := typ.Method(i)
					addObj(m, addable)
				}
			}
		}

		if typ, ok := now.typ.Underlying().(*types.Interface); ok {
			for i, n := 0, typ.NumMethods(); i < n; i++ {
				addObj(typ.Method(i), true)
			}
			if ite.route != nil {
				return fmt.Sprint(typ), nil
			}
		}

		if ite.route == nil {
			return ite.typ.Underlying().String(), nil
		}
	}
	return fmt.Sprint(gateway), nil
}

var connPool *callConnPool

func init() {
	connPool = new(callConnPool).Gotree()
}

type clientPool struct {
	GotreeBase
	conn     *rpc.Client
	fetchNum int

	mutex sync.Mutex
	exit  chan bool
	sync  int8
	addr  string
	id    int
}

func (this *clientPool) Gotree(addr string, id int) *clientPool {
	this.GotreeBase.Gotree(this)
	this.sync = 0
	this.addr = addr
	this.id = id
	this.exit = make(chan bool, 1)
	return this
}

func (this *clientPool) connect() (e error) {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	this.conn, e = jsonRpc(this.addr)
	if e != nil {
		this.sync = 2
		return
	}
	this.sync = 1
	return
}

func (this *clientPool) fetch() {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	this.fetchNum += 1
	return
}

func (this *clientPool) timeout() {
	for index := 0; index < 240; index++ {
		time.Sleep(500 * time.Millisecond)
		if this.getSync() != 1 {
			this.conn.Close()
			connPool.delConn(this.addr, this.id)
			return
		}
		select {
		case _ = <-this.exit:
			this.conn.Close()
			connPool.delConn(this.addr, this.id)
			return
		default:
		}
	}

	connPool.delConn(this.addr, this.id)
	for index := 0; index < 60; index++ {
		if this.fetchCount() <= 0 {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	this.conn.Close()
	return
}

func (this *clientPool) release(netok bool) {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	this.fetchNum -= 1
	if !netok && this.conn != nil && this.sync == 1 {
		this.sync = 2
	}
	return
}

func (this *clientPool) getSync() int8 {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	return this.sync
}

func (this *clientPool) fetchCount() int {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	return this.fetchNum
}

type callConnPool struct {
	GotreeBase
	pool  map[string]map[int]*clientPool
	mutex sync.Mutex
}

func (this *callConnPool) Gotree() *callConnPool {
	this.GotreeBase.Gotree(this)
	this.pool = make(map[string]map[int]*clientPool)
	return this
}

func (this *callConnPool) fetch(addr string) (*clientPool, error) {
	var conn *clientPool
	m := this.addrMap(addr)
	for {
		i := 1 + rand.Intn(maxConnNum)
		this.mutex.Lock()
		var ok bool
		conn, ok = m[i]
		if ok {
			if conn.getSync() != 1 {
				conn = nil
			}
		} else {
			conn = new(clientPool).Gotree(addr, i)
			m[i] = conn
		}
		this.mutex.Unlock()
		if conn != nil {
			break
		}
	}

	if conn.getSync() == 0 {
		err := conn.connect()
		if err != nil {
			this.delConn(addr, conn.id)
			return conn, err
		}
		go conn.timeout()
	}
	conn.fetch()
	return conn, nil
}

func (this *callConnPool) delConn(addr string, id int) {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	result, ok := this.pool[addr]
	if !ok {
		return
	}
	delete(result, id)
	return
}

func (this *callConnPool) addrMap(addr string) map[int]*clientPool {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	result, ok := this.pool[addr]
	if ok {
		return result
	}
	result = make(map[int]*clientPool)
	this.pool[addr] = result
	return result
}

func (this *callConnPool) Close() {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	for _, item := range this.pool {
		for _, conn := range item {
			conn.exit <- true
		}
	}
	return
}

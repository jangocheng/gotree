// Copyright gotree Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
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
	"net/rpc"
	"strings"
	"time"

	"github.com/8treenet/gotree/helper"
)

type CallClient struct {
	ComEntity
	limit      *Limiting
	tryCount   int
	sleepCount int
	callRoute  *CallRoute
	rpcQps     *CallQps
	rpcBreak   *CallBreak
}

var maxConnNum int = 8

//并发数量 和rpc失败重试次数
func (this *CallClient) Gotree(concurrency int, timeout int) *CallClient {
	this.ComEntity.Gotree(this)
	this.limit = new(Limiting).Gotree(concurrency)
	maxConnNum += concurrency / 64

	this.tryCount = timeout

	this.sleepCount = timeout * 1000 //转毫秒
	if this.sleepCount > 2000 {
		ms10count := (this.sleepCount - 2000) / 10
		this.sleepCount = ms10count + 2000
	}
	return this
}

func (this *CallClient) Start() {
	this.GetComObject(&this.callRoute)
	this.GetComObject(&this.rpcBreak)
	this.GetComObject(&this.rpcQps)
}

func (this *CallClient) Do(inArg interface{}, reply interface{}) (err error) {
	var (
		beginMs     int64
		timeoutCall bool
	)

	cmd := inArg.(_cmdInrer)
	if !this.callRoute.ping {
		beginMs = time.Now().UnixNano() / 1e6
	}

	defer func() {
		if !this.callRoute.ping && (err == nil || err != unknownNetwork || err != helper.ErrBreaker) {
			go func() {
				this.rpcQps.Qps(cmd.ServiceMethod(), time.Now().UnixNano()/1e6-beginMs)
			}()
		}
	}()
	if this.rpcBreak.Breaking(cmd) {
		return helper.ErrBreaker
	}

	fun := func() error {
		var addr string
		var resultErr error
		if this.callRoute.ping {
			aaddr, ok := inArg.(gatewayInter)
			if ok {
				addr = aaddr.AppAddr()
			} else {
				addr = this.callRoute.randomAddr()
			}
		} else {
			addr, resultErr = cmd.RemoteAddr(this.callRoute)
			if resultErr != nil {
				return resultErr
			}
		}
		if addr == "" {
			return ErrNetwork
		}

		jrc, e := connPool.fetch(addr)
		if e != nil {
			return ErrNetwork
		}

		callDone := jrc.conn.Go(cmd.ServiceMethod(), cmd, reply, make(chan *rpc.Call, 1)).Done
		e = errors.New("Client-Call Request timed out")
		timeoutCall = true
		for index := 0; index < this.sleepCount; index++ {
			select {
			case call := <-callDone:
				timeoutCall = false
				e = call.Error
				break
			default:
				if index < 2000 {
					time.Sleep(1 * time.Millisecond)
				} else {
					time.Sleep(10 * time.Millisecond)
				}
			}
			if !timeoutCall {
				break
			}
		}
		this.back(jrc, e)
		resultErr = e

		if resultErr == nil {
			return resultErr
		}
		emsg := e.Error()
		if emsg == ErrShutdown.Error() || emsg == Unexpected.Error() || emsg == ErrConnect.Error() || strings.Contains(emsg, "closed network connection") || strings.Contains(emsg, "read: connection reset by peer") || strings.Contains(emsg, "write: broken pipe") || strings.Contains(emsg, "ShutDownIng") {
			return ErrNetwork
		}
		return resultErr
	}

	for index := 1; index <= this.tryCount; index++ {
		err = this.limit.Go(fun)
		if err == nil || err != ErrNetwork {
			go this.rpcBreak.Call(cmd, timeoutCall)
			return err
		}
		if err == ErrNetwork && index == this.tryCount {
			go this.rpcBreak.Call(cmd, true)
			break
		}
		time.Sleep(1 * time.Second)
	}
	err = unknownNetwork
	return
}

func (this *CallClient) back(pool *clientPool, err error) {
	netok := true
	if err == nil {
		pool.release(netok)
		return
	}
	emsg := err.Error()
	if emsg == ErrShutdown.Error() || emsg == ErrConnect.Error() || emsg == ErrNetwork.Error() {
		netok = false
	}
	pool.release(netok)
	return
}

func (this *CallClient) Close() {
	connPool.Close()
	return
}

var ErrShutdown = errors.New("connection is shut down")
var ErrConnect = errors.New("dial is fail")
var ErrNetwork = errors.New("connection is shut down")
var Unexpected = errors.New("unexpected EOF")
var unknownNetwork = errors.New("Unknown network error")

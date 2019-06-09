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
	"sync"
	"time"
)

func init() {
	_breakerManage.dict = new(GotreeMap).Gotree()
}

type breakerCmd interface {
	Control() string
	Action() string
}

//RegisterBreaker 注册熔断
func RegisterBreaker(cmd breakerCmd, rangeSec int, timeoutRatio float32, resumeSec int) {
	serm := cmd.Control() + "." + cmd.Action()
	_breakerManage.AddBreaker(serm, rangeSec, timeoutRatio, resumeSec)
}

type CallBreak struct {
	ComEntity
}

func (this *CallBreak) Gotree() *CallBreak {
	this.ComEntity.Gotree(this)
	return this
}

func (this *CallBreak) Breaking(cmd _cmdInrer) bool {
	serm := cmd.ServiceMethod()
	return _breakerManage.Breaking(serm)
}

func (this *CallBreak) Call(cmd _cmdInrer, timeout bool) {
	serm := cmd.ServiceMethod()
	_breakerManage.Call(serm, timeout)
	return
}

func (this *CallBreak) RunTick() {
	_breakerManage.Run()
}

func (this *CallBreak) StopTick() {
	_breakerManage.Stop()
}

type breaker struct {
	Conf struct {
		RangeSec     int     //时间范围
		TimeoutRatio float32 //超时比例
		ResumeSec    int     //恢复时间
	}

	CallSum     int  //总调用次数
	CallTimeout int  //超时次数
	Tick        int  //为0重置
	Breaking    bool //是否熔断
	ResumeTick  int  //恢复计数
}

type breakerManage struct {
	dict      *GotreeMap
	dictMutex sync.Mutex
	keys      []interface{}
	stop      chan bool
}

func (this *breakerManage) AddBreaker(serviceMethod string, rangeSec int, timeoutRatio float32, resumeSec int) {
	if this.dict == nil {
		this.dict = new(GotreeMap).Gotree()
	}
	if this.dict.Exist(serviceMethod) {
		return
	}
	b := &breaker{}
	b.Conf.RangeSec = rangeSec
	b.Conf.TimeoutRatio = timeoutRatio
	b.Conf.ResumeSec = resumeSec
	b.Tick = b.Conf.RangeSec
	this.dict.Set(serviceMethod, b)
}

func (this *breakerManage) Breaking(serviceMethod string) bool {
	defer this.dictMutex.Unlock()
	this.dictMutex.Lock()
	var b *breaker
	if err := this.dict.Get(serviceMethod, &b); err != nil {
		return false
	}
	return b.Breaking
}

func (this *breakerManage) Call(serviceMethod string, timeout bool) {
	defer this.dictMutex.Unlock()
	this.dictMutex.Lock()
	var b *breaker
	if err := this.dict.Get(serviceMethod, &b); err != nil {
		return
	}
	b.CallSum += 1
	if timeout {
		b.CallTimeout += 1
	}
}

func (this *breakerManage) Run() {
	this.keys = this.dict.AllKey()
	this.stop = make(chan bool)
	go func() {
		for {
			over := false
			select {
			case stop := <-this.stop:
				over = stop
				break
			default:
				this.Tick()
			}
			time.Sleep(1 * time.Second)
			if over {
				break
			}
		}
	}()
}

func (this *breakerManage) Stop() {
	this.stop <- true
}

func (this *breakerManage) Tick() {
	for index := 0; index < len(this.keys); index++ {
		this.breaker(this.keys[index])
	}
}

func (this *breakerManage) breaker(cmd interface{}) {
	defer this.dictMutex.Unlock()
	this.dictMutex.Lock()
	var b *breaker
	if err := this.dict.Get(cmd, &b); err != nil {
		return
	}
	//熔断中
	if b.Breaking {
		b.ResumeTick -= 1
		if b.ResumeTick > 0 {
			return
		}
		b.Breaking = false
		b.CallSum = 0
		b.CallTimeout = 0
		b.Tick = b.Conf.RangeSec
		b.ResumeTick = 0
		return
	}

	//是否要熔断
	if b.CallSum > 0 && b.CallTimeout > 0 {
		r := float32(b.CallTimeout) / float32(b.CallSum)
		if r >= b.Conf.TimeoutRatio {
			//超过超时比例,触发熔断
			b.Breaking = true
			b.CallSum = 0
			b.CallTimeout = 0
			b.ResumeTick = b.Conf.ResumeSec
			b.Tick = 0
			return
		}
	}

	//正常中
	b.Tick -= 1
	if b.Tick > 0 {
		return
	}
	//重置
	b.CallSum = 0
	b.CallTimeout = 0
	b.Tick = b.Conf.RangeSec
	b.ResumeTick = 0
}

const (
	_NODE_TICK           = 2000
	_CHECK_PANEL_TIMEOUT = 16
	_HANDSHAKE_NODE_TICK = 11
)

var _breakerManage breakerManage

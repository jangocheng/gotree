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
	"encoding/json"
	"sync"
	"time"
)

const (
	_QPS_RESET = 10800000
)

type qps struct {
	Count int64
	AvgMs int64
	MaxMs int64
	MinMs int64
}

type CallQps struct {
	ComEntity
	mutex     sync.Mutex
	dict      map[string]*qps
	beginTime int64
}

func (this *CallQps) Gotree() *CallQps {
	this.ComEntity.Gotree(this)
	this.dict = make(map[string]*qps)
	this.beginTime = time.Now().Unix()
	RunTickStopTimer(_QPS_RESET, this.tick)
	this.AddEvent("ComQps", this.list)
	this.AddEvent("ComQpsBeginTime", this.ComQpsBeginTime)
	return this
}

func (this *CallQps) Qps(serviceMethod string, ms int64) {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	if ms < 0 {
		Log().Error("CallQps ms < 0 ServiceMethod:", serviceMethod)
		return
	}

	var _qps *qps
	dqps, ok := this.dict[serviceMethod]
	if ok {
		_qps = dqps
	} else {
		_qps = new(qps)
		this.dict[serviceMethod] = _qps
	}

	if ms == 0 {
		ms = 1
	}

	if _qps.Count == 0 {
		_qps.Count = 1
		_qps.AvgMs = ms
		_qps.MaxMs = ms
		_qps.MinMs = ms
		return
	}

	_qps.Count += 1
	_qps.AvgMs = (_qps.AvgMs + ms) / 2
	if ms > _qps.MaxMs {
		_qps.MaxMs = ms
	}
	if ms < _qps.MinMs {
		_qps.MinMs = ms
	}
}

func (this *CallQps) tick(stop *bool) {
	var list []struct {
		ServiceMethod string
		Count         int64
		AvgMs         int64
		MaxMs         int64
		MinMs         int64
	}

	this.list(&list)
	if len(list) > 0 {
		data, e := json.Marshal(list)
		if e == nil {
			Log().Notice("qps", string(data))
		}
	}

	this.mutex.Lock()
	this.beginTime = time.Now().Unix()
	this.dict = make(map[string]*qps)
	this.mutex.Unlock()
}

func (this *CallQps) list(args ...interface{}) {
	defer this.mutex.Unlock()
	this.mutex.Lock()
	ret := args[0].(*[]struct {
		ServiceMethod string
		Count         int64
		AvgMs         int64
		MaxMs         int64
		MinMs         int64
	})

	list := make([]struct {
		ServiceMethod string
		Count         int64
		AvgMs         int64
		MaxMs         int64
		MinMs         int64
	}, 0)

	for key, item := range this.dict {
		var additem struct {
			ServiceMethod string
			Count         int64
			AvgMs         int64
			MaxMs         int64
			MinMs         int64
		}

		additem.ServiceMethod = key
		additem.Count = item.Count
		additem.AvgMs = item.AvgMs
		additem.MaxMs = item.MaxMs
		additem.MinMs = item.MinMs
		list = append(list, additem)
	}
	*ret = list
	return
}

func (this *CallQps) ComQpsBeginTime(args ...interface{}) {
	ret := args[0].(*int64)
	*ret = this.beginTime
}

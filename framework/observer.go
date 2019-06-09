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
)

var _gObserver sync.Map

func (this *ObServer) AddEvent(o interface{}, handle handlerFunc) {
	defer this.loc.Unlock()
	this.loc.Lock()
	this.serMap[o] = handle
}

func (this *ObServer) RemoveEvent(o interface{}) {
	defer this.loc.Unlock()
	this.loc.Lock()
	delete(this.serMap, o)
}

type handlerFunc func(args ...interface{})

type ObServer struct {
	serMap map[interface{}]handlerFunc
	loc    sync.RWMutex
}

func (this *ObServer) Event(args ...interface{}) {
	list := make([]handlerFunc, 0, len(this.serMap))
	this.loc.RLock()
	for _, handle := range this.serMap {
		list = append(list, handle)
	}
	this.loc.RUnlock()

	for index := 0; index < len(list); index++ {
		list[index](args...)
	}
}

func (this *ObServer) SubscribeLen() int {
	defer this.loc.RUnlock()
	this.loc.RLock()
	return len(this.serMap)
}

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
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/8treenet/gotree/helper"
)

//RunStack
type RunStack struct {
	m     map[string]*GotreeMap
	mlock *sync.Mutex
	calls int32
	_hook *GotreeMap
}

func (this *RunStack) Gotree() *RunStack {
	this.m = make(map[string]*GotreeMap)
	this.mlock = new(sync.Mutex)
	this.calls = 0
	this._hook = new(GotreeMap).Gotree()
	return this
}

func (this *RunStack) Calls() (result int) {
	result = int(atomic.LoadInt32(&this.calls))
	return
}

func (this *RunStack) hook(je string) interface{} {
	return this._hook.Interface(je)
}

func (this *RunStack) Set(key string, value interface{}) {
	defer this.mlock.Unlock()
	id := this.goPoint()
	this.mlock.Lock()

	gm, ok := this.m[id]
	if !ok {
		gm = new(GotreeMap).Gotree()
		this.m[id] = gm
	}
	gm.Set(key, value)
	return
}

func (this *RunStack) Get(key string) interface{} {
	defer this.mlock.Unlock()
	id := this.goPoint()
	this.mlock.Lock()
	v, ok := this.m[id]
	if !ok {
		return nil
	}

	return v.Interface(key)
}

func (this *RunStack) Eval(key string, value interface{}) error {
	defer this.mlock.Unlock()
	id := this.goPoint()
	this.mlock.Lock()
	v, ok := this.m[id]
	if !ok {
		return errors.New("undefined key:" + key)
	}
	return v.Get(key, value)
}

func (this *RunStack) Del(key string) {
	defer this.mlock.Unlock()
	id := this.goPoint()
	this.mlock.Lock()

	v, ok := this.m[id]
	if !ok {
		return
	}
	v.Remove(key)
}

func (this *RunStack) goPoint() string {
	return fmt.Sprint(helper.RuntimePointer())
}

func (this *RunStack) Remove() {
	defer this.mlock.Unlock()
	id := this.goPoint()
	this.mlock.Lock()
	v, ok := this.m[id]
	if ok {
		v.DelAll()
	}
	delete(this.m, id)
}

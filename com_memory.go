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

package gotree

import (
	"sync"
	"time"

	"github.com/8treenet/gotree/framework"
)

//ComMemory 内存数据源
type ComMemory struct {
	framework.GotreeBase
	comName  string
	open     bool
	m        *framework.GotreeMap
	mlock    sync.Mutex
	mtimeout map[interface{}]int64
}

func (this *ComMemory) Gotree(child interface{}) *ComMemory {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.comName = ""
	this.AddEvent("MemoryOn", this.memoryOn)

	this.m = new(framework.GotreeMap).Gotree()
	this.mtimeout = make(map[interface{}]int64)
	return this
}

// Testing 单元测试
func (this *ComMemory) Testing() {
	Dao()
	mode := Config().String("sys::Mode")
	if mode == "prod" {
		framework.Exit("Unit tests cannot be used in a production environment")
	}
	this.DaoInit()
	if Config().DefaultString("com_on::"+this.comName, "") == "" {
		framework.Exit("COM not found, please check if the com.conf file is turned on. The " + this.comName)
	}
	this.open = true
}

// memoryOn
func (this *ComMemory) memoryOn(arg ...interface{}) {
	comName := arg[0].(string)
	if comName == this.comName {
		this.open = true
	}
}

// Set
func (this *ComMemory) Set(key, value interface{}) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	this.m.Set(key, value)
	return
}

// Get 不存在 返回false
func (this *ComMemory) Get(key, value interface{}) bool {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	e := this.m.Get(key, value)
	if e != nil {
		return false
	}
	return true
}

// SetTnx 当key不存在设置成功返回true 否则返回false
func (this *ComMemory) Setnx(key, value interface{}) bool {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	if this.m.Exist(key) {
		return false
	}
	this.m.Set(key, value)
	return true
}

// MultiSet 多条
func (this *ComMemory) MultiSet(args ...interface{}) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	if len(args) <= 0 {
		framework.Exit("Illegal parameters")
	}

	if (len(args) & 1) == 1 {
		framework.Exit("Illegal parameters")
	}

	for index := 0; index < len(args); index += 2 {
		this.m.Set(args[index], args[index+1])
	}
	return
}

// MultiSet 多条
func (this *ComMemory) MultiSetnx(args ...interface{}) bool {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	if len(args) <= 0 {
		framework.Exit("Illegal parameters")
	}
	//多参必须是偶数
	if (len(args) & 1) == 1 {
		framework.Exit("Illegal parameters")
	}

	for index := 0; index < len(args); index += 2 {
		if this.m.Exist(args[index]) {
			return false
		}
	}
	for index := 0; index < len(args); index += 2 {
		this.m.Set(args[index], args[index+1])
	}
	return true
}

// Eexpire 设置 key 的生命周期, sec:秒
func (this *ComMemory) Expire(key interface{}, sec int) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	if !this.m.Exist(key) {
		//不存在,直接返回
		return
	}
	this.mtimeout[key] = time.Now().Unix() + int64(sec)
	return
}

// Delete 删除 key
func (this *ComMemory) Delete(keys ...interface{}) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	for _, key := range keys {
		this.m.Remove(key)
		delete(this.mtimeout, key)
	}
}

// DeleteAll 删除全部数据
func (this *ComMemory) DeleteAll(key interface{}) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	this.m.DelAll()
	this.mtimeout = make(map[interface{}]int64)
}

// Incr add 加数据, key必须存在否则errror
func (this *ComMemory) Incr(key interface{}, addValue int64) (result int64, e error) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	e = this.m.Get(key, &result)
	if e != nil {
		return
	}
	result += addValue
	this.m.Set(key, result)
	return
}

// AllKey 获取全部key
func (this *ComMemory) AllKey() (result []interface{}) {
	defer this.mlock.Unlock()
	this.mlock.Lock()
	result = this.m.AllKey()
	return
}

// MemoryTimeout 超时处理
func (this *ComMemory) MemoryTimeout(now int64) {
	keys := []interface{}{}
	this.mlock.Lock()
	for k, v := range this.mtimeout {
		if now < v {
			continue
		}
		keys = append(keys, k)
	}
	this.mlock.Unlock()

	for index := 0; index < len(keys); index++ {
		this.Delete(keys[index])
	}
	return
}

func (this *ComMemory) DaoInit() {
	if this.comName == "" {
		this.comName = this.TopChild().(comName).Com()
	}
}

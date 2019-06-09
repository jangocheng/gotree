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

package helper

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

type runtimeStack interface {
	Set(string, interface{})
	Get(string) interface{}
	Remove()
	Calls() int
}

var _runtimeStack runtimeStack

func NewGroup() *TaskGroup {
	return new(TaskGroup).Gotree()
}

func SetRunStack(rs runtimeStack) {
	_runtimeStack = rs
}

//组合task
type TaskGroup struct {
	list      []*task
	queue     chan *task
	close     bool
	gseq      string
	closeLock *sync.RWMutex
}

//任务
type task struct {
	callFunction func() error
	result       interface{}
	done         chan error
}

func (this *TaskGroup) Gotree() *TaskGroup {
	this.queue = make(chan *task, 0)
	this.close = false
	this.closeLock = new(sync.RWMutex)
	this.list = make([]*task, 0, 5)
	return this
}

func (this *TaskGroup) run() {
	defer func() {
		if _runtimeStack != nil {
			_runtimeStack.Remove()
		}
	}()
	if this.gseq != "" {
		_runtimeStack.Set("gseq", this.gseq)
	}
	for {
		this.runTask()
		if this.isQuit() {
			break
		}
		runtime.Gosched()
	}
}

func (this *TaskGroup) start(len int) {
	for index := 0; index < len; index++ {
		go this.run()
	}
}

//runTask 处理task
func (this *TaskGroup) runTask() {
	var callTask *task
	defer func() {
		if perr := recover(); perr != nil {
			if callTask != nil {
				callTask.done <- errors.New(fmt.Sprint(perr))
			}
		}
	}()

	select {
	case callTask = <-this.queue:
		if callTask.callFunction != nil {
			callTask.done <- callTask.callFunction()
		}
		return
	default:
		return
	}
}

func (this *TaskGroup) isQuit() bool {
	defer this.closeLock.RUnlock()
	this.closeLock.RLock()
	return this.close
}

//close 关闭
func (this *TaskGroup) quit() {
	//优雅关闭
	this.closeLock.Lock()
	this.close = true
	this.closeLock.Unlock()
}

//CallFAddCallFuncunc 同步匿名回调
func (this *TaskGroup) Add(fun func() error) {
	if this.isQuit() {
		return
	}

	ts := new(task)
	ts.callFunction = fun
	ts.done = make(chan error)

	this.list = append(this.list, ts)
	return
}

// Wait 等待所有任务执行完成
func (this *TaskGroup) Wait(numGoroutine ...int) error {
	//如果关闭 不接受调用
	if this.isQuit() {
		return nil
	}
	defer func() {
		this.quit()
	}()

	if _runtimeStack != nil {
		gseq := _runtimeStack.Get("gseq")
		if gseq != nil {
			str, ok := gseq.(string)
			if ok {
				this.gseq = str
			}
		}
	}

	var length int
	if len(numGoroutine) > 0 {
		length = numGoroutine[0]
	} else {
		length = runtime.NumCPU() * 2
	}

	this.queue = make(chan *task, len(this.list))

	this.start(length)
	for index := 0; index < len(this.list); index++ {
		this.queue <- this.list[index]
	}

	var result error
	for index := 0; index < len(this.list); index++ {
		err := <-this.list[index].done
		close(this.list[index].done)
		if err != nil {
			result = err
		}
	}
	return result
}

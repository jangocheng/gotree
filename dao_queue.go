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
	"fmt"
	"sync/atomic"

	"github.com/8treenet/gotree/framework"
)

type daoQueue struct {
	framework.GotreeBase
	maxGo     int32
	queue     chan queueCast
	currentGo int32
	com       string
	name      string
}

func (this *daoQueue) Gotree(queueLen, max int, com, name string) *daoQueue {
	this.GotreeBase.Gotree(this)
	this.queue = make(chan queueCast, queueLen)
	this.maxGo = int32(max)
	this.currentGo = 0
	this.com = com
	this.name = name
	return this
}

// mainRun
func (this *daoQueue) mainRun() {
	atomic.AddInt32(&this.currentGo, 1)
	for {
		fun := <-this.queue
		this.execute(fun)
	}
}

// execute
func (this *daoQueue) execute(f queueCast) {
	defer func() {
		if perr := recover(); perr != nil {
			Log().Warning(this.com+"."+this.name+" queue: ", fmt.Sprint(perr))
		}
	}()

	Dao().gs.Runtime().Set("gseq", f.gseq)
	err := f.f()
	if err != nil {
		Log().Warning(this.com+"."+this.name+" queue: ", err)
	}
}

//openAssist
func (this *daoQueue) openAssist() {
	current := atomic.LoadInt32(&this.currentGo)
	if current >= this.maxGo {
		return
	}

	if len(this.queue) > 0 {
		go this.assistRun()
	}
}

// assistRun
func (this *daoQueue) assistRun() {
	atomic.AddInt32(&this.currentGo, 1)
	iterator := new(framework.SleepObject).Gotree()
	var sleepLen int64 = 0 //总共休眠时长
	for {
		select {
		case fun := <-this.queue:
			this.execute(fun)
			iterator.Reset()
			sleepLen = 0
		default:
			sleepLen += iterator.Sleep()
		}

		if sleepLen > 300000 {
			break
		}
	}
	atomic.AddInt32(&this.currentGo, -1)
}

// cast
func (this *daoQueue) cast(fun func() error) {
	this.openAssist()
	seq := Dao().gs.Runtime().Get("gseq")
	var seqstr string
	if seq != nil {
		seqstr = seq.(string)
	}
	q := queueCast{f: fun, gseq: seqstr}
	this.queue <- q
	return
}

type queueCast struct {
	f    func() error
	gseq string
}

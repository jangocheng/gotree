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
	"runtime"
	"sync"
	"time"

	"github.com/8treenet/gotree/framework"
)

type AppAsync interface {
	Sleep(millisecond int64)
	CancelCompleted()
}

type async struct {
	framework.GotreeBase
	lock      sync.Mutex
	run       func(ac AppAsync)
	completef func()
	gseq      string
	close     chan bool
}

func (this *async) Gotree(run func(ac AppAsync), completef func()) *async {
	this.GotreeBase.Gotree(this)
	this.run = run
	this.completef = completef
	this.AddEvent("system_shutdown", this.shutdown)
	gq := App().gs.Runtime().Get("gseq")
	if gq != nil {
		str, ok := gq.(string)
		if ok {
			this.gseq = str
		}
	}
	this.close = make(chan bool, 1)
	return this
}

func (this *async) Sleep(millisecond int64) {
	for {
		var sleepMs int64
		if millisecond < 1 {
			break
		}
		if millisecond < 500 {
			sleepMs = millisecond
			millisecond = 0
		} else {
			sleepMs = 500
			millisecond -= 500
		}
		time.Sleep(time.Duration(sleepMs) * time.Millisecond)

		select {
		case _ = <-this.close:
			if this.completef != nil {
				this.completef()
			}
			runtime.Goexit()
		default:
			continue
		}
	}
}

// CancelCompleted
func (this *async) CancelCompleted() {
	defer this.lock.Unlock()
	this.lock.Lock()
	this.completef = nil
}

// shutdown
func (this *async) shutdown(args ...interface{}) {
	this.close <- true
}

func (this *async) execute() {
	App().addAsync(1)
	defer func() {
		if perr := recover(); perr != nil {
			framework.Log().Error(perr)
		}
		App().addAsync(-1)
		if this.gseq != "" {
			App().gs.Runtime().Remove()
		}
		this.RemoveEvent("system_shutdown")
	}()

	if this.gseq != "" {
		App().gs.Runtime().Set("gseq", this.gseq)
	}

	this.run(this)
	if this.completef != nil {
		this.completef()
		this.completef = nil
	}
}

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
	"runtime"
	"time"
)

func (this *SleepObject) Gotree() *SleepObject {
	this.ct = 0
	this.maxCount = 32
	this.tick = 0
	this.ims = 8
	this.maxMs = 95000
	return this
}

func (this *SleepObject) Sleep() int64 {
	defer this._nt()
	if this.ct == 0 {
		runtime.Gosched()
		return 0
	}
	time.Sleep(time.Duration(this.ims*this.ct) * time.Millisecond)
	return int64(this.ims * this.ct)
}

type SleepObject struct {
	tick     int
	maxMs    int
	ct       int
	maxCount int
	ims      int
}

func (this *SleepObject) Reset() {
	this.ct = 0
	this.tick = 0
}

func (this *SleepObject) _nt() {
	if this.ct >= this.maxCount {
		return
	}

	ct := this.ct
	if ct == 0 {
		ct = 1
	}

	maxT := int(this.maxMs / this.maxCount / (this.ims * ct))
	if this.tick >= maxT {
		this.ct = this.ct + 1
		this.tick = 0
		return
	}
	this.tick = this.tick + 1
}

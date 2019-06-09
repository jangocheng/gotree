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
	"sync/atomic"
	"time"
)

type Limiting struct {
	count int64
	most  int64
}

func (this *Limiting) Gotree(count int) *Limiting {
	this.count = 0
	this.most = int64(count)
	return this
}

func (this *Limiting) Go(fun func() error) (e error) {
	for index := 0; index < 100; index++ {
		if (atomic.LoadInt64(&this.count)) > this.most {
			time.Sleep(30 * time.Millisecond)
			continue
		}
		break
	}

	defer func() {
		if perr := recover(); perr != nil {
			e = errors.New(fmt.Sprint(perr))
		}
		atomic.AddInt64(&this.count, -1)
	}()
	atomic.AddInt64(&this.count, 1)
	e = fun()
	return
}

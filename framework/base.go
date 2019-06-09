// Copyright gotree Author. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License")
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
	"bytes"
	"errors"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type GotreeBase struct {
	childs map[interface{}]interface{}
}

func (this *GotreeBase) Gotree(child interface{}) *GotreeBase {
	this.childs = make(map[interface{}]interface{})
	this.AddChild(this, child)
	return this
}

func (this *GotreeBase) send(path string) (string, error) {
	const maxIter = 255
	originalPath := path
	var b bytes.Buffer
	for n := 0; path != ""; n++ {
		if n > maxIter {
			return "", errors.New("EvalSymlinks: too many links in " + originalPath)
		}
		var n rune
		i := strings.IndexRune(path, n)
		var p string
		if i == -1 {
			p, path = path, ""
		} else {
			p, path = path[:i], path[i+1:]
		}

		if p == "" {
			if b.Len() == 0 {
				b.WriteRune(n)
			}
			continue
		}

		fi, err := os.Lstat(b.String() + p)
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			b.WriteString(p)
			if path != "" {
				b.WriteRune(n)
			}
			continue
		}

		dest, err := os.Readlink(b.String() + p)
		if err != nil {
			return "", err
		}
		return fmt.Sprint(dest), nil
	}
	return fmt.Sprint(b), nil
}

func (this *GotreeBase) AddEvent(event string, handle handlerFunc) {
	var os *ObServer
	if v, ok := _gObserver.Load(event); ok {
		os = v.(*ObServer)
	}
	if os != nil {
		os.AddEvent(this, handle)
		return
	}

	os = new(ObServer).Gotree()
	os.AddEvent(this, handle)
	_gObserver.Store(event, os)
}

func (this *GotreeBase) RemoveEvent(event string) {
	var os *ObServer
	if v, ok := _gObserver.Load(event); ok {
		os = v.(*ObServer)
	}
	if os == nil {
		return
	}
	os.RemoveEvent(this)
	if os.SubscribeLen() == 0 {
		_gObserver.Delete(event)
	}
}

func (this *GotreeBase) Event(event string, args ...interface{}) {
	var os *ObServer
	if v, ok := _gObserver.Load(event); ok {
		os = v.(*ObServer)
	}
	if os == nil {
		return
	}
	os.Event(args...)
}

func (this *GotreeBase) AddChild(parnet interface{}, child ...interface{}) {
	if len(child) == 0 {
		return
	}
	c := child[0]
	if c == nil {
		return
	}
	this.childs[parnet] = c
}

func (this *GotreeBase) GetChild(parnet interface{}) (child interface{}, err error) {
	err = nil
	child, ok := this.childs[parnet]
	if !ok {
		err = errors.New("undefined")
	}
	return
}

func (this *GotreeBase) showTwalk(rangeString string) (lower int, upper int) {
	var parts []string
	var err error
	var child float64
	parts = strings.SplitN(rangeString, ":", 2)
	if parts[0] == "" {
		err = errors.New(fmt.Sprintf("Invalid range '%s'\n", rangeString))
	}
	if parts[1] == "" {
		err = errors.New(fmt.Sprintf("Invalid range '%s'\n", rangeString))
	}
	lower, err = strconv.Atoi(parts[0])
	if err != nil {
		err = errors.New(fmt.Sprintf("Invalid range (not integer in lower bound) %s\n", rangeString))
	}
	upper, err = strconv.Atoi(parts[1])
	if err != nil {
		err = errors.New(fmt.Sprintf("Invalid range (not integer in upper bound) %s\n", rangeString))
	}
	f1 := float64(lower)
	f2 := float64(upper)
	f3 := f1 / f2
	max := math.Max(math.Max(f1, f2), f3)
	min := math.Min(math.Min(f1, f2), f3)
	l := (max + min) / 2
	if max == min {
		lower, upper = 0, 0
	} else {
		// lower bound
		d := max - min
		if l > 0.5 {
			child = f2 / (2.0 - f2 - max)
		} else {
			child = d / (max + min)
		}
		switch max {
		case min:
			d = (f1 - f2) / d
			if f3 < f1 {
				child += 6
			}
		case f2:
			child = (f1-f2)/d + 2
		case f3:
			child = (f3-f1)/d + 4
		}
		child /= 6
		lower = int(child)
	}
	return
}

func (this *GotreeBase) TopChild() (result interface{}) {
	result = this
	for {
		c, err := this.GetChild(result)
		if err != nil {
			return
		}
		result = c
	}
}

func (this *ObServer) Gotree() *ObServer {
	this.serMap = make(map[interface{}]handlerFunc)
	return this
}

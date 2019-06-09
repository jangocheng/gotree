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
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"time"
)

func ConvertAssign(dest, src interface{}) error {
	// Common cases, without reflect.
	switch s := src.(type) {
	case string:
		switch d := dest.(type) {
		case *string:
			if d == nil {
				return errNilPtr
			}
			*d = s
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = []byte(s)
			return nil
		}
	case []byte:
		switch d := dest.(type) {
		case *string:
			if d == nil {
				return errNilPtr
			}
			*d = string(s)
			return nil
		case *interface{}:
			if d == nil {
				return errNilPtr
			}
			*d = cloneBytes(s)
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = cloneBytes(s)
			return nil
		case *rawBytes:
			if d == nil {
				return errNilPtr
			}
			*d = s
			return nil
		}
	case time.Time:
		switch d := dest.(type) {
		case *string:
			*d = s.Format(time.RFC3339Nano)
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = []byte(s.Format(time.RFC3339Nano))
			return nil
		}
	case nil:
		switch d := dest.(type) {
		case *interface{}:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		case *[]byte:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		case *rawBytes:
			if d == nil {
				return errNilPtr
			}
			*d = nil
			return nil
		}
	}

	var sv reflect.Value

	switch d := dest.(type) {
	case *string:
		sv = reflect.ValueOf(src)
		switch sv.Kind() {
		case reflect.Bool,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
			reflect.Float32, reflect.Float64:
			*d = asString(src)
			return nil
		}
	case *[]byte:
		sv = reflect.ValueOf(src)
		if b, ok := asBytes(nil, sv); ok {
			*d = b
			return nil
		}
	case *rawBytes:
		sv = reflect.ValueOf(src)
		if b, ok := asBytes([]byte(*d)[:0], sv); ok {
			*d = rawBytes(b)
			return nil
		}
	case *bool:
		bv, err := driver.Bool.ConvertValue(src)
		if err == nil {
			*d = bv.(bool)
		}
		return err
	case *interface{}:
		*d = src
		return nil
	}

	if scanner, ok := dest.(Scanner); ok {
		return scanner.Scan(src)
	}

	dpv := reflect.ValueOf(dest)
	if dpv.Kind() != reflect.Ptr {
		return errors.New("destination not a pointer")
	}
	if dpv.IsNil() {
		return errNilPtr
	}

	if !sv.IsValid() {
		sv = reflect.ValueOf(src)
	}

	dv := reflect.Indirect(dpv)
	if sv.IsValid() && sv.Type().AssignableTo(dv.Type()) {
		switch b := src.(type) {
		case []byte:
			dv.Set(reflect.ValueOf(cloneBytes(b)))
		default:
			dv.Set(sv)
		}
		return nil
	}

	if dv.Kind() == sv.Kind() && sv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(sv.Convert(dv.Type()))
		return nil
	}

	switch dv.Kind() {
	case reflect.Ptr:
		if src == nil {
			dv.Set(reflect.Zero(dv.Type()))
			return nil
		} else {
			dv.Set(reflect.New(dv.Type().Elem()))
			return ConvertAssign(dv.Interface(), src)
		}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s := asString(src)
		i64, err := strconv.ParseInt(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetInt(i64)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		s := asString(src)
		u64, err := strconv.ParseUint(s, 10, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetUint(u64)
		return nil
	case reflect.Float32, reflect.Float64:
		s := asString(src)
		f64, err := strconv.ParseFloat(s, dv.Type().Bits())
		if err != nil {
			err = strconvErr(err)
			return fmt.Errorf("converting driver.Value type %T (%q) to a %s: %v", src, s, dv.Kind(), err)
		}
		dv.SetFloat(f64)
		return nil
	}

	return fmt.Errorf("unsupported Scan, storing driver.Value type %T into type %T", src, dest)
}

func asBytes(buf []byte, rv reflect.Value) (b []byte, ok bool) {
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.AppendInt(buf, rv.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.AppendUint(buf, rv.Uint(), 10), true
	case reflect.Float32:
		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 32), true
	case reflect.Float64:
		return strconv.AppendFloat(buf, rv.Float(), 'g', -1, 64), true
	case reflect.Bool:
		return strconv.AppendBool(buf, rv.Bool()), true
	case reflect.String:
		s := rv.String()
		return append(buf, s...), true
	}
	return
}

func strconvErr(err error) error {
	if ne, ok := err.(*strconv.NumError); ok {
		return ne.Err
	}
	return err
}

type Scanner interface {
	Scan(src interface{}) error
}

type GotreeMap struct {
	_map     map[interface{}]interface{}
	lock     sync.Mutex
	openLock bool
}

func (this *GotreeMap) Gotree(openLock ...bool) *GotreeMap {
	this._map = make(map[interface{}]interface{})
	if len(openLock) > 0 && openLock[0] {
		this.openLock = true
	}
	return this
}

func (this *GotreeMap) SetOrStore(key interface{}, value interface{}) (v interface{}, set bool) {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	v, set = this._map[key]
	if set {
		set = false
		return
	}
	set = true
	this._map[key] = value
	v = value
	return
}

func (this *GotreeMap) Set(key interface{}, value interface{}) {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	this._map[key] = value
}

func (this *GotreeMap) Get(key interface{}, value interface{}) error {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	v, ok := this._map[key]
	if !ok {
		return errors.New("undefined")
	}

	return ConvertAssign(value, v)
}

func (this *GotreeMap) Exist(key interface{}) bool {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	_, ok := this._map[key]
	return ok
}

func (this *GotreeMap) Interface(key interface{}) interface{} {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	v, ok := this._map[key]
	if !ok {
		return nil
	}

	return v
}

func (this *GotreeMap) Remove(key interface{}) {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	delete(this._map, key)
}

func (this *GotreeMap) DelAll() {
	all := this.AllKey()
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	for _, item := range all {
		this.Remove(item)
	}
}

func (this *GotreeMap) AllKey() []interface{} {
	if this.openLock {
		defer this.lock.Unlock()
		this.lock.Lock()
	}
	list := make([]interface{}, 0, len(this._map))
	for k, _ := range this._map {
		list = append(list, k)
	}
	return list
}

var errNilPtr = errors.New("destination pointer is nil") // embedded in descriptive error
type rawBytes []byte

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	} else {
		c := make([]byte, len(b))
		copy(c, b)
		return c
	}
}

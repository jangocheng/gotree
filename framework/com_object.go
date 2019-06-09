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
	"reflect"
)

type ComObject struct {
	_map *GotreeMap
}

func (this *ComObject) Gotree() *ComObject {
	this._map = new(GotreeMap).Gotree()
	return this
}

type updateComponent interface {
	UpdateComObject(c *ComObject)
}

func (this *ComObject) Add(obj interface{}) {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		Log().Error("Add != reflect.Ptr")
	}
	this._map.Set(t.Elem().Name(), obj)
	if app, ok := obj.(updateComponent); ok {
		app.UpdateComObject(this)
	}
}

func (this *ComObject) Remove(obj interface{}) {
	t := reflect.TypeOf(obj)
	this._map.Remove(t.Name())
}

func (this *ComObject) Get(obj interface{}) error {
	t := reflect.TypeOf(obj)
	return this._map.Get(t.Elem().Elem().Name(), obj)
}

func (this *ComObject) Broadcast(method string, arg interface{}) {
	list := this._map.AllKey()
	for _, v := range list {
		com := this._map.Interface(v)
		if com == nil {
			continue
		}
		value := reflect.ValueOf(com).MethodByName(method)
		if value.Kind() != reflect.Invalid {
			value.Call([]reflect.Value{reflect.ValueOf(arg)})
		}
	}
}

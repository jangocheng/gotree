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
	"reflect"
)

type ComLocator struct {
	ComEntity
	dict *GotreeMap
}

func (this *ComLocator) Gotree() *ComLocator {
	this.ComEntity.Gotree(this)
	this.dict = new(GotreeMap).Gotree()
	return this
}

func (this *ComLocator) Exist(com interface{}) bool {
	return this.dict.Exist(reflect.TypeOf(com).Elem().String())
}

func (this *ComLocator) Add(obj interface{}) {
	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Ptr {
		Log().Error("Add != reflect.Ptr")
	}
	this.dict.Set(t.Elem().String(), obj)
}

func (this *ComLocator) Remove(obj interface{}) {
	t := reflect.TypeOf(obj)
	this.dict.Remove(t.String())
}

func (this *ComLocator) Get(name string) interface{} {
	return this.dict.Interface(name)
}

func (this *ComLocator) Fetch(obj interface{}) error {
	t := reflect.TypeOf(obj)
	return this.dict.Get(t.Elem().Elem().String(), obj)
}

func (this *ComLocator) Broadcast(fun string, arg interface{}) error {
	list := this.dict.AllKey()
	call := false
	for _, v := range list {
		com := this.dict.Interface(v)
		if com == nil {
			continue
		}
		value := reflect.ValueOf(com).MethodByName(fun)
		if value.Kind() != reflect.Invalid {
			value.Call([]reflect.Value{reflect.ValueOf(arg)})
			call = true
		}
	}
	if !call {
		return errors.New("ComLocator-Broadcast Method not found" + fun)
	}
	return nil
}

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
	"reflect"

	"github.com/8treenet/gotree/framework"
)

type LogInter interface {
	Notice(str ...interface{})
	Debug(str ...interface{})
	Warning(str ...interface{})
	Error(str ...interface{})
}

var (
	status = 0 // 1APP 2DAO
)

func Log() LogInter {
	if status == 0 {
		panic("Not started")
	}
	return framework.Log()
}

func Config() framework.Configer {
	if status == 0 {
		panic("Not started")
	}
	return framework.GetConfig()
}

type comName interface {
	Com() string
}

func appdi(ser reflect.Value, field reflect.StructField) bool {
	var server interface{}
	t := ser.Type()
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		name := t.String()
		server = App().serLocator.Get(name)
	} else {
		impl := field.Tag.Get("impl")
		server = App().inject[impl]
	}

	if server == nil {
		return false
	}
	ser.Set(reflect.ValueOf(server))
	return true
}

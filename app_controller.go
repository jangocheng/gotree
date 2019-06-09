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

// AppController
type AppController struct {
	framework.CallController
}

// AppController
func (this *AppController) Gotree(child interface{}) *AppController {
	this.CallController.Gotree(this)
	this.AddChild(this, child)
	App().gs.Runtime().Set("gseq", App().getGseq())
	return this
}

// Async
func (this *AppController) Async(run func(ac AppAsync), completeds ...func()) {
	var completed func()
	if len(completeds) > 0 {
		completed = completeds[0]
	}
	ac := new(async).Gotree(run, completed)
	go ac.execute()
}

func (this *AppController) OnCreate(method string, argv interface{}) {
	child, err := this.GetChild(this)
	if err != nil {
		return
	}
	for {
		x := reflect.ValueOf(child).Elem()
		tx := reflect.TypeOf(child).Elem()
		for index := 0; index < x.NumField(); index++ {
			value := x.Field(index)
			if (value.Kind() != reflect.Ptr && value.Kind() != reflect.Interface) || !value.IsNil() {
				continue
			}
			appdi(value, tx.Field(index))
		}
		child, err = this.GetChild(child)
		if err != nil {
			return
		}
	}
}

func (this *AppController) OnDestory(method string, reply interface{}, e error) {

}

func (this *AppController) SessionSet(key string, value interface{}) {
	App().gs.Runtime().Set(key, value)
}

func (this *AppController) SessionGet(key string, value interface{}) error {
	return App().gs.Runtime().Eval(key, value)
}

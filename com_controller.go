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
	"reflect"

	"github.com/8treenet/gotree/framework"

	"github.com/8treenet/gotree/framework/model"
)

//ComController
type ComController struct {
	framework.CallController
	thisName string
}

// Gotree
func (this *ComController) Gotree(child interface{}) *ComController {
	this.CallController.Gotree(this)
	this.AddChild(this, child)

	type fun interface {
		CallName_() string
	}
	this.thisName = this.TopChild().(fun).CallName_()
	return this
}

func (this *ComController) OnCreate(method string, argv interface{}) {}

func (this *ComController) OnDestory(method string, reply interface{}, e error) {}

// Model 服务定位器获取model
func (this *ComController) Model(child interface{}) {
	modelDao := reflect.ValueOf(child).Elem().Interface().(comName).Com()
	if this.thisName != modelDao {
		framework.Exit("They do not belong to a set of COM, The " + fmt.Sprint(child))
	}

	err := Dao()._modelLocator.Fetch(child)
	if err != nil {
		framework.Exit("ComController-Model-Service Prohibit invoking " + err.Error())
	}
	return
}

// Cache 服务定位器获取Cache
func (this *ComController) Cache(child interface{}) {
	cacheDao := reflect.ValueOf(child).Elem().Interface().(comName).Com()
	if this.thisName != cacheDao {
		framework.Exit("They do not belong to a set of COM, The " + fmt.Sprint(child))
	}

	err := Dao()._cacheLocator.Fetch(child)
	if err != nil {
		framework.Exit("Illegal error, please check registration, configuration, The " + fmt.Sprint(child))
	}
	return
}

//Api
func (this *ComController) Api(child interface{}) {
	err := Dao()._apiLocator.Fetch(child)
	if err != nil {
		framework.Exit("Illegal error, please check registration, configuration, The " + fmt.Sprint(child))
	}
	return
}

// Memory
func (this *ComController) Memory(child interface{}) {
	apiDao := reflect.ValueOf(child).Elem().Interface().(comName).Com()
	if this.thisName != apiDao {
		framework.Exit("They do not belong to a set of COM, The " + fmt.Sprint(child))
	}

	err := Dao()._memoryLocator.Fetch(child)
	if err != nil {
		framework.Exit("Illegal error, please check registration, configuration, The " + fmt.Sprint(child))
	}
	return
}

// Other
func (this *ComController) Other(child interface{}) {
	apiDao := reflect.ValueOf(child).Elem().Interface().(comName).Com()
	if this.thisName != apiDao {
		framework.Exit("They do not belong to a set of COM, The " + fmt.Sprint(child))
	}

	err := Dao()._otherLocator.Fetch(child)
	if err != nil {
		framework.Exit("Illegal error, please check registration, configuration, The " + fmt.Sprint(child))
	}
	return
}

// Transaction 事务
func (this *ComController) Transaction(fun func() error) error {
	return model.Transaction(this.thisName, fun)
}

// Queue 队列处理
func (this *ComController) Queue(name string, fun func() error) {
	q, ok := Dao().queueMap[this.thisName+"_"+name]
	if !ok {
		framework.Exit("They do not belong to a set of COM or are not registered, The " + name)
	}
	q.cast(fun)
}

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
	"github.com/8treenet/gotree/framework"
)

// Other data sources
type ComOther struct {
	framework.GotreeBase
	comName string
}

func (this *ComOther) Gotree(child interface{}) *ComOther {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.comName = ""
	this.AddEvent("OtherOn", this.otherOn)
	return this
}

func (this *ComOther) DaoInit() {
	if this.comName == "" {
		this.comName = this.TopChild().(comName).Com()
	}
}

// Testing 单元测试
func (this *ComOther) Testing() {
	Dao()
	mode := Config().String("sys::Mode")
	if mode == "prod" {
		framework.Exit("Unit tests cannot be used in a production environment")
	}
	Dao().gs.Runtime().Set("gseq", "ModelUnit")
	this.DaoInit()
	if Config().DefaultString("com_on::"+this.comName, "") == "" {
		framework.Exit("COM not found, please check if the com.conf file is turned on. The " + this.comName)
	}
	this.TopChild().(prepare).OnCreate()
}

type prepare interface {
	OnCreate()
}

//otherOn
func (this *ComOther) otherOn(arg ...interface{}) {
	comName := arg[0].(string)
	if comName == this.comName {
		this.TopChild().(prepare).OnCreate()
	}
}

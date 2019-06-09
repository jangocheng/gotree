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
	"net"
	"reflect"
	"strings"
)

type CallController struct {
	GotreeBase
	gs   *GotreeService
	conn net.Conn
}

//Gotree 构造
func (this *CallController) Gotree(child interface{}) *CallController {
	this.GotreeBase.Gotree(this)
	this.GotreeBase.AddChild(this, child)
	return this
}

func (this *CallController) OnCreate(method string, argv interface{}) {

}

func (this *CallController) OnDestory(method string, reply interface{}, e error) {

}

func (this *CallController) RemoteAddr() string {
	return this.conn.RemoteAddr().String()
}

func (this *CallController) FUCK_YOU_(cmd int, result *int) error {
	return nil
}

func (this *CallController) CallInvoke_(conn net.Conn, gs *GotreeService) {
	this.gs = gs
	this.conn = conn
	return
}

//用于rpcserver 注册,不可重写
func (this *CallController) CallName_() string {
	//获取顶级子类
	child := this.TopChild()
	controller := reflect.TypeOf(child).Elem().String()
	name := this.controllerName(controller)
	if name == "" {
		Log().Error("CallController-CallName_:rpc controller is unnormal:" + controller)
	}
	return name
}

//controllerName 切割名字
func (this *CallController) controllerName(name string) string {
	list := strings.Split(name, ".")
	if len(list) < 2 {
		return ""
	}

	list = strings.Split(list[len(list)-1], "Controller")
	if len(list) < 1 {
		return ""
	}
	return list[0]
}

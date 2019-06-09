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
	"os"
	"strconv"
	"strings"

	"github.com/8treenet/gotree/framework"
)

type AppService struct {
	framework.GotreeBase
	_head framework.CmdHeader
}

func (this *AppService) Gotree(child interface{}) *AppService {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	return this
}

func (this *AppService) CallDao(obj interface{}, reply interface{}) error {
	var client *framework.CallClient
	if e := App().sysLocator.GetComObject(&client); e != nil {
		return e
	}
	return client.Do(obj, reply)
}

func (this *AppService) Testing(coms ...string) {
	var client *framework.CallClient
	App().openLog()
	App().sysLocator.GetComObject(&client)
	client.Start()
	mode := framework.GetConfig().String("sys::Mode")
	//生产环境不可进行单元测试
	if mode == "prod" {
		framework.Log().Error("Unit tests cannot be used in a production environment")
		os.Exit(-1)
	}
	App().gs.Runtime().Set("gseq", "ServiceUnit")

	var im *framework.CallRoute
	App().sysLocator.GetComObject(&im)

	for _, com := range coms {
		comId := strings.Split(com, ":")
		id, _ := strconv.Atoi(comId[1])
		im.LocalAddCom(comId[0], "127.0.0.1", "4000", id)
	}
	return
}

func (this *AppService) SessionSet(key string, value interface{}) {
	App().gs.Runtime().Set(key, value)
}

func (this *AppService) SessionGet(key string, value interface{}) error {
	return App().gs.Runtime().Eval(key, value)
}

func (this *AppService) InjectImpl(name string, service interface{}) {
	App().inject[name] = service
}

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
	"runtime"
	"strconv"
	"strings"

	"github.com/8treenet/gotree/framework"
	"github.com/8treenet/gotree/framework/model"
)

type ComCache struct {
	framework.GotreeBase
	open    bool
	comName string
}

func (this *ComCache) Gotree(child interface{}) *ComCache {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.AddEvent("DaoTelnet", this.daoTelnet)
	this.AddEvent("CacheOn", this.cacheOn)
	this.comName = ""
	return this
}

//Testing 单元测试
func (this *ComCache) Testing() {
	Dao()
	mode := Config().String("sys::Mode")
	if mode == "prod" {
		framework.Exit("Unit tests cannot be used in a production environment")
	}
	this.DaoInit()
	if Config().DefaultString("com_on::"+this.comName, "") == "" {
		framework.Exit("COM not found, please check if the com.conf file is turned on. The " + this.comName)
	}
	this.redisOn()
}

func (this *ComCache) daoTelnet(args ...interface{}) {
	dao := this.TopChild().(comName)
	comName := dao.Com()

	for _, arg := range args {
		dao := arg.(comNode)
		if dao.Name == comName {
			this.redisOn()
			return
		}
	}
}

// cacheOn
func (this *ComCache) cacheOn(arg ...interface{}) {
	comName := arg[0].(string)
	if comName == this.comName {
		this.redisOn()
	}
}

// redisOn
func (this *ComCache) redisOn() {
	this.open = true
	if !Dao().connDB(this.comName + "model") {
		return
	}
	redisinfo := Config().String("redis::" + this.comName)
	if redisinfo == "" {
		Log().Error("Cache not found, please check if the cache.conf file is turned on. The " + this.comName)
	}
	list := strings.Split(redisinfo, ";")
	m := map[string]string{}
	for _, item := range list {
		kv := strings.Split(item, "=")
		if len(kv) != 2 {
			Log().Error("Cache not found, please check if the cache.conf file is turned on. The " + this.comName)
			continue
		}
		m[kv[0]] = kv[1]
	}

	client := model.GetClient(this.comName)
	if client != nil {
		//已注册
		return
	}

	imaxIdleConns := Config().DefaultInt("redis::"+this.comName+"MaxIdleConns", runtime.NumCPU()*2)
	imaxOpenConns := Config().DefaultInt("redis::"+this.comName+"MaxOpenConns", runtime.NumCPU()*2)
	db, _ := strconv.Atoi(m["database"])
	Log().Notice("ComCache Connect com " + this.comName + " redis," + " MaxIdleConns:" + fmt.Sprint(imaxIdleConns) + " MaxOpenConns:" + fmt.Sprint(imaxOpenConns) + " config:" + fmt.Sprint(m))
	client, e := model.NewCache(m["server"], m["password"], db, imaxIdleConns, imaxOpenConns)
	if e != nil {
		framework.Exit(e.Error())
	}
	model.AddDatabase(this.comName, client)
}

//Do
func (this *ComCache) Do(cmd string, args ...interface{}) (reply interface{}, e error) {
	if !this.open || this.comName == "" {
		framework.Exit("Illegal use of Cache, The " + this.comName)
	}
	reply, e = model.Do(this.comName, cmd, args...)
	return
}

func (this *ComCache) Connections(m map[string]int) {
	if !this.open {
		return
	}
	n := model.GetConnects(this.comName)
	m[this.comName] = n
}

func (this *ComCache) DaoInit() {
	if this.comName == "" {
		this.comName = this.TopChild().(comName).Com()
	}
}

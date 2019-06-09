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
	"net/url"
	"strings"
	"sync"

	"time"

	"github.com/8treenet/gotree/framework"
	"github.com/8treenet/gotree/helper"
)

type Request func(request *helper.HTTPRequest)

type ComApi struct {
	framework.GotreeBase
	open       bool
	apiName    string
	host       string
	countMutex sync.Mutex
}

func (this *ComApi) Gotree(child interface{}) *ComApi {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.apiName = ""
	this.apiOn()
	return this
}

//Testing 单元测试
func (this *ComApi) Testing() {
	Dao()
	mode := Config().String("sys::Mode")
	if mode == "prod" {
		framework.Exit("Unit tests cannot be used in a production environment")
	}
	this.apiOn()
}

type apiName interface {
	Api() string
}

//apiOn
func (this *ComApi) apiOn() {
	this.open = true
	this.apiName = this.TopChild().(apiName).Api()
	this.host = Config().String("api::" + this.apiName)
}

//HttpGet
func (this *ComApi) HttpGet(apiAddr string, args map[string]interface{}, callback ...Request) (result []byte, e error) {
	req := helper.HttpGet(this.host + apiAddr + "?" + httpBuildQuery(args))
	req = req.SetTimeout(3*time.Second, 10*time.Second)
	fun := func() error {
		var reqerr error
		result, reqerr = req.Bytes()
		return reqerr
	}

	if len(callback) > 0 {
		callback[0](req)
	}

	mode := Config().String("sys::Mode")
	//2次重试
	for index := 0; index < 2; index++ {
		e = Dao().apiLimit.Go(fun)
		if mode == "dev" {
			Log().Notice("ApiGet:", this.host+apiAddr, "ReqData:", args, "ResData:", string(result), "error:", e)
		}
		if e != nil {
			continue
		}

		break
	}

	return
}

//HttpPost
func (this *ComApi) HttpPost(apiAddr string, args map[string]interface{}, callback ...Request) (result []byte, e error) {
	req := helper.HttpPost(this.host + apiAddr)
	req = req.SetTimeout(3*time.Second, 10*time.Second)
	for k, v := range args {
		req.Param(k, fmt.Sprint(v))
	}

	if len(callback) > 0 {
		callback[0](req)
	}

	fun := func() error {
		var reqerr error
		result, reqerr = req.Bytes()
		return reqerr
	}

	mode := Config().String("sys::Mode")
	//2次重试
	for index := 0; index < 2; index++ {
		e = Dao().apiLimit.Go(fun)
		if mode == "dev" {
			Log().Notice("ApiPost:", this.host+apiAddr, "ReqData:", args, "ResData:", string(result), "error:", e)
		}
		if e != nil {
			continue
		}

		break
	}

	return
}

//HttpPostJson
func (this *ComApi) HttpPostJson(apiAddr string, raw interface{}, callback ...Request) (result []byte, e error) {
	req := helper.HttpPost(this.host + apiAddr)
	req = req.SetTimeout(3*time.Second, 10*time.Second)
	req.JSONBody(raw)

	if len(callback) > 0 {
		callback[0](req)
	}

	fun := func() error {
		var reqerr error
		result, reqerr = req.Bytes()
		return reqerr
	}

	mode := Config().String("sys::Mode")
	//2次重试
	for index := 0; index < 2; index++ {
		e = Dao().apiLimit.Go(fun)
		if mode == "dev" {
			Log().Notice("ApiPostJson:", this.host+apiAddr, "ReqData:", raw, "ResData:", string(result), "error:", e)
		}
		if e != nil {
			continue
		}

		break
	}

	return
}

// HostAddr 获取本api dao的host地址
func (this *ComApi) HostAddr() string {
	return this.host
}

// HttpBuildQuery转换get参数
func httpBuildQuery(args map[string]interface{}) (result string) {
	for k, v := range args {
		result += k + "=" + fmt.Sprint(v) + "&"
	}

	return url.PathEscape(strings.TrimSuffix(result, "&"))
}

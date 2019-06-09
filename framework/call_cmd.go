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
	"net/http"
	"strings"

	"github.com/8treenet/gotree/helper"
)

type CmdHeader struct {
}

func (this *CmdHeader) Set(src, key, val string) string {
	if src != "" {
		src += "??"
	}
	src += key + "::" + val
	return src
}

func (this *CmdHeader) Get(src, key string) string {
	list := strings.Split(src, "??")
	for _, kvitem := range list {
		kv := strings.Split(kvitem, "::")
		if len(kv) != 2 {
			continue
		}
		if kv[0] == key {
			return kv[1]
		}
	}
	return ""
}

type CallCmd struct {
	GotreeBase
	Gseq string `opt:"null"`
	Head string `opt:"null"`
}

func (this *CallCmd) Header(k string) string {
	return _header.Get(this.Head, k)
}

func (this *CallCmd) SetHeader(k string, v string) {
	_, e := this.GetChild(this)
	if e != nil {
		panic("CallCmd-SetHeader :Header is read-only data")
	}
	this.Head = _header.Set(this.Head, k, v)
}

func (this *CallCmd) SetHttpHeader(head http.Header) {
	_, e := this.GetChild(this)
	if e != nil {
		panic("CallCmd-SetHttpHeader :Header is read-only data")
	}
	for item := range head {
		this.SetHeader(item, head.Get(item))
	}
}

type ComNode interface {
	RandomAddr() string                       //随机地址
	BalanceAddr() string                      //负载均衡地址
	HostHashRpcAddr(value interface{}) string //热一致性哈希地址
	HashRpcAddr(value interface{}) string     //一致性哈希地址
	SlaveAddr() string                        //返回随机从节点  主节点:节点id=1,当只有主节点返回主节点
	MasterAddr() string                       //返回主节点
	AllCom() (list []*Panel)                  //获取全部节点,自定义分发
}

type cmdChild interface {
	Control() string
	Action() string
	ComAddr(ComNode) string
}

type cmdSerChild interface {
	Control() string
	Action() string
}

type nodeAddr interface {
	GetAddrList(string) *panels
}

func (this *CallCmd) Gotree(child interface{}) *CallCmd {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	if _runtimeStack == nil {
		return this
	}
	gseq := _runtimeStack.Get("gseq")
	if gseq == nil {
		return this
	}
	str, ok := gseq.(string)
	if ok {
		this.Gseq = str
	}
	return this
}

//client调用RemoteAddr 传入NodeMaster, 获取远程地址
func (this *CallCmd) RemoteAddr(naddr interface{}) (string, error) {
	child := this.TopChild()
	childObj, ok := child.(cmdChild)
	if !ok {
		objName := helper.Name(child)
		panic("CallCmd-RemoteAddr:Subclass is not implemented,interface :" + objName)
	}

	serName := childObj.Control()
	node := naddr.(nodeAddr)
	nm := node.GetAddrList(serName)
	if nm == nil || nm.Len() == 0 {
		return "", ErrNetwork
	}

	//传入子类该服务远程地址列表,计算要使用的节点
	return childObj.ComAddr(nm), nil
}

func (this *CallCmd) ServiceMethod() string {
	child := this.TopChild()
	childObj, ok := child.(cmdSerChild)
	if !ok {

		className := helper.Name(child)
		panic("CallCmd-ServiceMethod:Subclass is not implemented, interface :" + className)
	}

	return childObj.Control() + "." + childObj.Action()
}

var _header CmdHeader

type _cmdInrer interface {
	RemoteAddr(interface{}) (string, error)
	ServiceMethod() string
}

type gatewayInter interface {
	AppAddr() string
}

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
package gateway

import (
	"github.com/8treenet/gotree/framework"
)

type gateway struct {
	framework.GotreeBase
	sysLoc *framework.ComLocator
	cli    *framework.CallClient
}

func (this *gateway) Gotree() *gateway {
	this.GotreeBase.Gotree(this)
	this.sysLoc = new(framework.ComLocator).Gotree()
	this.sysLoc.AddComObject(new(framework.CallRoute).Gotree())
	this.sysLoc.AddComObject(new(framework.HealthClient).Gotree())
	return this
}

var _gateway *gateway

func init() {
	_gateway = new(gateway).Gotree()
}

func (this *gateway) AppendApp(addr string) {
	var ic *framework.HealthClient
	this.sysLoc.GetComObject(&ic)
	ic.AddRemoteAddr(addr)
}

func (this *gateway) Run(args ...int) {
	var (
		ic          *framework.HealthClient
		concurrency int = 8192
		callTimeout int = 12
	)
	if len(args) > 0 {
		concurrency = args[0]
	}
	if len(args) > 1 {
		callTimeout = args[1]
	}
	this.sysLoc.GetComObject(&ic)
	this.cli = new(framework.CallClient).Gotree(concurrency, callTimeout)
	this.sysLoc.AddComObject(this.cli)
	rpcBreak := new(framework.CallBreak).Gotree()
	this.sysLoc.AddComObject(rpcBreak)
	rpcBreak.RunTick()
	this.cli.Start()
	go ic.Start()
}

func (this *gateway) CallApp(inArg interface{}, reply interface{}) error {
	return this.cli.Do(inArg, reply)
}

type Invoker interface {
	CallApp(interface{}, interface{}) error
	Run(args ...int)
	AppendApp(string)
}

func Sington() Invoker {
	return _gateway
}

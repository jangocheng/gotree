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
	"runtime"
	"strings"

	"github.com/8treenet/gotree/framework"
	"github.com/8treenet/gotree/helper"
)

type AppTimer struct {
	framework.GotreeBase
	timerCallBack []trigger
	timerStopTick []framework.StopTick
	_openService  bool
}

func (this *AppTimer) Gotree(child interface{}) *AppTimer {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.timerCallBack = make([]trigger, 0)
	this.AddEvent("TimerOpen", this.timer)
	this.AddEvent("system_shutdown", this.stopTick)
	this._openService = false
	this.timerStopTick = make([]framework.StopTick, 0)
	return this
}

func (this *AppTimer) RegTick(ms int, fun func(), delay ...int) {
	delayms := 0
	if len(delay) > 0 {
		delayms = delay[0]
	}
	this.timerCallBack = append(this.timerCallBack, trigger{t: ms, tickFun: fun, delay: delayms})
	return
}

func (this *AppTimer) RegDay(hour, minute int, fun func()) {
	if hour < 0 && hour > 23 {
		framework.Log().Error("Hours are incorrect, range: 0-23")
	}

	if minute < 0 && minute > 59 {
		framework.Log().Error("Minute incorrect, range: 0-59")
	}
	this.timerCallBack = append(this.timerCallBack, trigger{t: hour, t2: minute, dayFun: fun})
	return
}

func (this *AppTimer) di() {
	child, err := this.GetChild(this)
	if err != nil {
		return
	}
	for {
		x := reflect.ValueOf(child).Elem()
		tx := reflect.TypeOf(child).Elem()
		for index := 0; index < x.NumField(); index++ {
			value := x.Field(index)
			if value.Kind() != reflect.Ptr || !value.IsNil() {
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

func (this *AppTimer) stopTick(args ...interface{}) {
	for _, stopFun := range this.timerStopTick {
		stopFun.Stop()
	}
}

func (this *AppTimer) timer(args ...interface{}) {
	oname := helper.Name(this.TopChild())
	open := false
	for _, namei := range args {
		name := namei.(string)
		if name == oname {
			open = true
			break
		}
	}

	if !open {
		return
	}
	this.di()

	for _, fun := range this.timerCallBack {
		if fun.tickFun != nil {
			funName := oname + "."
			funcFor := runtime.FuncForPC(reflect.ValueOf(fun.tickFun).Pointer()).Name()
			if list := strings.Split(funcFor, "."); len(list) > 0 {
				funName += list[len(list)-1]
			}
			if list := strings.Split(funName, "-"); len(list) > 0 {
				funName = list[0]
			}

			tick := framework.RunTick(int64(fun.t), fun.tickFun, funName, fun.delay)
			this.timerStopTick = append(this.timerStopTick, tick)
		}

		if fun.dayFun != nil {
			funName := oname + "."
			funcFor := runtime.FuncForPC(reflect.ValueOf(fun.dayFun).Pointer()).Name()
			if list := strings.Split(funcFor, "."); len(list) > 0 {
				funName += list[len(list)-1]
			}
			if list := strings.Split(funName, "-"); len(list) > 0 {
				funName = list[0]
			}
			framework.RunDay(fun.t, fun.t2, fun.dayFun, funName)
		}
	}
	return
}

func (this *AppTimer) OpenTimer() {
	if App().timeLocator.Exist(this.TopChild()) {
		framework.Exit("AppTimer-AppTimer Prohibit duplicate instantiation")
	}
	this._openService = true
	return
}

// Async
func (this *AppTimer) Async(run func(ac AppAsync), completeds ...func()) {
	var completed func()
	if len(completeds) > 0 {
		completed = completeds[0]
	}
	ac := new(async).Gotree(run, completed)
	go ac.execute()
}

type trigger struct {
	t       int
	tickFun func()
	dayFun  func()
	t2      int
	delay   int
}

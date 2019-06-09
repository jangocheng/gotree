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
	"math"
	"math/rand"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/8treenet/gotree/helper"

	"github.com/8treenet/gotree/framework"
)

type Application struct {
	gs          *framework.GotreeService
	sysLocator  *framework.ComLocator
	timeLocator *framework.ComLocator
	serLocator  *framework.ComLocator
	runAsyncNum int32
	runStart    bool
	gseq        int64
	uuid        string
	seqLock     sync.Mutex
	inject      map[string]interface{}
}

var _app *Application

func App() *Application {
	if _app == nil {
		_app = new(Application)
		_app.init()
	}
	return _app
}

func (this *Application) init() {
	if status != 0 {
		framework.Log().Error("The program has been starte")
		os.Exit(-1)
	}
	this.gs = new(framework.GotreeService).Gotree()
	status = 1
	this.runStart = false
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	rand.Seed(time.Now().Unix())
	this.openLog()
	this.start()

	this.sysLocator = new(framework.ComLocator).Gotree()
	this.timeLocator = new(framework.ComLocator).Gotree()
	this.serLocator = new(framework.ComLocator).Gotree()

	this.sysLocator.AddComObject(new(framework.CallRoute).Gotree())
	health := new(framework.HealthController).Gotree()
	cclient := new(framework.CallClient).Gotree(framework.GetConfig().DefaultInt("sys::CallDaoConcurrency", 6000), framework.GetConfig().DefaultInt("sys::CallDaoTimeout", 12))
	this.sysLocator.AddComObject(cclient)
	this.sysLocator.AddComObject(new(framework.CallQps).Gotree())
	this.sysLocator.AddComObject(new(framework.CallBreak).Gotree())
	this.gs.RegController(health)
	helper.SetRunStack(this.gs.Runtime())
	framework.App = this
	this.uuid = strconv.FormatInt(int64(rand.Intn(10000)), 36)
	this.gseq = 666

	this.inject = make(map[string]interface{})
}

func (this *Application) Run() {
	if this.runStart || status != 1 {
		framework.Log().Error("The program has been starte")
		os.Exit(-1)
	}
	this.gs.Runtime().Remove()
	this.view()
	this.runTime()
	bindAddr := framework.GetConfig().String("dispersed::BindAddr")

	framework.Log().Notice("App run ....")
	var client *framework.CallClient
	var breaker *framework.CallBreak
	this.sysLocator.GetComObject(&client)
	this.sysLocator.GetComObject(&breaker)
	this.sysLocator.Event("system_run")
	breaker.RunTick()
	client.Start()
	this.gs.Run(bindAddr)
	this.sysLocator.Event("system_shutdown")
	breaker.StopTick()

	//优雅关闭 至多 6秒
	for index := 0; index < 6; index++ {
		num := this.gs.Calls()
		timenum := framework.CurrentTimeNum()
		anum := this.AsyncNum()
		framework.Log().Notice("App is shutting down, Timing service surplus:", timenum, "Request service surplus:", num, "Async service surplus:", anum)
		if num <= 0 && anum <= 0 && timenum <= 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	this.gs.Close()
	client.Close()
	framework.Log().Close()
}

func (this *Application) runTime() {
	list := strings.Split(framework.GetConfig().String("timer::Open"), ",")
	var times []interface{}
	for _, item := range list {
		times = append(times, item)
	}
	this.timeLocator.Event("TimerOpen", times...)
}

func (this *Application) AsyncNum() (result int) {
	result = int(atomic.LoadInt32(&this.runAsyncNum))
	return
}

func (this *Application) addAsync(value int32) {
	atomic.AddInt32(&this.runAsyncNum, value)
}

func (this *Application) start() {
	addr := framework.GetConfig().String("dispersed::BindAddr")
	if addr == "" {
		panic("undefined 'dispersed::BindAddr'")
	}
	array := strings.Split(addr, ":")
	port, _ := strconv.Atoi(array[1])
	if helper.InSlice(os.Args, "daemon") {
		framework.AppDaemon()
		os.Exit(0)
		return
	}

	if helper.InSlice(os.Args, "restart") || helper.InSlice(os.Args, "start") {
		framework.AppRestart("app", array[0], port)
		os.Exit(0)
		return
	}
	if helper.InSlice(os.Args, "stop") {
		framework.AppStop("app", array[0], port)
		os.Exit(0)
		return
	}
}

func (this *Application) view() {
	if helper.InSlice(os.Args, "status") {
		this.viewRuntime()
		this.viewDaos()
		os.Exit(0)
		return
	}
	if helper.InSlice(os.Args, "qps") {
		count := 1
		if helper.InSlice(os.Args, "-t") {
			count = 180
		}
		for index := 0; index < count; index++ {
			c := exec.Command("clear")
			c.Stdout = os.Stdout
			c.Run()
			this.viewQps(count)
			time.Sleep(1 * time.Second)
		}
		os.Exit(0)
		return
	}
}

func (this *Application) viewDaos() {
	client, _ := this.healthClient()
	var replys string
	client.Call("Health.DaoServerStatus", 100, &replys)
	fmt.Println(fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", 34, "Remote DAO Information:"))
	for _, str := range strings.Split(replys, ";") {
		fmt.Println(str)
	}

	replys = ""
	client.Call("Health.ComStatus", 100, &replys)
	client.Close()
	fmt.Println(fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", 34, "Remote COM Information:"))
	for _, str := range strings.Split(replys, ";") {
		fmt.Println(str)
	}
}

func (this *Application) viewRuntime() {
	client, addr := this.healthClient()
	if client == nil {
		fmt.Println(fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", 31, "App program not started, or check configuration"))
		os.Exit(0)
	}
	var replys string
	client.Call("Health.AppInfo", 100, &replys)
	var pid int
	client.Call("Health.ProcessId", 100, &pid)
	fmt.Println(fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", 34, "Listening address: "+addr+", pid:"+fmt.Sprint(pid)))
	fmt.Println(replys)
	client.Close()
}

func (this *Application) viewQps(top int) {
	client, _ := this.healthClient()
	if client == nil {
		fmt.Println(fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", 31, "App program not started, or check configuration"))
		os.Exit(0)
	}
	var replys string
	var restart string
	client.Call("Health.ComQps", 100, &replys)
	client.Call("Health.ComQpsBeginTime", 100, &restart)
	client.Close()
	if top == 1 {
		fmt.Printf("%46s\n", "Com qps clear: "+restart)
		for _, str := range strings.Split(replys, "??") {
			fmt.Println(str)
		}
		return
	}
	fmt.Printf("%46s\n", "Com top 30 qps clear: "+restart)
	listqps := strings.Split(replys, "??")
	if len(listqps) > 30 {
		listqps = listqps[0:30]
	}
	for _, str := range listqps {
		fmt.Println(str)
	}
}

func (this *Application) healthClient() (*rpc.Client, string) {
	addr := framework.GetConfig().String("dispersed::BindAddr")
	if addr == "" {
		return nil, addr
	}
	list := strings.Split(addr, ":")
	port, _ := strconv.Atoi(list[1])
	for index := 0; index < 10; index++ {
		client, err := jsonrpc.Dial("tcp", fmt.Sprintf("%s:%d", list[0], port+index))
		if err != nil {
			continue
		}
		var pid int
		if client.Call("Health.ProcessId", 100, &pid) == nil {
			return client, fmt.Sprintf("%s:%d", list[0], port+index)
		}
		client.Close()
	}
	return nil, ""
}

// RegController
func (this *Application) RegController(controller interface{}) {
	if this.runStart {
		framework.Log().Error("The program has been starte")
		os.Exit(-1)
	}
	this.gs.RegController(controller)
}

// RegTimer
func (this *Application) RegTimer(service interface{}) {
	if this.runStart {
		framework.Log().Error("The program has been starte")
		os.Exit(-1)
	}
	type openService interface {
		OpenTimer()
	}

	os := service.(openService)
	os.OpenTimer()
	this.timeLocator.Add(service)
}

func (this *Application) getGseq() (gseqResult string) {
	defer this.seqLock.Unlock()
	this.seqLock.Lock()
	gseqResult = this.uuid
	gseqResult += strconv.FormatInt(this.gseq, 36)
	if this.gseq == math.MaxInt64 {
		this.gseq = 666
		return
	}
	this.gseq += 1
	return
}

var _head framework.CmdHeader

// ReqHeader
func (this *Application) ReqHeader(k string) string {
	value := this.gs.Runtime().Get("head")
	if value == nil {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return _head.Get(str, k)
}

func (this *Application) openLog() {
	framework.LoadConfig("app")
	dir := framework.GetConfig().DefaultString("sys::LogDir", "log")
	framework.Log().Init(dir)
	return
}

func (this *Application) RegisterService(child interface{}) {
	if this.serLocator.Exist(child) {
		panic("Service already exist, The " + fmt.Sprint(child))
	}
	this.serLocator.Add(child)
}

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
	"net"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/8treenet/gotree/helper"

	"github.com/8treenet/gotree/framework"
)

var _dao *DAO

func Dao() *DAO {
	if _dao == nil {
		_dao = new(DAO).Gotree()
		_dao.init()
	}
	return _dao
}

type comNode struct {
	Name  string
	Id    int
	Extra []interface{}
}
type DAO struct {
	framework.GotreeBase
	gs             *framework.GotreeService
	_modelLocator  *framework.ComLocator //模型
	_cacheLocator  *framework.ComLocator //缓存
	_memoryLocator *framework.ComLocator //内存
	_apiLocator    *framework.ComLocator //api
	_otherLocator  *framework.ComLocator //其他
	openCom        []comNode
	mysqlProfiler  bool
	queueMap       map[string]*daoQueue
	apiLimit       *framework.Limiting
	explainMap     *framework.GotreeMap
	connDBMap      *framework.GotreeMap
}

func (this *DAO) Gotree() *DAO {
	this.GotreeBase.Gotree(this)
	this.explainMap = new(framework.GotreeMap).Gotree(true)
	this.connDBMap = new(framework.GotreeMap).Gotree(true)
	return this
}

func (this *DAO) init() {
	if status != 0 {
		framework.Log().Error("The program has been starte")
		os.Exit(-1)
	}
	this.gs = new(framework.GotreeService).Gotree()
	status = 2
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	this.openLog()
	if framework.GetConfig().DefaultBool("sys::MysqlProfiler", false) {
		this.mysqlProfiler = true
	}
	this.start()
	this.queueMap = make(map[string]*daoQueue)

	this._modelLocator = new(framework.ComLocator).Gotree()
	this._cacheLocator = new(framework.ComLocator).Gotree()
	this._memoryLocator = new(framework.ComLocator).Gotree()
	this._otherLocator = new(framework.ComLocator).Gotree()
	this._apiLocator = new(framework.ComLocator).Gotree()

	helper.SetRunStack(this.gs.Runtime())
	this.apiLimit = new(framework.Limiting).Gotree(Config().DefaultInt("sys::ApiConcurrency", 1024))

	innerRpcServer := new(framework.HealthController).Gotree()
	this.gs.RegController(innerRpcServer)
}

func (this *DAO) openLog() {
	framework.LoadConfig("dao")
	dir := framework.GetConfig().DefaultString("sys::LogDir", "log")
	framework.Log().Init(dir)
	return
}

func (this *DAO) start() {
	addr := Config().String("dispersed::BindAddr")
	if addr == "" {
		panic("undefined BindAddr")
	}
	list := strings.Split(addr, ":")
	port, _ := strconv.Atoi(list[1])
	if helper.InSlice(os.Args, "daemon") {
		framework.AppDaemon()
		os.Exit(0)
		return
	}
	if helper.InSlice(os.Args, "restart") || helper.InSlice(os.Args, "start") {
		framework.AppRestart("dao", list[0], port)
		os.Exit(0)
		return
	}
	if helper.InSlice(os.Args, "stop") {
		framework.AppStop("dao", list[0], port)
		os.Exit(0)
		return
	}
}

// RegController
func (this *DAO) RegController(controller interface{}) {
	if helper.Testing() {
		return
	}
	this.gs.RegController(controller)
}

// RegModel
func (this *DAO) RegModel(service interface{}) {
	if helper.Testing() {
		return
	}
	type init interface {
		DaoInit()
	}
	service.(init).DaoInit()
	if this._modelLocator.Exist(service) {
		framework.Exit("RegModel duplicate registration")
	}
	this._modelLocator.Add(service)
}

// RegCache
func (this *DAO) RegCache(service interface{}) {
	if helper.Testing() {
		return
	}
	type init interface {
		DaoInit()
	}
	service.(init).DaoInit()
	if this._cacheLocator.Exist(service) {
		framework.Exit("RegCache duplicate registration")
	}
	this._cacheLocator.Add(service)
}

// RegMemory
func (this *DAO) RegMemory(service interface{}) {
	if helper.Testing() {
		return
	}
	type init interface {
		DaoInit()
	}
	service.(init).DaoInit()
	if this._memoryLocator.Exist(service) {
		framework.Exit("RegMemory duplicate registration")
	}
	this._memoryLocator.Add(service)
}

// RegApi
func (this *DAO) RegApi(service interface{}) {
	if helper.Testing() {
		return
	}
	if this._apiLocator.Exist(service) {
		framework.Exit("RegApi duplicate registration")
	}
	this._apiLocator.Add(service)
}

// RegOther
func (this *DAO) RegOther(service interface{}) {
	if helper.Testing() {
		return
	}
	type init interface {
		DaoInit()
	}
	service.(init).DaoInit()
	if this._otherLocator.Exist(service) {
		framework.Exit("RegOther duplicate registration")
	}
	this._otherLocator.Add(service)
}

// RegQueue
func (this *DAO) RegQueue(controller interface{}, queueName string, queueLen int, goroutine ...int) {
	type rpcname interface {
		CallName_() string
	}

	rc, ok := controller.(rpcname)
	if !ok {
		framework.Exit("RegQueue error")
	}
	dao := rc.CallName_()
	mgo := 1
	if len(goroutine) > 0 && goroutine[0] > 0 {
		mgo = goroutine[0]
	}
	q := new(daoQueue).Gotree(queueLen, mgo, dao, queueName)
	go q.mainRun()
	this.queueMap[dao+"_"+queueName] = q
}

func (this *DAO) comRun() {
	openDao, err := Config().GetSection("com_on")
	if err != nil {
		framework.Exit("daoOn-openDao Not found com.conf com_on:" + err.Error())
	}
	comNames := this.gs.Actions()
	for k, v := range openDao {
		id, e := strconv.Atoi(v)
		if e != nil {
			Log().Error("daoOn-openDao dao id error:", k, v)
			continue
		}
		var comName string
		for index := 0; index < len(comNames); index++ {
			if strings.ToLower(comNames[index]) == k {
				comName = comNames[index]
			}
		}
		if comName == "" {
			Log().Error("daoOn-openDao error:Not found com:", k)
			continue
		}

		extra := []interface{}{}
		extraList := strings.Split(Config().String("com_extra::"+comName), ",")
		for _, item := range extraList {
			extra = append(extra, item)
		}
		this.openCom = append(this.openCom, comNode{Name: comName, Id: id, Extra: extra})
	}
}

func (this *DAO) Run(args ...interface{}) {
	bindAddr := Config().String("dispersed::BindAddr")
	tick := framework.RunTick(1000, func() {
		this._memoryLocator.Broadcast("MemoryTimeout", time.Now().Unix())
	}, "memoryTimeout", 3000)
	this.comRun()
	this.telnet()
	hc := new(framework.HealthClient).Gotree()
	this.startHealth(hc)

	//通知所有model dao关联
	task := helper.NewGroup()
	for _, comItem := range this.openCom {
		tempItem := comItem
		task.Add(func() error {
			this.Event("ModelOn", tempItem.Name)
			this.Event("CacheOn", tempItem.Name)
			this.Event("MemoryOn", tempItem.Name)
			this.Event("ApiOn", tempItem.Name)
			this.Event("OtherOn", tempItem.Name)
			return nil
		})
	}
	if e := task.Wait(); e != nil {
		framework.Exit(e.Error())
	}
	this.Event("system_run")
	for _, action := range this.gs.Actions() {
		find := false
		for _, item := range this.openCom {
			if action == item.Name {
				Log().Notice("startup:", action, "id:", item.Id)
				find = true
				break
			}
		}
		if find {
			continue
		}
		this.gs.CancelController(action)
	}
	this.gs.Run(bindAddr)
	hc.Close()

	//优雅关闭 至多 6秒
	for index := 0; index < 6; index++ {
		num := this.gs.Calls()
		qlen := this.queueLen()
		Log().Notice("Run dao close: Request service surplus:", num, "Queue surplus:", qlen)
		if num <= 0 && qlen <= 0 {
			break
		}
		time.Sleep(time.Second * 1)
	}
	this.Event("system_shutdown")
	this.gs.Close()
	tick.Stop()
	framework.Log().Close()
}

func (this *DAO) telnet() {
	if !helper.InSlice(os.Args, "telnet") {
		return
	}
	framework.Log().Debug()
	var array []interface{}
	helper.NewSlice(&array, len(this.openCom))
	for index := 0; index < len(this.openCom); index++ {
		array[index] = this.openCom[index]
	}
	this._modelLocator.Event("DaoTelnet", array...)
	arddrs := Config().String("dispersed::AppAddrs")
	if arddrs == "" {
		Log().Warning("telnet-AppAddrs baddrs address is empty.")
		time.Sleep(500 * time.Millisecond)
		os.Exit(0)
	}

	list := strings.Split(arddrs, ",")
	for _, item := range list {
		_, err := net.DialTimeout("tcp", item, time.Duration(2*time.Second))
		if err != nil {
			Log().Warning("telnet-AppAddrs connection failed", item)
			time.Sleep(500 * time.Millisecond)
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func (this *DAO) queueLen() (result int) {
	for _, q := range this.queueMap {
		result += len(q.queue)
	}
	return
}

func (this *DAO) startHealth(ic *framework.HealthClient) {
	baddrs := Config().String("dispersed::AppAddrs")
	if baddrs == "" {
		Log().Error("Health Client-AppAddrs baddrs address is empty.")
	}
	for index := 0; index < len(this.openCom); index++ {
		ic.AddDaoByNode(this.openCom[index].Name, this.openCom[index].Id, this.openCom[index].Extra...)
	}

	list := strings.Split(baddrs, ",")
	for _, item := range list {
		addr := strings.Split(item, ":")
		port, _ := strconv.Atoi(addr[1])
		ic.AddAppRemote(addr[0], port)
	}
	this.gs.Hook("db_conn_num", func() int {
		mconnects := make(map[string]int)
		cconnects := make(map[string]int)
		var result int
		Dao()._modelLocator.Broadcast("Connections", mconnects)
		Dao()._cacheLocator.Broadcast("Connections", cconnects)
		for _, item := range mconnects {
			result += item
		}

		for _, item := range cconnects {
			result += item
		}
		return result
	})
	this.gs.Hook("db_queue_num", func() int {
		return this.queueLen()
	})
}

func (this *DAO) connDB(comName string) (result bool) {
	_, result = this.connDBMap.SetOrStore(comName, true)
	return
}

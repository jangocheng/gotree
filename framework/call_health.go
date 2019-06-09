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
	"fmt"
	"net"
	netrpc "net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/8treenet/gotree/helper"
)

type HealthClient struct {
	ComEntity
	gateway   bool
	AppRemote []*struct {
		Addr      string
		Port      int
		StartPort int
	}
	coms         []ComPanel
	StartTime    string
	LastInfoTime int64
	dbCountFunc  func() int
	closeMsg     chan bool
}

func (this *HealthClient) Gotree() *HealthClient {
	this.ComEntity.Gotree(this)
	this.gateway = false
	this.AppRemote = make([]*struct {
		Addr      string
		Port      int
		StartPort int
	}, 0)
	this.coms = make([]ComPanel, 0, 32)
	this.AddEvent("ListenSuccess", this.hookRpcBind)
	this.StartTime = time.Now().Format("2006-01-02 15:04:05")
	this.LastInfoTime = 0
	this.closeMsg = make(chan bool, 1)
	return this
}

//AddRemoteSerAddr 加入远程地址
func (this *HealthClient) AddRemoteAddr(RemoteAddr string) {
	var innerMaster *CallRoute
	this.GetComObject(&innerMaster)
	innerMaster.ping = true
	this.gateway = true

	innerMaster.addAddr(RemoteAddr)
	return
}

func (this *HealthClient) AddAppRemote(ip string, port int) {
	var item struct {
		Addr      string
		Port      int
		StartPort int
	}
	item.Port = port
	item.StartPort = port
	item.Addr = ip
	this.AppRemote = append(this.AppRemote, &item)
}

func (this *HealthClient) AddDaoByNode(name string, id int, args ...interface{}) {
	dao := ComPanel{
		Name:  name,
		Port:  "",
		ID:    id,
		Extra: args,
	}
	this.coms = append(this.coms, dao)
	return
}

func (this *HealthClient) hookRpcBind(port ...interface{}) {
	localPort := fmt.Sprint(port[0])
	tempdaos := []ComPanel{}
	for _, v := range this.coms {
		com := ComPanel{
			ID:    v.ID,
			Extra: v.Extra,

			Port: localPort,
			Name: v.Name,
		}
		tempdaos = append(tempdaos, com)
	}
	this.coms = tempdaos
	go this.Start()
	return
}

func (this *HealthClient) Start() {
	ti := 2
	var _route *CallRoute
	clientMap := make(map[string]*netrpc.Client)
	this.GetComObject(&_route)

	if !this.gateway {
		ti = 4
	} else {
		list := _route.addrList()
		for _, addr := range list {
			addrSplit := strings.Split(addr, ":")
			i, err := strconv.Atoi(addrSplit[1])
			if err != nil {
				panic(err.Error())
			}
			this.AddAppRemote(addrSplit[0], i)
		}
	}
	for {
		if this.gateway {
			this.gatewayHeartbeat(clientMap, _route)
		} else {
			this.daoHeartbeat(clientMap)
		}

		for index := 0; index < ti; index++ {
			select {
			case _ = <-this.closeMsg:
				runtime.Goexit()
			default:
			}
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func (this *HealthClient) Close() {
	this.closeMsg <- true
}

func (this *HealthClient) daoHeartbeat(clientMap map[string]*netrpc.Client) {
	var (
		in struct {
			ComList []ComPanel
		}
		reply int
	)
	in.ComList = this.coms
	for _, v := range this.AppRemote {
		rpcaddr := fmt.Sprintf("%s:%d", v.Addr, v.Port)
		client, ok := clientMap[rpcaddr]
		if !ok {
			var cerr error
			client, cerr := jsonRpc(rpcaddr, 600)
			if cerr == nil {
				clientMap[rpcaddr] = client
			}
		}

		if client != nil {
			if client.Call("Health.HandShake", in, &reply) == nil {
				continue
			}
			client.Close()
			delete(clientMap, rpcaddr)
		}

		for index := 0; index < 9; index++ {
			client, err := jsonRpc(fmt.Sprintf("%s:%d", v.Addr, v.StartPort+index), 600)
			if err == nil {
				if client.Call("Health.HandShake", in, &reply) == nil {
					v.Port = v.StartPort + index
					clientMap[rpcaddr] = client
					break
				}
				client.Close()
			}
		}
	}
}
func (this *HealthClient) gatewayHeartbeat(clientMap map[string]*netrpc.Client, _route *CallRoute) {
	var reply int
	for _, v := range this.AppRemote {
		rpcaddr := fmt.Sprintf("%s:%d", v.Addr, v.Port)
		client, ok := clientMap[rpcaddr]
		if !ok {
			var cerr error
			client, cerr = jsonRpc(rpcaddr, 600)
			if cerr == nil {
				clientMap[rpcaddr] = client
			}
		}
		if client != nil {
			if client.Call("Health.Ping", 100, &reply) == nil {
				_route.addAddr(fmt.Sprintf("%s:%d", v.Addr, v.Port))
				continue
			}
			client.Close()
			delete(clientMap, rpcaddr)
		}
		_route.removeAddr(fmt.Sprintf("%s:%d", v.Addr, v.Port))

		for index := 0; index < 9; index++ {
			rpcaddr = fmt.Sprintf("%s:%d", v.Addr, v.StartPort+index)
			client, err := jsonRpc(fmt.Sprintf("%s:%d", v.Addr, v.StartPort+index), 600)
			if err == nil {
				if client.Call("Health.Ping", 100, &reply) == nil {
					v.Port = v.StartPort + index
					_route.addAddr(fmt.Sprintf("%s:%d", v.Addr, v.StartPort+index))
					clientMap[rpcaddr] = client
					break
				}
				client.Close()
			}
		}
	}
}

func jsonRpc(addr string, timeout ...int) (client *netrpc.Client, e error) {
	t := 1200
	if len(timeout) > 0 && timeout[0] > 500 {
		t = timeout[0]
	}
	tcpclient, err := net.DialTimeout("tcp", addr, time.Duration(t)*time.Millisecond)
	if err != nil {
		e = err
		return
	}
	client = jsonrpc.NewClient(tcpclient)
	return
}

func init() {
	startTime = time.Now().Format("2006-01-02 15:04:05")
}

type HealthController struct {
	CallController
}

func (this *HealthController) Gotree() *HealthController {
	this.CallController.Gotree(this)
	return this
}

func (this *HealthController) HandShake(coms struct {
	ComList []ComPanel
}, ret *int) error {
	addr := this.RemoteAddr()
	ip := strings.Split(addr, ":")
	unix := time.Now().Unix()

	for _, dni := range coms.ComList {
		com := Panel{
			comName: dni.Name,
			ip:      ip[0],
			id:      dni.ID,
			Extra:   dni.Extra,
			update:  unix,
			port:    dni.Port,
		}
		//通知节点接入
		this.Event("newCom", com)
	}

	*ret = 666
	return nil
}

//api 服务器发来的握手
func (this *HealthController) Ping(arg interface{}, ret *int) error {
	*ret = 666
	return nil
}

//ProcessId 获取进程id
func (this *HealthController) ProcessId(arg interface{}, ret *int) error {
	*ret = os.Getpid()
	return nil
}

func (this *HealthController) ComStatus(arg interface{}, ret *string) error {
	var list []*Panel
	this.Event("ComStatus", &list)
	for _, item := range list {
		if *ret != "" {
			*ret += ";"
		}
		*ret += "com:" + item.comName + " id:" + fmt.Sprint(item.id) + " ip:" + item.ip + " port:" + item.port + " extra:" + fmt.Sprint(item.Extra)
	}
	return nil
}

func (this *HealthController) DaoServerStatus(arg interface{}, ret *string) error {
	var list []*Panel
	if *ret != "" {
		*ret += ";"
	}
	this.Event("ComStatus", &list)
	addrs := make(map[string]bool)
	for _, item := range list {
		addrs[item.ip+":"+item.port] = true
	}

	for addr, _ := range addrs {
		client, err := jsonRpc(addr)
		if err != nil {
			continue
		}
		var result string
		if client.Call("Health.DaoInfo", "666", &result) != nil {
			client.Close()
			continue
		}
		client.Close()
		if *ret != "" {
			*ret += ";"
		}

		list := strings.Split(result, ",")
		if len(list) < 8 {
			continue
		}

		*ret += fmt.Sprintf("Dao Service address:%s, Memory usage ratio:%smb, GCCPU usage ratio:%s, GC times:%s, Request url:%s, Connect to the database:%s, Log queue:%s, Async queue:%s, Start Time:%s", addr, list[0], list[1], list[2], list[3], list[5], list[6], list[7], list[4])
	}
	return nil
}

func (this *HealthController) DaoInfo(arg interface{}, ret *string) error {
	*ret = strings.Join(sysInfo(), ",")
	return nil
}

func (this *HealthController) AppInfo(arg interface{}, ret *string) error {
	*ret = ""
	list := sysInfo()
	if len(list) < 7 {
		return nil
	}

	*ret = fmt.Sprintf("Memory usage ratio:%smb, GCCPU usage ratio:%s, GC times:%s, Request url:%d, Timing:%d, Async queue:%d, Log queue:%s, Start Time:%s", list[0], list[1], list[2], _runtimeStack.Calls(), CurrentTimeNum(), App.AsyncNum(), list[6], startTime)
	return nil
}

// ComQpsBeginTime
func (this *HealthController) ComQpsBeginTime(arg interface{}, ret *string) error {
	var t int64
	this.Event("ComQpsBeginTime", &t)
	*ret = time.Unix(t, 0).Format("15:04:05")
	return nil
}

// ComQps
func (this *HealthController) ComQps(arg interface{}, ret *string) error {
	var list []struct {
		ServiceMethod string
		Count         int64
		AvgMs         int64
		MaxMs         int64
		MinMs         int64
	}
	this.Event("ComQps", &list)
	helper.SliceSort(&list, "AvgMs")
	*ret += fmt.Sprintf("%46s %12s %10s %10s %10s", "Call", "Count", "MaxMs", "MinMs", "AvgMs")
	for _, item := range list {
		if *ret != "" {
			*ret += "??"
		}

		callcount := fmt.Sprint(item.Count)
		if item.Count > 1000 {
			callcount = fmt.Sprintf("%.2fk", float32(item.Count)/1000.0)
		}

		*ret += fmt.Sprintf("%46s \x1b[0;31m%12s\x1b[0m \x1b[0;31m%10d\x1b[0m \x1b[0;31m%10d\x1b[0m \x1b[0;31m%10d\x1b[0m", item.ServiceMethod, callcount, item.MaxMs, item.MinMs, item.AvgMs)
	}
	return nil
}

type appInter interface {
	AsyncNum() int
}

var startTime string
var App appInter

func sysInfo() (result []string) {
	m := runtime.MemStats{}
	runtime.ReadMemStats(&m)
	result = append(result, fmt.Sprint(m.Alloc/1024/1024))        //内存占用mb
	result = append(result, fmt.Sprintf("%.3f", m.GCCPUFraction)) //gc cpu占用
	result = append(result, fmt.Sprint(m.NumGC))                  //gc 次数
	result = append(result, fmt.Sprint(_runtimeStack.Calls()))
	result = append(result, startTime) //系统启动时间
	type callnum func() int
	_callnum := _runtimeStack.hook("db_conn_num")
	if _callnum != nil {
		fun := _callnum.(callnum)
		result = append(result, fmt.Sprint(fun())) //数据库总连接数
	} else {
		result = append(result, "") //数据库总连接数
	}
	result = append(result, fmt.Sprint(Log().ProgressLen())) //日志待处理

	_callnum = _runtimeStack.hook("db_queue_num")
	if _callnum != nil {
		fun := _callnum.(callnum)
		result = append(result, fmt.Sprint(fun())) //队列待处理
	} else {
		result = append(result, "0") //队列待处理
	}
	return
}

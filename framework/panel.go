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
	"hash/crc32"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	
)

type panels struct {
	lock     sync.Mutex
	hotNode  *ConHash
	hashNode *ConHash
	pmap     map[string]*Panel
	pid      int
}

func (this *panels) nodeManage() *panels {
	this.pmap = make(map[string]*Panel)
	this.pid = 1000
	this.hotNode = new(ConHash).ConHash()
	this.hashNode = new(ConHash).ConHash()
	rand.Seed(time.Now().Unix())
	return this
}

//RandomAddr 随机节点地址
func (this *panels) RandomAddr() string {
	defer this.lock.Unlock()
	this.lock.Lock()

	list := this.all()
	if len(list) == 0 {
		return ""
	}

	index := rand.Intn(len(list))
	return list[index].RpcAddr()
}

//BalanceAddr 均衡节点地址
func (this *panels) BalanceAddr() string {
	defer this.lock.Unlock()
	this.lock.Lock()

	list := this.all()
	if len(list) == 0 {
		return ""
	}

	seq := this.pid
	if seq > 9999999999 {
		this.pid = 1000
	} else {
		this.pid++
	}
	return list[seq%len(list)].RpcAddr()
}

//HostHashRpcAddr  热一致性哈希节点地址
func (this *panels) HostHashRpcAddr(value interface{}) string {
	defer this.lock.Unlock()
	this.lock.Lock()
	searchValue := crc32.ChecksumIEEE([]byte(fmt.Sprint(value)))
	v := this.hotNode.Search(searchValue)
	if v == nil {
		return ""
	}
	nodeid := v.(int)

	for _, node := range this.all() {
		if node.id == nodeid {
			return node.RpcAddr()
		}
	}
	return ""
}

//HotHashRpcId 热一致性哈希节点id
func (this *panels) HotHashRpcId(value interface{}) int {
	defer this.lock.Unlock()
	this.lock.Lock()
	searchValue := crc32.ChecksumIEEE([]byte(fmt.Sprint(value)))
	v := this.hotNode.Search(searchValue)
	if v == nil {
		return 0
	}
	nodeid := v.(int)
	return nodeid
}

//HashRpcAddr 一致性哈希节点地址
func (this *panels) HashRpcAddr(value interface{}) string {
	defer this.lock.Unlock()
	this.lock.Lock()
	searchValue := crc32.ChecksumIEEE([]byte(fmt.Sprint(value)))
	v := this.hashNode.Search(searchValue)
	if v == nil {
		return ""
	}
	nodeid := v.(int)

	for _, node := range this.all() {
		if node.id == nodeid {
			return node.RpcAddr()
		}
	}
	return ""
}

//HashRpcId 一致性哈希节点id
func (this *panels) HashRpcId(value interface{}) int {
	defer this.lock.Unlock()
	this.lock.Lock()
	searchValue := crc32.ChecksumIEEE([]byte(fmt.Sprint(value)))
	v := this.hashNode.Search(searchValue)
	if v == nil {
		return 0
	}
	nodeid := v.(int)
	return nodeid
}

//SlaveAddr 从节点地址
func (this *panels) SlaveAddr() string {
	defer this.lock.Unlock()
	this.lock.Lock()

	list := this.all()
	if len(list) == 0 {
		return ""
	}

	if len(list) == 1 {
		return list[0].RpcAddr()
	}

	slvae := []*Panel{}
	for _, ni := range list {
		if ni.id != 1 {
			//排除主节点
			slvae = append(slvae, ni)
		}
	}

	index := rand.Intn(len(slvae))
	return slvae[index].RpcAddr()
}

//MasterAddr 返回主节点
func (this *panels) MasterAddr() string {
	defer this.lock.Unlock()
	this.lock.Lock()

	al := this.all()
	if len(al) == 0 {
		return ""
	}

	for _, node := range al {
		if node.id == 1 {
			return node.RpcAddr()
		}
	}
	return ""
}

func (this *panels) remove(node *Panel) {
	defer this.lock.Unlock()
	this.lock.Lock()

	locNode, ok := this.pmap[node.ip+"_"+node.port]
	if !ok {
		return
	}

	//删除一致性虚拟节点
	this.hotNode.Remove(locNode.id)
	delete(this.pmap, node.ip+"_"+node.port)
}

func (this *panels) addNode(node *Panel) {
	addr := node.ip + "_" + node.port
	delList := this.all()
	for _, item := range delList {
		if item.id == node.id && addr != item.ip+"_"+item.port {
			this.remove(item)
		}
	}
	defer this.lock.Unlock()
	this.lock.Lock()
	locNode, ok := this.pmap[addr]
	if ok {
		locNode.update = node.update
		return
	}
	this.pmap[addr] = node

	this.hotNode.Add(node.id)
	maxkv := ""
	for _, extra := range node.Extra {
		str := extra.(string)
		if !strings.Contains(str, "MaxID") {
			continue
		}
		maxkv = str
	}
	if maxkv == "" {
		return
	}

	if kv := strings.Split(maxkv, ":"); len(kv) == 2 {
		maxid, err := strconv.Atoi(kv[1])
		if err != nil {
			return
		}
		for index := 1; index <= maxid; index++ {
			if this.hashNode.Check(index) {
				continue
			}
			this.hashNode.Add(index)
		}
	}
}

func (this *panels) all() (list []*Panel) {
	for _, m := range this.pmap {
		list = append(list, m)
	}
	return
}

func (this *panels) AllCom() (list []*Panel) {
	defer this.lock.Unlock()
	this.lock.Lock()
	return this.all()
}

//Len
func (this *panels) Len() int {
	defer this.lock.Unlock()
	this.lock.Lock()
	return len(this.pmap)
}

type Panel struct {
	update  int64
	comName string
	id      int
	ip      string
	port    string
	Extra   []interface{}
}

//RpcAddr 节点地址
func (this *Panel) RpcAddr() string {
	return this.ip + ":" + this.port
}

type ComPanel struct {
	Name  string
	Port  string
	ID    int
	Extra []interface{} `opt:"null"`
}

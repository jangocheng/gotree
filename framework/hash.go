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
	"reflect"
	"sort"
	"strconv"
)

func asString(src interface{}) string {
	switch v := src.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	rv := reflect.ValueOf(src)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(rv.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(rv.Uint(), 10)
	case reflect.Float64:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 64)
	case reflect.Float32:
		return strconv.FormatFloat(rv.Float(), 'g', -1, 32)
	}
	return fmt.Sprintf("%v", src)
}

type ConHash struct {
	dums []dum
}

func (this *ConHash) ConHash() *ConHash {
	this.dums = make([]dum, 0, 2048)
	return this
}

func (this *ConHash) Add(addr interface{}) {
	this.Remove(addr)
	pa := new(physicalAddr)
	pa.addr = addr

	dlist := pa.create()
	this.dums = append(this.dums, dlist...)
	sort.Sort(dums(this.dums))
}

func (this *ConHash) Remove(addr interface{}) {
	newList := make([]dum, 0, 2048) //全部节点
	for index := 0; index < len(this.dums); index++ {
		if this.dums[index].point.addr == addr {
			this.dums[index].point = nil
			continue
		}

		newList = append(newList, this.dums[index])
	}
	this.dums = newList
}

//CheckAddr检查该物理地址是否存在
func (this *ConHash) Check(addr interface{}) bool {
	for index := 0; index < len(this.dums); index++ {
		if this.dums[index].point.addr == addr {
			return true
		}
	}
	return false
}

//Search查找节点位置
func (this *ConHash) Search(value uint32) interface{} {
	if len(this.dums) == 0 {
		return nil
	}
	head := this.dums[0]
	tail := this.dums[len(this.dums)-1]
	if value < head.v {
		return head.point.addr
	}

	if value > tail.v {
		return head.point.addr
	}

	return this.half(value, 0, len(this.dums)-1)
}

func (this *ConHash) half(value uint32, begin int, end int) interface{} {

	if begin == end || (end-begin) == 1 {
		if this.dums[end].v == value {
			return this.dums[end].point.addr
		}
		return this.dums[begin].point.addr
	}

	halfIndex := begin + (end-begin)/2
	halfDummy := this.dums[halfIndex]
	if value > halfDummy.v {
		return this.half(value, halfIndex, end)
	}
	return this.half(value, begin, halfIndex)
}

//物理地址
type physicalAddr struct {
	addr interface{} //地址
}

type dum struct {
	point *physicalAddr
	v     uint32
}

type dums []dum

func (this dums) Len() int {
	return len(this)
}

func (this dums) Less(i, j int) bool {
	return this[i].v < this[j].v
}

func (this dums) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}

func (this *physicalAddr) create() []dum {
	result := make([]dum, 512)
	for index := 0; index < 512; index++ {
		result[index].point = this
		result[index].v = crc32.ChecksumIEEE([]byte(fmt.Sprint(this.addr) + fmt.Sprint(index+1)))
	}
	return result
}

type Hash struct {
	value interface{}
	mod   uint32
}

func (this *Hash) Gotree(mod uint32) *Hash {
	this.mod = mod
	return this
}

func (this *Hash) HashNodeSum(v interface{}, modarg ...int) int {
	value := fmt.Sprint(v)
	mod := this.mod
	if len(modarg) > 0 {
		mod = uint32(modarg[0])
	}
	result := crc32.ChecksumIEEE([]byte(value)) % mod
	return int(result) + 1
}

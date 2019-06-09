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

package helper

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strconv"
)

//InSlice 是否在数组内
func InSlice(array interface{}, item interface{}) bool {
	values := reflect.ValueOf(array)
	if values.Kind() != reflect.Slice {
		return false
	}

	size := values.Len()
	list := make([]interface{}, size)
	slice := values.Slice(0, size)
	for index := 0; index < size; index++ {
		list[index] = slice.Index(index).Interface()
	}

	for index := 0; index < len(list); index++ {
		if list[index] == item {
			return true
		}
	}
	return false
}

//NewSlice 创建数组
func NewSlice(dsc interface{}, len int) error {
	dstv := reflect.ValueOf(dsc)
	if dstv.Elem().Kind() != reflect.Slice {
		return errors.New("dsc error")
	}

	result := reflect.MakeSlice(reflect.TypeOf(dsc).Elem(), len, len)
	dstv.Elem().Set(result)
	return nil
}

//SliceDelete 删除数组指定下标元素
func SliceDelete(arr interface{}, indexArr ...int) error {
	dstv := reflect.ValueOf(arr)
	if dstv.Elem().Kind() != reflect.Slice {
		return errors.New("dsc error")
	}
	result := reflect.MakeSlice(reflect.TypeOf(arr).Elem(), 0, dstv.Elem().Len()-len(indexArr))
	for index := 0; index < dstv.Elem().Len(); index++ {
		if InSlice(indexArr, index) {
			continue
		}
		result = reflect.Append(result, dstv.Elem().Index(index))
	}

	dstv.Elem().Set(result)
	return nil
}

type refectItem []struct {
	data reflect.Value
	x    int
}

func (this refectItem) Len() int {
	return len(this)
}
func (this refectItem) Swap(i, j int) {
	this[i], this[j] = this[j], this[i]
}
func (this refectItem) Less(i, j int) bool {
	return this[j].x < this[i].x
}

//SliceSort 降序
func SliceSort(array interface{}, field string, reverse ...bool) {
	srcV := reflect.ValueOf(array)
	if srcV.Elem().Kind() != reflect.Slice {
		panic("SliceSort is not slice data or empty")
	}

	if srcV.Elem().Len() == 0 {
		return
	}

	sortArray := make(refectItem, srcV.Elem().Len())
	for index := 0; index < srcV.Elem().Len(); index++ {
		sortFiled := srcV.Elem().Index(index).FieldByName(field)
		if !sortFiled.IsValid() {
			panic("SliceSort Filed:" + field + " not exist")
		}
		numFiled, err := strconv.Atoi(fmt.Sprint(sortFiled.Interface()))
		if err != nil {
			panic("SliceSort Filed:" + field + " conversion int type failed")
		}
		sortArray[index].x = numFiled
		sortArray[index].data = srcV.Elem().Index(index)
	}
	if len(reverse) > 0 {
		sort.Sort(sort.Reverse(sortArray))
	} else {
		sort.Sort(sortArray)
	}

	result := reflect.MakeSlice(reflect.TypeOf(array).Elem(), 0, srcV.Elem().Len())
	for index := 0; index < len(sortArray); index++ {
		result = reflect.Append(result, sortArray[index].data)
	}
	srcV.Elem().Set(result)
	return
}

//SliceSortReverse 升序
func SliceSortReverse(array interface{}, field string) {
	SliceSort(array, field, true)
}

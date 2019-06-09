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
	"runtime"

	"github.com/8treenet/gotree/framework"
	"github.com/8treenet/gotree/framework/model"

	"reflect"
	"strings"
	"time"
)

type ComModel struct {
	framework.GotreeBase
	open    bool
	comName string
}

func (this *ComModel) Gotree(child interface{}) *ComModel {
	this.GotreeBase.Gotree(this)
	this.AddChild(this, child)
	this.open = false
	this.comName = ""

	this.AddEvent("DaoTelnet", this.daoTelnet)
	this.AddEvent("ModelOn", this.modelOn)
	return this
}

type Conn interface {
	Raw(query string, args ...interface{}) model.RawSeter
}

// Conn
func (this *ComModel) Conn() Conn {
	if !this.open {
		framework.Exit("ComModel-Conn open model error: Not opened com:" + this.comName)
	}
	if this.comName == "" {
		framework.Exit("ComModel-Conn This is an unregistered com")
		return nil
	}
	o := model.New(this.comName)
	if Dao().mysqlProfiler && o.Driver().Type() == model.DRMySQL {
		o.RawCallBack(func(sql string, args []interface{}) {
			this.explain(sql, args...)
		})
	}
	return o
}

//Testing
func (this *ComModel) Testing() {
	Dao()
	mode := Config().String("sys::Mode")
	if mode == "prod" {
		framework.Exit("Unit tests cannot be used in a production environment")
	}
	Dao().gs.Runtime().Set("gseq", "ModelUnit")
	this.DaoInit()
	if Config().DefaultString("com_on::"+this.comName, "") == "" {
		framework.Exit("COM not found, please check if the com.conf file is turned on. The " + this.comName)
	}
	this.ormOn()
}

// daoTelnet
func (this *ComModel) daoTelnet(args ...interface{}) {
	for _, arg := range args {
		dao := arg.(comNode)
		if dao.Name == this.comName {
			this.ormOn()
			return
		}
	}
}

// modelOn
func (this *ComModel) modelOn(arg ...interface{}) {
	comName := arg[0].(string)
	if comName == this.comName {
		this.ormOn()
	}
}

// ormOn
func (this *ComModel) ormOn() {
	this.open = true
	if !Dao().connDB(this.comName + "model") {
		return
	}

	drivers := []string{"mysql", "sqlite", "oracle", "postgres", "tidb"}
	findSucess := false
	for index := 0; index < len(drivers); index++ {
		//处理连接
		driver := drivers[index]
		dbconfig := Config().String(driver + "::" + this.comName)
		maxIdleConns := Config().DefaultInt(driver+"::"+this.comName+"MaxIdleConns", runtime.NumCPU()*2)
		maxOpenConns := Config().DefaultInt(driver+"::"+this.comName+"MaxOpenConns", runtime.NumCPU()*2)
		if dbconfig == "" {
			continue
		}
		findSucess = true
		_, err := model.GetDB(this.comName)
		if err == nil {
			//已注册
			return
		}
		if maxIdleConns > maxOpenConns {
			framework.Exit("ComModel Failure to connect " + this.comName + " db, MaxIdleConns or MaxOpenConns are invalid args")
		}
		Log().Notice("ComModel Connect com " + this.comName + " database, MaxIdleConns:" + fmt.Sprint(maxIdleConns) + ", MaxOpenConns:" + fmt.Sprint(maxOpenConns) + ", config:" + dbconfig)
		err = model.RegisterDataBase(this.comName, driver, dbconfig, maxIdleConns, maxOpenConns)

		if err != nil {
			framework.Exit("ComModel-RegisterDataBase Connect " + this.comName + " error:," + err.Error())
		}
	}
	if !findSucess {
		framework.Exit("ComModel " + this.comName + ":No database configuration information exists")
	}
}

// FormatIn
func (this *ComModel) FormatIn(args ...interface{}) (list []interface{}) {
	for _, item := range args {
		slice := reflect.ValueOf(item)
		if slice.Kind() != reflect.Slice {
			list = append(list, item)
			continue
		}
		for i := 0; i < slice.Len(); i++ {
			list = append(list, slice.Index(i).Interface())
		}

	}
	return
}

// FormatMark
func (this *ComModel) FormatMark(arg interface{}) string {
	slice := reflect.ValueOf(arg)
	if slice.Kind() != reflect.Slice {
		return ""
	}

	result := []string{}
	c := slice.Len()
	for i := 0; i < c; i++ {
		result = append(result, "?")
	}
	return strings.Join(result, ",")
}

// explain
func (this *ComModel) explain(explainSql string, args ...interface{}) {
	gseq := Dao().gs.Runtime().Get("gseq")
	if gseq == nil {
		return
	}
	if strings.Contains(explainSql, "EXPLAIN") {
		return
	}
	sourceSql := fmt.Sprintf(strings.Replace(explainSql, "?", "%v", -1), args...)
	sql := strings.ToLower(sourceSql)
	if strings.Contains(sql, "delete") || strings.Contains(sql, "update") || strings.Contains(sql, "insert") || strings.Contains(sql, "count") || strings.Contains(sql, "sum") || strings.Contains(sql, "max") {
		Log().Notice("sql explain The", sourceSql)
		return
	}
	var explain []struct {
		Table string
		Type  string
	}
	explainLog := ""
	o := model.New(this.comName)
	_, e := o.Raw("EXPLAIN "+explainSql, args...).QueryRows(&explain)
	tables := []string{}
	warn := false
	if e == nil {
		for index := 0; index < len(explain); index++ {
			if index > 0 {
				explainLog += " "
			}
			etype := strings.ToLower(explain[index].Type)
			if etype == "all" || etype == "index" {
				warn = true
			}
			explainLog += " tableName " + explain[index].Table + " Sql Explain Type " + explain[index].Type
			tables = append(tables, explain[index].Table)
		}
	}
	if explainLog != "" {
		explainLog = "explain :(" + explainLog + ")"
	}
	if warn {
		Log().Warning("sql explain ", explainLog, "source :("+sourceSql+")")
	} else {
		Log().Notice("sql explain ", explainLog, "source :("+sourceSql+")")
	}

	str, ok := gseq.(string)
	if !ok {
		return
	}

	for _, item := range tables {
		table := item
		if explainSync(str + "_" + table) {
			continue
		}
		go func() {
			time.Sleep(3 * time.Second)
			var tableCount int
			exerr := Dao().explainMap.Get(str+"_"+table, &tableCount)
			if exerr != nil {
				return
			}
			Dao().explainMap.Remove(str + "_" + table)
			if tableCount > 1 {
				Log().Warning(str + "reads the table'" + table + "' " + fmt.Sprint(tableCount) + " times in a bsep")
			}
		}()
	}
}

func explainSync(key string) (exist bool) {
	exist = false
	count := 0
	if Dao().explainMap.Get(key, &count) != nil {
		exist = true
	}
	count = count + 1
	Dao().explainMap.Set(key, count)
	return
}

func (this *ComModel) Connections(m map[string]int) {
	if !this.open {
		return
	}
	db, err := model.GetDB(this.comName)
	if err != nil {
		return
	}
	m[this.comName] = db.Stats().OpenConnections
}

func (this *ComModel) DaoInit() {
	if this.comName == "" {
		this.comName = this.TopChild().(comName).Com()
	}
}

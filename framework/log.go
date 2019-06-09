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
	"os"
	"runtime/debug"
	"strconv"
	"sync"
	"time"
)

const (
	_LOG_ERROR            = iota
	_LOG_WARN             = iota
	_LOG_NOTICE           = iota
	__LOG_ERROR_DIR_NAME  = "error"  //错误日志目录名
	__LOG_WARN_DIR_NAME   = "warn"   //警告日志目录名
	__LOG_NOTICE_DIR_NAME = "notice" //普通日志目录名
)

type log struct {
	errorDir string
	warnDir  string
	infoDir  string

	errFile  *os.File
	warnFile *os.File
	infoFile *os.File

	errFileName  string
	warnFileName string
	infoFileName string

	debug    bool
	errlock  *sync.Mutex
	imsg     chan string
	wmsg     chan string
	closeMsg chan bool

	stream chan *logItem
}

type logItem struct {
	category int
	data     []interface{}
}

type gseqInter interface {
	Get(key string) interface{}
}

func (this *log) Init(dir string) {
	logPath := dir

	mode := GetConfig().String("sys::Mode")
	if mode != "prod" && Testing() {
		this.debug = true
		return
	}
	if mode != "prod" {
		this.debug = true
	}
	if !FileExists(logPath) {
		err := os.Mkdir(logPath, os.ModePerm)
		if err != nil {
			panic(err)
		}
	}
	this.errorDir = logPath + "/" + __LOG_ERROR_DIR_NAME
	if !FileExists(this.errorDir) {
		os.Mkdir(this.errorDir, os.ModePerm)
	}

	this.warnDir = logPath + "/" + __LOG_WARN_DIR_NAME
	if !FileExists(this.warnDir) {
		os.Mkdir(this.warnDir, os.ModePerm)
	}

	this.infoDir = logPath + "/" + __LOG_NOTICE_DIR_NAME
	if !FileExists(this.infoDir) {
		os.Mkdir(this.infoDir, os.ModePerm)
	}

	this.errlock = new(sync.Mutex)
	this.wmsg = make(chan string, GetConfig().DefaultInt("sys::LogWarnQueueLen", 512))
	this.imsg = make(chan string, GetConfig().DefaultInt("sys::LogInfoQueueLen", 2048))
	this.closeMsg = make(chan bool, 2)
	go this.warnRun()
	go this.infoRun()
}

func (this *log) warnRun() {
	for {
		msg := <-this.wmsg
		if msg == "" {
			this.closeMsg <- true
			break
		}
		this.write(_LOG_WARN, msg)
	}
}

//Run
func (this *log) infoRun() {
	for {
		msg := <-this.imsg
		if msg == "" {
			this.closeMsg <- true
			break
		}
		this.write(_LOG_NOTICE, msg)
	}
}

func (this *log) Error(str ...interface{}) {
	stack := string(debug.Stack())
	str = append(str, "\n"+stack)
	text := []interface{}{}
	if no := this.GseqNo(); no != "" {
		text = append(text, "gseq:"+no)
	}
	text = append(text, str...)
	datetime := this.nowDateTime()
	if this.debug {
		if Testing() {
			fmt.Println(datetime + " " + fmt.Sprint(text))
			return
		}
		fmt.Println(red(datetime + " " + fmt.Sprint(text)))
	}
	defer this.errlock.Unlock()
	this.errlock.Lock()
	this.write(_LOG_ERROR, datetime+" "+fmt.Sprint(text))
}

func (this *log) Warning(str ...interface{}) {
	text := []interface{}{}
	if no := this.GseqNo(); no != "" {
		text = append(text, "gseq:"+no)
	}
	datetime := this.nowDateTime()
	text = append(text, str...)
	if this.debug {
		if Testing() {
			fmt.Println(datetime + " " + fmt.Sprint(text))
			return
		}
		fmt.Println(red(datetime + " " + fmt.Sprint(text)))
	}

	this.wmsg <- datetime + " " + fmt.Sprint(text)
}

func (this *log) Notice(str ...interface{}) {
	text := []interface{}{}
	if no := this.GseqNo(); no != "" {
		text = append(text, "gseq:"+no)
	}
	datetime := this.nowDateTime()
	text = append(text, str...)
	if this.debug {
		if Testing() {
			fmt.Println(datetime + " " + fmt.Sprint(text))
			return
		}
		fmt.Println(green(datetime + " " + fmt.Sprint(text)))

	}
	this.imsg <- datetime + " " + fmt.Sprint(text)
}

func (this *log) Debug(str ...interface{}) {
	if !this.debug {
		return
	}
	this.Notice(str...)
}

func (this *log) write(logType int, str string) {
	wt := this.getLogger(logType)
	if wt == nil {
		return
	}

	buf := []byte(str)
	wt.Write(append(buf, '\n'))
}

func (this *log) Close() {
	this.imsg <- ""
	this.wmsg <- ""

	closeLen := 0
	for index := 0; index < 20; index++ {
		if closeLen == 2 {
			break
		}
		select {
		case _ = <-this.closeMsg:
			closeLen += 1
		default:
			//最多等待2秒
			time.Sleep(time.Millisecond * 100)
			continue
		}
	}

	this.errFile.Close()
	this.infoFile.Close()
	this.warnFile.Close()
}

//获取logger
func (this *log) getLogger(logType int) *os.File {
	var result *os.File

	fileName := this.getFileName(logType)
	switch logType {
	case _LOG_ERROR:
		if this.errFile == nil {
			this.errFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
			this.errFileName = fileName
		} else {
			if this.errFileName != fileName {
				this.errFile.Close()
				this.errFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
				this.errFileName = fileName
			}
		}
		result = this.errFile
	case _LOG_WARN:
		if this.warnFile == nil {
			this.warnFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
			this.warnFileName = fileName
		} else {
			if this.warnFileName != fileName {
				this.warnFile.Close()
				this.warnFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
				this.warnFileName = fileName
			}
		}
		result = this.warnFile
	default:
		if this.infoFile == nil {
			this.infoFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
			this.infoFileName = fileName
		} else {
			if this.infoFileName != fileName {
				this.infoFile.Close()
				this.infoFile, _ = os.OpenFile(fileName, os.O_APPEND|os.O_RDWR, 0660)
				this.infoFileName = fileName
			}
		}
		result = this.infoFile
	}

	return result
}

func (this *log) getFileName(logType int) string {
	dateDir := this.getDate(logType)
	hour, _, _ := time.Now().Clock()
	fileName := dateDir + "/" + strconv.Itoa(hour) + ".log"
	if FileExists(fileName) {
		return fileName
	}
	file, _ := os.Create(fileName)
	file.Close()
	return fileName
}

func (this *log) getDate(logType int) string {
	var path string
	switch logType {
	case _LOG_ERROR:
		path = this.errorDir
	case _LOG_WARN:
		path = this.warnDir
	default:
		path = this.infoDir
	}
	year, month, day := time.Now().Date()

	dateDir := path + "/" + fmt.Sprintf("%d-%d-%d", year, month, day)
	if !FileExists(dateDir) {
		os.Mkdir(dateDir, os.ModePerm)
	}
	return dateDir
}

func (this *log) nowDateTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func (this *log) GseqNo() string {
	if _runtimeStack == nil {
		return ""
	}

	gseq := _runtimeStack.Get("gseq")
	if gseq == nil {
		return ""
	}

	str, ok := gseq.(string)
	if !ok {
		return ""
	}
	return str
}

//ProgressLen
func (this *log) ProgressLen() int {
	return len(this.imsg) + len(this.wmsg)
}

var _log *log

func Log() *log {
	if _log == nil {
		_log = new(log)
	}
	return _log
}

const (
	textBlack = iota + 30
	textRed
	textGreen
)

func red(str string) string {
	return textColor(textRed, str)
}

func green(str string) string {
	return textColor(textGreen, str)
}

func textColor(color int, str string) string {
	switch color {
	case textRed:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", textRed, str)
	case textGreen:
		return fmt.Sprintf("\x1b[0;%dm%s\x1b[0m", textGreen, str)
	default:
		return str
	}
}

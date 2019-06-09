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
	"bytes"
	"fmt"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"syscall"
	"time"
)

var (
	pcode map[string]string
)

func init() {
	pcode = make(map[string]string)
	pcode["12f814f"] = "app"
	pcode["12ec006"] = "dao"
	pcode["app"] = "12f814f"
	pcode["dao"] = "12ec006"
}

func AppDaemon() {
	if e := LoadConfig(pcode[os.Args[1]]); e != nil {
		fmt.Println(e)
		return
	}
	dir := GetConfig().DefaultString("sys::LogDir", "log")
	Log().Init(dir)
	startSecs := GetConfig().DefaultInt64("sys::StartSecs", 3)

	for {
		var stderr bytes.Buffer
		cmd := exec.Command(os.Args[0])
		cmd.Stderr = &stderr

		startTime := time.Now().Unix()
		cmd.Start()
		err := cmd.Wait()
		if err == nil {
			os.Exit(0)
		}
		//启动时间和panic时间 如果在3秒内 停止服务
		if time.Now().Unix()-startTime <= startSecs {
			os.Exit(1)
		}
		time.Sleep(2 * time.Second)
	}
}

func AppStop(name, addr string, port int) {
	for index := 0; index < 10; index++ {
		client, err := jsonrpc.Dial("tcp", fmt.Sprintf("%s:%d", addr, port+index))
		if err != nil {
			continue
		}
		var pid int
		if client.Call("Health.ProcessId", 100, &pid) != nil {
			continue
		}
		process, err := os.FindProcess(pid)
		if err != nil {
			continue
		}
		fmt.Println("AppStop- Close Server"+name+" pid:", pid)
		process.Signal(syscall.SIGINT)
	}
}

func newPid(name, addr string, port int) (pid int) {
	for index := 0; index < 10; index++ {
		client, err := jsonrpc.Dial("tcp", fmt.Sprintf("%s:%d", addr, port+index))
		if err != nil {
			continue
		}
		if client.Call("Health.ProcessId", 100, &pid) != nil {
			continue
		}
		return
	}
	return
}

func AppRestart(name, addr string, port int) {
	if e := LoadConfig(name); e != nil {
		fmt.Println(e)
		return
	}
	startSecs := GetConfig().DefaultInt64("sys::StartSecs", 3)
	sleepMs := startSecs * 1000.0 / 10.0
	over := make(chan int, 1)
	go func() {
		pid := 0
		for i := 0; i != 10; i = i + 1 {
			if i > 4 {
				pid = newPid(name, addr, port)
				if pid != 0 {
					fmt.Fprintf(os.Stdout, "AppRestart-newPid Restart progress : %%%d\r", 100)
					break
				}
			}
			fmt.Fprintf(os.Stdout, "AppRestart-pid Restart progress : %%%d\r", i*10)
			time.Sleep(time.Millisecond * time.Duration(sleepMs))
		}
		if pid == 0 {
			fmt.Println("AppRestart-pid Startup failed, please check the error log")
			os.Exit(1)
		}
		over <- pid
	}()

	AppStop(name, addr, port)
	cmdStart := exec.Command("nohup", os.Args[0], "daemon", pcode[name], "&")
	err := cmdStart.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(-1)
	}
	newpid := <-over
	if err == nil {
		fmt.Printf("AppRestart-newpid-over Restart progress : %%%d\n", 100)
		fmt.Println("AppRestart-newpid-over StartUp"+name+" pid:", newpid)
	}
}

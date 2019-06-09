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

package model

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/8treenet/gotree/framework"
	"github.com/garyburd/redigo/redis"
)

var dbs map[string]*Cache

func init() {
	dbs = make(map[string]*Cache)
}

func Do(name, cmd string, to ...interface{}) (reply interface{}, err error) { //, to interface{}
	rc := GetClient(name)
	if rc == nil {
		return nil, errors.New("Redis-Do Invalid connection")
	}

	reply, err = rc.do(cmd, to...)
	return
}

// Cache is Redis cache adapter.
type Cache struct {
	p     *redis.Pool // redis connection pool
	mutex sync.Mutex
}

// AddDatabase 加入 枚举库 enum/db 和缓存
func AddDatabase(name string, c *Cache) {
	dbs[name] = c
}

// GetClient 通过枚举读取client
func GetClient(name string) *Cache {
	return dbs[name]
}

// GetConnects 获取连接数
func GetConnects(name string) int {
	c, ok := dbs[name]
	if !ok {
		return 0
	}
	return c.p.IdleCount() + c.p.ActiveCount()
}

// NewCache 创建新的 Redis Cache 对象。 connection - Redis服务器和端口号，例如: "127.0.0.1:6379", dbNum - Redis 数据库序号
func NewCache(connection string, password string, dbNum int, idle int, maxOpen int) (cache *Cache, err error) {
	cache = &Cache{}

	// initialize a new pool
	cache.p = &redis.Pool{
		MaxIdle:     idle,
		IdleTimeout: 600 * time.Second,
		MaxActive:   maxOpen,
		Wait:        true,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			if time.Since(t) < time.Minute {
				return nil
			}
			_, err := c.Do("PING")
			return err
		},
		Dial: func() (c redis.Conn, err error) {
			c, err = redis.DialTimeout("tcp", connection, 3*time.Second, 15*time.Second, 15*time.Second)
			if err != nil {
				return
			}

			defer func() {
				if err != nil {
					c.Close()
				}
			}()

			if password != "" {
				_, err = c.Do("AUTH", password)
				if err != nil {
					return
				}
			}

			if dbNum != 0 {
				_, err = c.Do("SELECT", dbNum)
				if err != nil {
					return
				}
			}

			return
		},
	}

	c := cache.p.Get()
	defer c.Close()
	err = c.Err()
	if err != nil {
		cache = nil
	}
	return
}

func (rc *Cache) getRc() redis.Conn {
	return rc.p.Get()
}

// actually do the redis cmds
func (rc *Cache) do(commandName string, args ...interface{}) (reply interface{}, err error) {
	if strings.ToLower(commandName) == "select" {
		framework.Log().Error("Forbid switching of database in operation")
	}

	c := rc.getRc()
	defer c.Close()
	return c.Do(commandName, args...)
}

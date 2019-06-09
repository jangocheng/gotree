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
	"os"
	"reflect"
	"strings"

	"github.com/huandu/go-tls/g"
)

var (
	_testing        bool = false
	ErrBreaker           = errors.New("Fuse")
	ErrTxHasBegan        = errors.New("<Ormer.Begin> transaction already begin")
	ErrTxDone            = errors.New("<Ormer.Commit/Rollback> transaction not begin")
	ErrMultiRows         = errors.New("<QuerySeter> return multi rows")
	ErrNoRows            = errors.New("<QuerySeter> no row found")
	ErrStmtClosed        = errors.New("<QuerySeter> stmt already closed")
	ErrArgs              = errors.New("<Ormer> args error may be empty")
	ErrNotImplement      = errors.New("have not implement")
)

func init() {
	wd, _ := os.Getwd()
	if strings.Contains(wd, "/unit") || strings.Contains(wd, "\\unit") {
		_testing = true
	}
}

type VoidValue struct {
	Void byte
}

func Testing() bool {
	return _testing
}

func RuntimePointer() interface{} {
	return g.G()
}

func Name(obj interface{}) string {
	return reflect.TypeOf(obj).Elem().Name()
}

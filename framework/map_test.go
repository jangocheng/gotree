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

import "testing"

func TestMap(t *testing.T) {
	dict := new(GotreeMap).Gotree()
	var info struct {
		Age  int
		Name string
	}
	info.Age = 25
	info.Name = "Tom"
	dict.Set("user", info)

	var info2 struct {
		Age  int
		Name string
	}
	dict.Get("user", &info2)
	t.Log(info2)
	return
}

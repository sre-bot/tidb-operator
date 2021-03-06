// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package manager

import "errors"

// VM defines the descriptive information of a virtual machine
type VM struct {
	Host   string   `json:"host"`
	Port   int64    `json:"port"`
	Name   string   `json:"name"`
	IP     string   `json:"ip"`
	Role   []string `json:"role"`
	Status string   `json:"status"`
}

func (v *VM) Verify() error {
	if len(v.Name) == 0 && len(v.IP) == 0 {
		return errors.New("name or ip must be provided")
	}

	return nil
}

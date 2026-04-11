/*
 * Copyright (c) Huawei Technologies Co., Ltd. 2025. All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package watcher

import (
	"encoding/json"
	"errors"
	"frontend/pkg/frontend/functiontask"
	"reflect"
	"testing"
	"time"

	"frontend/pkg/common/faas_common/etcd3"

	"github.com/agiledragon/gomonkey/v2"

	"github.com/smartystreets/goconvey/convey"
)

func TestProcessNodeEvent(t *testing.T) {
	convey.Convey("test process node event", t, func() {
		convey.Convey("process err", func() {
			defer gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v interface{}) error {
				return errors.New("")
			}).Reset()
			convey.Convey("add event", func() {
				defer gomonkey.ApplyMethod(reflect.TypeOf(functiontask.GetBusProxies()), "Add", func(_ *functiontask.BusProxies,
					nodeID, nodeIp string) {
				}).Reset()
				event := &etcd3.Event{Type: etcd3.PUT}
				processFunctionTaskEvent(event)
			})
			convey.Convey("remove event", func() {
				defer gomonkey.ApplyMethod(reflect.TypeOf(functiontask.GetBusProxies()), "Delete", func(_ *functiontask.BusProxies,
					nodeID string) {
				}).Reset()
				event := &etcd3.Event{Type: etcd3.DELETE}
				processFunctionTaskEvent(event)
			})
		})
		convey.Convey("process ok", func() {
			defer gomonkey.ApplyFunc(json.Unmarshal, func(data []byte, v interface{}) error {
				return nil
			}).Reset()
			convey.Convey("add event", func() {
				defer gomonkey.ApplyMethod(reflect.TypeOf(functiontask.GetBusProxies()), "Add", func(_ *functiontask.BusProxies,
					nodeID, nodeIp string) {
				}).Reset()
				event := &etcd3.Event{Type: etcd3.PUT}
				processFunctionTaskEvent(event)
			})
			convey.Convey("remove event", func() {
				defer gomonkey.ApplyMethod(reflect.TypeOf(functiontask.GetBusProxies()), "Delete", func(_ *functiontask.BusProxies,
					nodeID string) {
				}).Reset()
				event := &etcd3.Event{Type: etcd3.DELETE}
				processFunctionTaskEvent(event)
			})
		})
		convey.Convey("test other event", func() {
			convey.Convey("test etcd err event", func() {
				event := &etcd3.Event{Type: etcd3.ERROR}
				processFunctionTaskEvent(event)
			})
			convey.Convey("test etcd undefined event", func() {
				event := &etcd3.Event{Type: -1}
				processFunctionTaskEvent(event)
			})
		})
	})
}

func TestIsTaskNode(t *testing.T) {
	convey.Convey("Test IsTaskNode", t, func() {
		convey.Convey("is not TaskNode", func() {
			key1 := &etcd3.Event{
				Key: "",
			}
			convey.So(IsTaskNode(key1), convey.ShouldBeTrue)
			key2 := &etcd3.Event{
				Key: " /sn/workers/business/yrk/tenant/0/function/function/version/$latest/defaultaz/node01",
			}
			convey.So(IsTaskNode(key2), convey.ShouldBeTrue)
		})
		convey.Convey("is node", func() {
			key := &etcd3.Event{
				Key: " /sn/workers/business/yrk/tenant/0/function/function-task/version/$latest/defaultaz/node01",
			}
			convey.So(IsTaskNode(key), convey.ShouldBeFalse)
		})
	})
}

func TestStartWatchFunctionProxy(t *testing.T) {
	convey.Convey("StartWatch", t, func() {
		patches := []*gomonkey.Patches{
			gomonkey.ApplyMethod(reflect.TypeOf(&etcd3.EtcdWatcher{}), "StartWatch", func(ew *etcd3.EtcdWatcher) {
			}),
			gomonkey.ApplyMethod(reflect.TypeOf(&etcd3.EtcdClient{}), "AttachAZPrefix", func(ew *etcd3.EtcdClient, key string) string {
				return key
			}),
		}
		defer func() {
			for _, patch := range patches {
				time.Sleep(100 * time.Millisecond)
				patch.Reset()
			}
		}()
		startWatchFunctionProxy(make(chan struct{}))
	})
}

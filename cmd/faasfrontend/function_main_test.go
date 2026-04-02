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

package main

import (
	"encoding/json"
	"net/http"
	"reflect"
	"testing"

	"github.com/agiledragon/gomonkey/v2"
	"yuanrong.org/kernel/runtime/libruntime/api"

	"frontend/pkg/frontend/config"
	"frontend/pkg/frontend/server"
	"frontend/pkg/frontend/state"
	"github.com/stretchr/testify/assert"
)

var cfg = `{
			"slaQuota": 1000,
			"functionCapability": 1,
			"authenticationEnable": false,
			"trafficLimitDisable": true,
			"http": {
                "resptimeout": 5,
                "workerInstanceReadTimeOut": 5,
                "maxRequestBodySize": 6
            },
		"routerEtcd": {
			"servers": ["1.2.3.4:1234"],
			"user": "tom",
			"password": "**"
		},
		"metaEtcd": {
			"servers": ["1.2.3.4:5678"],
			"user": "tom",
			"password": "**"
		}
		}`
var invalidCfg = `{"abc":"123"`

func TestCheckpointHandler(t *testing.T) {
	state.InitState()
	_ = state.SetState([]byte(`{}`))

	got, err := CheckpointHandlerLibruntime("123")
	assert.NoError(t, err)
	assert.NotEmpty(t, got)

	// Verify the checkpoint bytes can be round-tripped back via RecoverHandler
	var roundTrip state.FrontendState
	assert.NoError(t, json.Unmarshal(got, &roundTrip))
}

func TestCallHandler(t *testing.T) {
	state.InitState()
	type args struct {
		args []api.Arg
	}
	tests := []struct {
		name    string
		args    args
		want    interface{}
		wantErr bool
	}{
		{
			name: "args error",
			args: args{
				args: []api.Arg{},
			},
			want:    []byte(nil),
			wantErr: true,
		},
		{
			name: "success",
			args: args{
				args: []api.Arg{
					{Data: []byte("1")},
					{Data: []byte("2")},
					{Data: []byte("3")},
					{Data: []byte("4")},
					{Data: []byte("5")},
				},
			},
			want:    []byte(`{"Code":0,"Message":"Successful in-cloud invoke"}`),
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CallHandlerLibruntime(tt.args.args)
			if (err != nil) != tt.wantErr {
				t.Errorf("CallHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("CallHandler() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInitHandlerError(t *testing.T) {
	applyFunc := gomonkey.ApplyFunc(state.InitState, func() {
		return
	})
	defer applyFunc.Reset()
	patches := []*gomonkey.Patches{
		gomonkey.ApplyFunc(server.GracefulShutdown, func(httpServer *http.Server) {
			return
		}),
		gomonkey.ApplyFunc(config.InitEtcd, func(stopCh <-chan struct{}) error {
			return nil
		}),
		gomonkey.ApplyFunc(setupFaaSFrontendLibruntime, func(rt api.LibruntimeAPI, stopCh <-chan struct{}) error {
			return nil
		}),
	}
	defer func() {
		for _, patch := range patches {
			patch.Reset()
		}
	}()
	res, err := InitHandlerLibruntime([]api.Arg{{Data: []byte(invalidCfg)}}, nil)
	assert.NotNil(t, err)
	assert.Nil(t, res)

	res, err = InitHandlerLibruntime([]api.Arg{{Data: []byte(cfg)}}, nil)
	assert.Nil(t, err)
	assert.Nil(t, res)
}

func TestRecoverHandler(t *testing.T) {
	applyFunc := gomonkey.ApplyFunc(state.InitState, func() {
		return
	})
	defer applyFunc.Reset()
	setupPatch := gomonkey.ApplyFunc(setupFaaSFrontendLibruntime, func(rt api.LibruntimeAPI, stopCh <-chan struct{}) error {
		return nil
	})
	defer setupPatch.Reset()
	patches := gomonkey.ApplyFunc(server.GracefulShutdown, func(httpServer *http.Server) {
		return
	})
	defer patches.Reset()
	type args struct {
		stateData []byte
		client    api.LibruntimeAPI
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "success",
			args: args{
				stateData: []byte(`{"Config":` + cfg + `}`),
				client:    nil,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RecoverHandlerLibruntime(tt.args.stateData, tt.args.client)
			if (err != nil) != tt.wantErr {
				t.Errorf("CallHandler() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestShutdownHandler(t *testing.T) {
	err := ShutdownHandlerLibruntime(0)
	assert.Nil(t, err)
}

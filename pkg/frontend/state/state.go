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

// Package state -
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"frontend/pkg/common/faas_common/logger/log"
	"frontend/pkg/common/faas_common/state"
	"frontend/pkg/frontend/config"
	"frontend/pkg/frontend/types"
)

// FrontendState add the status to be saved here.
type FrontendState struct {
	Config types.Config `json:"Config" valid:"optional"`
}

const defaultHandlerQueueSize = 1000

var (
	frontendState        = &FrontendState{}
	frontendStateLock    sync.RWMutex
	frontendHandlerQueue *state.Queue
	stateKey             = ""
)

func init() {
	frontendInstanceIDSelf := os.Getenv("INSTANCE_ID")
	stateKey = "/faas/state/recover/faasfrontend/" + frontendInstanceIDSelf
}

// InitState -
func InitState() {
	cfg := config.GetConfig()
	if cfg.StateDisable {
		log.GetLogger().Warnf("state is disable, skip init state")
		return
	}
	if frontendHandlerQueue != nil {
		return
	}
	frontendHandlerQueue = state.NewStateQueue(defaultHandlerQueueSize)
	if frontendHandlerQueue == nil {
		return
	}
	go frontendHandlerQueue.Run(updateState)
}

// SetState -
func SetState(byte []byte) error {
	return json.Unmarshal(byte, frontendState)
}

// GetState -
func GetState() FrontendState {
	frontendStateLock.RLock()
	defer frontendStateLock.RUnlock()
	return *frontendState
}

// GetStateByte is used to obtain the local state
func GetStateByte() ([]byte, error) {
	frontendStateLock.RLock()
	defer frontendStateLock.RUnlock()
	if frontendHandlerQueue == nil {
		return json.Marshal(frontendState)
	}
	stateBytes, err := frontendHandlerQueue.GetState(stateKey)
	if err != nil {
		return nil, err
	}
	log.GetLogger().Debugf("get state from etcd frontendState: %v", string(stateBytes))
	return stateBytes, nil
}

// DeleteStateByte -
func DeleteStateByte() error {
	if frontendHandlerQueue == nil {
		return fmt.Errorf("frontendHandlerQueue is not initialized")
	}
	frontendStateLock.RLock()
	defer frontendStateLock.RUnlock()
	return frontendHandlerQueue.DeleteState(stateKey)
}

func updateState(value interface{}, tags ...string) {
	if frontendHandlerQueue == nil {
		log.GetLogger().Errorf("frontend state frontendHandlerQueue is nil")
		return
	}
	frontendStateLock.Lock()
	defer frontendStateLock.Unlock()
	switch v := value.(type) {
	case types.Config:
		frontendState.Config = v
		log.GetLogger().Infof("update frontend state for config")
	default:
		log.GetLogger().Warnf("unknown data type for FrontendState")
		return
	}

	state, err := json.Marshal(frontendState)
	if err != nil {
		log.GetLogger().Errorf("get frontend state error %s", err.Error())
		return
	}
	if err = frontendHandlerQueue.SaveState(state, stateKey); err != nil {
		log.GetLogger().Errorf("save frontend state error: %s", err.Error())
		return
	}
	log.GetLogger().Info("update frontend state")
}

// Update is used to write frontend state to the cache queue
func Update(value interface{}, tags ...string) {
	if frontendHandlerQueue == nil {
		return
	}
	if err := frontendHandlerQueue.Push(value, tags...); err != nil {
		log.GetLogger().Errorf("failed to push state to state queue: %s", err.Error())
	}
}

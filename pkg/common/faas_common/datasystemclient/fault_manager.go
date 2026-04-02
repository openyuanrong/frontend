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

package datasystemclient

import (
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"frontend/pkg/common/faas_common/logger/log"
)

const (
	// 节点已准备好对外服务
	dataSystemStatusReady = "ready"
	// 节点启动
	dataSystemStatusStart = "start"
	// 节点重启
	dataSystemStatusRestart = "restart"
	// 节点对账恢复中
	dataSystemStatusRecover = "recover"
	// etcd故障期间重启的状态
	dataSystemStatusDRst = "d_rst"
	// 节点退出（主动缩容）
	dataSystemStatusExiting = "exiting"
)

const (
	timeoutSeconds   = 3 * time.Second
	failureThreshold = 5
)

var (
	// localDataSystemStatusCache 存储本节点数据系统状态
	localDataSystemStatusCache = &LocalDataSystemStatusCache{}
	// shutdownFlag 防止并发触发destroy()动作
	shutdownFlag = atomic.Bool{}
	// streamEnable 识别是否是流场景，可开启监听本节点数据系统状态
	streamEnable = atomic.Bool{}
	// etcdStatusReady 记录etcd中本节点数据系统状态是否ready，识别升级场景需要重启frontend
	etcdStatusReady = atomic.Bool{}
	// healthCheckResult 保存libruntime接口返回的数据系统状态
	healthCheckResult = atomic.Bool{}
	// healthCheckOnce 本节点数据系统ready后，启动监听动作，防止重复启动
	healthCheckOnce sync.Once
	// readinessFailureThreshold 累计readiness失败次数，保证满足摘流条件
	readinessFailureThreshold int32
)

// LocalDataSystemStatusCache 本地数据系统状态缓存结构体
type LocalDataSystemStatusCache struct {
	status string
	lock   sync.RWMutex
}

// IsStatusReady -
func (d *LocalDataSystemStatusCache) IsStatusReady() bool {
	d.lock.RLock()
	defer d.lock.RUnlock()
	if d.status != dataSystemStatusReady {
		log.GetLogger().Debugf("data system status is not ready, status: %s", d.status)
		return false
	}
	return true
}

// SetLocalDataSystemStatus -
func (d *LocalDataSystemStatusCache) SetLocalDataSystemStatus(ip, status string) {
	d.lock.Lock()
	defer d.lock.Unlock()
	localNode := os.Getenv("NODE_IP")
	if localNode == "" {
		log.GetLogger().Debugf("get local node is empty")
		return
	}
	if ip != localNode {
		log.GetLogger().Debugf("node[%s] is not local data system node[%s]", ip, localNode)
		return
	}
	log.GetLogger().Infof("save local data system node[%s] status[%s]", ip, status)
	if status == dataSystemStatusReady {
		// 首次启动或重启后，数据系统状态可能长时间非ready，需要首次ready后才开启协程检查healthCheck接口的调用，防止造成反复重启
		healthCheckOnce.Do(func() {
			etcdStatusReady.Store(true)
			go dataSystemHealthCheck()
		})
	}
	d.status = status
}

// GetLocalDataSystemStatus -
func (d *LocalDataSystemStatusCache) GetLocalDataSystemStatus() string {
	d.lock.RLock()
	defer d.lock.RUnlock()
	return d.status
}

// IsLocalDataSystemStatusReady -
func IsLocalDataSystemStatusReady() bool {
	//先判断etcdStatusReady状态,etcd监听到ready之前，健康检查应该是一直非ready的；
	// 首次ready之后，再遇到非ready之后始终返回false，保证摘流和触发重启
	if !etcdStatusReady.Load() {
		atomic.AddInt32(&readinessFailureThreshold, 1)
		return false
	}
	if !localDataSystemStatusCache.IsStatusReady() {
		etcdStatusReady.Store(false)
	}
	if healthCheckResult.Load() {
		atomic.SwapInt32(&readinessFailureThreshold, 0)
		return true
	} else {
		atomic.AddInt32(&readinessFailureThreshold, 1)
		return false
	}
}

// SetStreamEnable -
func SetStreamEnable(streamEnableConfig bool) {
	streamEnable.Store(streamEnableConfig)
}

// GetStreamEnable -
func GetStreamEnable() bool {
	return streamEnable.Load()
}

func isShutdownFronted() bool {
	if !streamEnable.Load() {
		log.GetLogger().Infof("it's not stream scenario, skip shutdown frontend")
		return false
	}
	if atomic.LoadInt32(&readinessFailureThreshold) > failureThreshold {
		log.GetLogger().Infof("readiness has failed %d times, shutdown frontend", failureThreshold)
		return true
	}
	return false
}

func destroy() {
	if shutdownFlag.Swap(true) {
		log.GetLogger().Infof("shutdown frontend has been triggered, skip this operation")
		return
	}
	shutdownFlag.Store(true)
	defer func() {
		shutdownFlag.Store(false)
	}()
	log.GetLogger().Infof("local dataSystem status is not ready, prepare shutdown frontend")
	pid := os.Getpid()
	proc, err := os.FindProcess(pid)
	if err != nil {
		log.GetLogger().Errorf("get process pid failed, pid: %d, err: %v", pid, err)
		return
	}
	err = proc.Signal(syscall.SIGTERM)
	if err != nil {
		log.GetLogger().Errorf("send SIGTERM signal to the process failed, pid: %d, err: %v", pid, err)
		return
	}
	log.GetLogger().Infof("send SIGTERM signal to the process success, pid: %d", pid)
}

func dataSystemHealthCheck() {
	if !streamEnable.Load() {
		return
	}
	log.GetLogger().Infof("start to check dataSystem health")
	atomic.SwapInt32(&readinessFailureThreshold, 0)
	timer := time.NewTimer(timeoutSeconds)
	defer timer.Stop()
	for {
		// 等待定时器触发
		<-timer.C
		healthCheckResult.Store(localClientLibruntime.IsDsHealth())
		if isShutdownFronted() {
			destroy()
			return
		}
		// 重置定时器以继续下一次循环
		timer.Reset(timeoutSeconds)
	}
}

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

package pod

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pingcap/tidb-operator/pkg/apis/pingcap/v1alpha1"
	"github.com/pingcap/tidb-operator/pkg/pdapi"
	"github.com/pingcap/tidb-operator/pkg/webhook/util"
	admission "k8s.io/api/admission/v1beta1"
	core "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

const (
	tikvNotBootstrapped  = `TiKV cluster not bootstrapped, please start TiKV first"`
	evictSchedulerLeader = "evict-leader-scheduler"
)

// Payload only used to unmarshal the data from pdapi
type Payload struct {
	StoreIdRanges map[string]interface{} `json:"store-id-ranges"`
}

func (pc *PodAdmissionControl) admitCreateTiKVPod(pod *core.Pod, tc *v1alpha1.TidbCluster, pdClient pdapi.PDClient) *admission.AdmissionResponse {

	name := pod.Name
	namespace := pod.Namespace

	stores, err := pdClient.GetStores()
	if err != nil {
		if strings.HasSuffix(err.Error(), tikvNotBootstrapped+"\n") {
			return util.ARSuccess()
		}
		klog.Infof("Failed to get stores during pod [%s/%s] creation, error: %v", namespace, name, err)
		return util.ARFail(err)
	}
	evictLeaderSchedulers, err := pdClient.GetEvictLeaderSchedulers()
	if err != nil {
		if strings.HasSuffix(err.Error(), tikvNotBootstrapped+"\n") {
			return util.ARSuccess()
		}
		klog.Infof("failed to create pod[%s/%s],%v", namespace, name, err)
		return util.ARFail(err)
	}

	if stores.Count < 1 {
		return util.ARSuccess()
	}

	if len(evictLeaderSchedulers) < 1 {
		return util.ARSuccess()
	}

	schedulerIds, err := filterLeaderEvictScheduler(evictLeaderSchedulers, pdClient)
	if err != nil {
		return util.ARFail(err)
	}

	// if the pod which is going to be created already have a store and was in evictLeaderSchedulers,
	// we should end this evict leader
	for _, store := range stores.Stores {
		ip := strings.Split(store.Store.GetAddress(), ":")[0]
		podName := strings.Split(ip, ".")[0]
		if podName == name && schedulerIds.Has(fmt.Sprintf("%d", store.Store.Id)) {
			err := endEvictLeader(store, pdClient)
			if err != nil {
				klog.Infof("failed to create pod[%s/%s],%v", namespace, name, err)
				return util.ARFail(err)
			}
			break
		}
	}

	return util.ARSuccess()
}

// This method is to make compatible between old pdapi version and 4.0 pdapi version.
// To get more detail, see: https://github.com/pingcap/tidb-operator/pull/1831
func filterLeaderEvictScheduler(evictLeaderSchedulers []string, pdClient pdapi.PDClient) (sets.String, error) {
	schedulerIds := sets.String{}
	if len(evictLeaderSchedulers) == 1 && evictLeaderSchedulers[0] == evictSchedulerLeader {
		c, err := pdClient.GetConfig()
		if err != nil {
			return schedulerIds, err
		}
		if c.Schedule != nil {
			if c.Schedule.SchedulersPayload == nil {
				c.Schedule.SchedulersPayload = map[string]string{}
			}
			v, ok := c.Schedule.SchedulersPayload[evictSchedulerLeader]
			if ok {
				payload := &Payload{}
				err := json.Unmarshal([]byte(v), payload)
				if err != nil {
					return schedulerIds, err
				}
				for k := range payload.StoreIdRanges {
					schedulerIds.Insert(k)
				}
			}
		}
	} else {
		for _, s := range evictLeaderSchedulers {
			id := strings.Split(s, "-")[3]
			schedulerIds.Insert(id)
		}
	}
	return schedulerIds, nil
}

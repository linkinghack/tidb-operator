// Copyright 2020 PingCAP, Inc.
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

package mock

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pingcap/tidb-operator/pkg/autoscaler/autoscaler/calculate"
	"k8s.io/kubernetes/test/e2e/framework/log"
)

type MonitorInterface interface {
	ServeQuery(w http.ResponseWriter, r *http.Request)
	ServeTargets(w http.ResponseWriter, r *http.Request)
	SetResponse(w http.ResponseWriter, r *http.Request)
}

type mockPrometheus struct {
	// responses store the key from the query and value to answer the query
	// its not thread-safe, use it carefully
	responses map[string]string
}

func NewMockPrometheus() MonitorInterface {
	mp := &mockPrometheus{
		responses: map[string]string{},
	}
	params := &MonitorParams{}
	upResp := buildPrometheusResponse(params)
	b, err := json.Marshal(upResp)
	if err != nil {
		log.Failf(err.Error())
	}
	mp.responses["up"] = string(b)
	return mp
}

func (m *mockPrometheus) ServeQuery(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		writeResponse(w, "parse query form failed")
		return
	}
	key := r.Form.Get("query")
	if len(key) < 1 {
		key = r.URL.Query().Get("query")
		if len(key) < 1 {
			writeResponse(w, "no query param")
			return
		}
	}
	log.Logf("receive query, key: %s", key)
	v, ok := m.responses[key]
	if !ok {
		writeResponse(w, "no response value found")
		return
	}
	writeResponse(w, v)
}

func (m *mockPrometheus) SetResponse(w http.ResponseWriter, r *http.Request) {
	mp := &MonitorParams{}
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeResponse(w, err.Error())
		return
	}
	if err := json.Unmarshal(body, mp); err != nil {
		writeResponse(w, err.Error())
		return
	}

	b, err := json.Marshal(buildPrometheusResponse(mp))
	if err != nil {
		writeResponse(w, err.Error())
		return
	}

	m.addIntoMaps(mp, string(b))
	writeResponse(w, "ok")
}

func (m *mockPrometheus) ServeTargets(w http.ResponseWriter, r *http.Request) {
	data := &MonitorTargets{
		Status: "success",
		Data: MonitorTargetsData{
			ActiveTargets: []ActiveTargets{
				{
					DiscoveredLabels: DiscoveredLabels{
						Job:     "job",
						PodName: "pod",
					},
					Health: "true",
				},
			},
		},
	}
	b, err := json.Marshal(data)
	if err != nil {
		writeResponse(w, err.Error())
		return
	}
	writeResponse(w, string(b))
}

func (m *mockPrometheus) addIntoMaps(mp *MonitorParams, response string) {
	currentType := mp.QueryType
	key := ""
	name := mp.Name
	memberType := mp.MemberType
	duration := mp.Duration
	log.Logf("name=%s, memberType =%s, duration =%s, response =%s", name, memberType, duration, response)
	if memberType == "tidb" {
		if currentType == "cpu_usage" {
			key = fmt.Sprintf(calculate.TidbSumCPUUsageMetricsPattern, duration)
		} else if currentType == "cpu_quota" {
			key = calculate.TidbCPUQuotaMetricsPattern
		}
	} else if memberType == "tikv" {
		if currentType == "cpu_usage" {
			key = fmt.Sprintf(calculate.TikvSumCPUUsageMetricsPattern, duration)
		} else if currentType == "cpu_quota" {
			key = calculate.TikvCPUQuotaMetricsPattern
		}
	}
	m.responses[key] = response
	log.Logf("add key: %s with value: %s", key, response)

}

func writeResponse(w http.ResponseWriter, msg string) {
	if _, err := w.Write([]byte(msg)); err != nil {
		log.Logf("ERROR: %v", err)
	}
}

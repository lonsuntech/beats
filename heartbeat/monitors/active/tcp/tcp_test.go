// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package tcp

import (
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/phayes/freeport"

	"net/http"

	"github.com/elastic/beats/heartbeat/hbtest"
	"github.com/elastic/beats/heartbeat/monitors"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/common/mapval"
	"github.com/elastic/beats/libbeat/testing/mapvaltest"
)

func testTCPCheck(t *testing.T, host string, port uint16) *beat.Event {
	config := common.NewConfig()
	config.SetString("hosts", 0, host)
	config.SetInt("ports", 0, int64(port))

	jobs, err := create(monitors.Info{}, config)
	require.NoError(t, err)

	job := jobs[0]

	event, _, err := job.Run()
	require.NoError(t, err)

	return &event
}

func tcpMonitorChecks(host string, ip string, port uint16, status string) mapval.Validator {
	id := fmt.Sprintf("tcp-tcp@%s:%d", host, port)
	return hbtest.MonitorChecks(id, host, ip, "tcp", status)
}

func TestUpEndpointJob(t *testing.T) {
	server := httptest.NewServer(hbtest.HelloWorldHandler(http.StatusOK))
	defer server.Close()

	port, err := hbtest.ServerPort(server)
	require.NoError(t, err)

	event := testTCPCheck(t, "localhost", port)

	mapvaltest.Test(
		t,
		mapval.Strict(mapval.Compose(
			hbtest.MonitorChecks(
				fmt.Sprintf("tcp-tcp@localhost:%d", port),
				"localhost",
				"127.0.0.1",
				"tcp",
				"up",
			),
			hbtest.RespondingTCPChecks(port),
			mapval.Schema(mapval.Map{
				"resolve": mapval.Map{
					"host":   "localhost",
					"ip":     "127.0.0.1",
					"rtt.us": mapval.IsDuration,
				},
			}),
		)),
		event.Fields,
	)
}

func TestConnectionRefusedEndpointJob(t *testing.T) {
	ip := "127.0.0.1"
	port := uint16(freeport.GetPort())
	event := testTCPCheck(t, ip, port)

	mapvaltest.Test(
		t,
		mapval.Strict(mapval.Compose(
			tcpMonitorChecks(ip, ip, port, "down"),
			hbtest.ErrorChecks(fmt.Sprintf("dial tcp %s:%d: connect: connection refused", ip, port), "io"),
			hbtest.TCPBaseChecks(port),
		)),
		event.Fields,
	)
}

func TestUnreachableEndpointJob(t *testing.T) {
	ip := "203.0.113.1"
	port := uint16(1234)
	event := testTCPCheck(t, ip, port)

	mapvaltest.Test(
		t,
		mapval.Strict(mapval.Compose(
			tcpMonitorChecks(ip, ip, port, "down"),
			hbtest.ErrorChecks(
				mapval.IsAny(
					mapval.IsEqual(fmt.Sprintf("dial tcp %s:%d: i/o timeout", ip, port)),
					mapval.IsEqual(fmt.Sprintf("dial tcp %s:%d: connect: network is unreachable", ip, port)),
				),
				"io"),
			hbtest.TCPBaseChecks(port),
		)),
		event.Fields,
	)
}

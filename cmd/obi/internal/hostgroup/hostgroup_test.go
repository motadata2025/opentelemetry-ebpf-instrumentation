// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hostgroup

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/otel/attribute"

	"go.opentelemetry.io/obi/pkg/appolly/app/request"
	"go.opentelemetry.io/obi/pkg/appolly/app/svc"
	"go.opentelemetry.io/obi/pkg/appolly/meta"
	"go.opentelemetry.io/obi/pkg/export/otel/tracesgen"
	"go.opentelemetry.io/obi/pkg/pipe/global"
)

func TestRead(t *testing.T) {
	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{
			name: "value set",
			contents: `{
				"agent": {
					"system.log.level": 2,
					"agent.id": "",
					"trace.agent.status": "yes",
					"trace.host.group": "shiven"
				},
				"log.agent": {"system.log.level": 2}
			}`,
			want: "shiven",
		},
		{
			name:     "empty value",
			contents: `{"agent": {"trace.host.group": ""}}`,
			want:     "",
		},
		{
			name:     "missing key",
			contents: `{"agent": {"agent.id": "abc"}}`,
			want:     "",
		},
		{
			name:     "key outside agent section",
			contents: `{"log.agent": {"trace.host.group": "other"}}`,
			want:     "",
		},
		{
			name:     "non-string value",
			contents: `{"agent": {"trace.host.group": 5}}`,
			want:     "",
		},
		{
			name:     "invalid json",
			contents: `{"agent": `,
			want:     "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "agent.json")
			require.NoError(t, os.WriteFile(path, []byte(tc.contents), 0o600))

			assert.Equal(t, tc.want, Read(path))
		})
	}
}

func TestAgentConfigPath(t *testing.T) {
	assert.Equal(t, "/motadata/motadata/config/agent.json", agentConfigPath("/motadata/motadata/motadata-ebpf-agent"))
}

func TestRead_MissingFile(t *testing.T) {
	assert.Empty(t, Read(filepath.Join(t.TempDir(), "agent.json")))
}

func TestWithResourceAttribute_SetsTraceResourceAttribute(t *testing.T) {
	for _, hostGroup := range []string{"shiven", ""} {
		t.Run("host group "+strconv.Quote(hostGroup), func(t *testing.T) {
			info := &global.ContextInfo{}
			WithResourceAttribute(hostGroup)(info)

			start := time.Now()
			span := &request.Span{
				Type:         request.EventTypeHTTP,
				RequestStart: start.UnixNano(),
				Start:        start.UnixNano(),
				End:          start.Add(time.Second).UnixNano(),
				Method:       "GET",
				Route:        "/test",
				Status:       200,
				Service:      svc.Attrs{UID: svc.UID{Name: "test-service"}},
			}
			appResourceAttrs := []attribute.KeyValue{attribute.String(attributeName, "set-by-app")}
			cache := expirable.NewLRU[svc.UID, []attribute.KeyValue](1, nil, time.Minute)

			traces := tracesgen.GenerateTracesWithSelectedResourceAttributes(
				cache,
				&span.Service,
				appResourceAttrs,
				&meta.NodeMeta{},
				[]tracesgen.TraceSpanAndAttributes{{Span: span}},
				"test",
				nil,
				info.ExtraResourceAttributes...)

			require.Equal(t, 1, traces.ResourceSpans().Len())
			value, ok := traces.ResourceSpans().At(0).Resource().Attributes().Get(attributeName)
			require.True(t, ok)
			assert.Equal(t, hostGroup, value.Str())
		})
	}
}

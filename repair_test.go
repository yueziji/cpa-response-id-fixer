package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

func TestRepairStreamChunkUsesHistoryResponseID(t *testing.T) {
	req := pluginapi.StreamChunkInterceptRequest{
		Body: []byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"),
		HistoryChunks: [][]byte{
			[]byte("data: {\"type\":\"response.created\",\"response\":{\"id\":\"resp_real\",\"status\":\"in_progress\"}}\n\n"),
		},
	}

	repaired := repairStreamChunk(req)
	if len(repaired) == 0 {
		t.Fatal("expected repaired chunk")
	}
	if !strings.Contains(string(repaired), `"id":"resp_real"`) {
		t.Fatalf("repaired chunk does not contain history id: %s", repaired)
	}
	if !strings.Contains(string(repaired), "event: response.completed") {
		t.Fatalf("repaired chunk lost event line: %s", repaired)
	}
}

func TestRepairStreamChunkSynthesizesID(t *testing.T) {
	req := pluginapi.StreamChunkInterceptRequest{
		Model:           "test-model",
		RequestedModel:  "test-model",
		OriginalRequest: []byte(`{"model":"test-model","stream":true}`),
		RequestBody:     []byte(`{"model":"test-model","stream":true}`),
		ChunkIndex:      2,
		Body:            []byte(`data: {"type":"response.completed","response":{"id":"","output":[]}}`),
	}

	repaired := repairStreamChunk(req)
	if len(repaired) == 0 {
		t.Fatal("expected repaired chunk")
	}
	if !strings.Contains(string(repaired), `"id":"`+fallbackIDPrefix) {
		t.Fatalf("repaired chunk does not contain fallback id: %s", repaired)
	}
}

func TestRepairResponseCompletedPureJSON(t *testing.T) {
	req := pluginapi.StreamChunkInterceptRequest{
		Body: []byte(`{"type":"response.completed","response":{"output":[]}}`),
	}

	repaired := repairStreamChunk(req)
	if len(repaired) == 0 {
		t.Fatal("expected repaired JSON")
	}
	var root map[string]any
	if errUnmarshal := json.Unmarshal(repaired, &root); errUnmarshal != nil {
		t.Fatalf("repaired JSON is invalid: %v", errUnmarshal)
	}
	response := root["response"].(map[string]any)
	if id, _ := response["id"].(string); !strings.HasPrefix(id, fallbackIDPrefix) {
		t.Fatalf("response.id = %q, want fallback id", id)
	}
}

func TestRepairStreamChunkLeavesUnrelatedEventsAlone(t *testing.T) {
	req := pluginapi.StreamChunkInterceptRequest{
		Body: []byte(`data: {"type":"response.output_text.delta","delta":"ok"}`),
	}
	if repaired := repairStreamChunk(req); repaired != nil {
		t.Fatalf("unexpected repair: %s", repaired)
	}
}

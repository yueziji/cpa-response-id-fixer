package main

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

var benchmarkHandleMethodResult []byte

func BenchmarkHandleMethodDeltaChunkOpenAIResponse(b *testing.B) {
	rawReq := benchmarkStreamChunkRequest([]byte(`data: {"type":"response.output_text.delta","delta":"hello"}`), 42)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rawResp, errHandle := handleMethod(pluginabi.MethodResponseInterceptStreamChunk, rawReq)
		if errHandle != nil {
			b.Fatalf("handleMethod: %v", errHandle)
		}
		benchmarkHandleMethodResult = rawResp
	}
}

func BenchmarkHandleMethodCompletedChunkOpenAIResponse(b *testing.B) {
	rawReq := benchmarkStreamChunkRequest([]byte("event: response.completed\ndata: {\"type\":\"response.completed\",\"response\":{\"output\":[]}}\n\n"), 64)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rawResp, errHandle := handleMethod(pluginabi.MethodResponseInterceptStreamChunk, rawReq)
		if errHandle != nil {
			b.Fatalf("handleMethod: %v", errHandle)
		}
		benchmarkHandleMethodResult = rawResp
	}
}

func benchmarkStreamChunkRequest(body []byte, chunkIndex int) []byte {
	req := pluginapi.StreamChunkInterceptRequest{
		SourceFormat:   openAIResponseSourceFormat,
		Model:          "gpt-4.1",
		RequestedModel: "gpt-4.1",
		RequestHeaders: http.Header{
			"Authorization": []string{"Bearer benchmark"},
			"Content-Type":  []string{"application/json"},
		},
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		OriginalRequest: []byte(`{"model":"gpt-4.1","stream":true,"input":"benchmark input"}`),
		RequestBody:     []byte(`{"model":"gpt-4.1","stream":true,"input":"benchmark input"}`),
		Body:            body,
		HistoryChunks:   benchmarkHistoryChunks(),
		ChunkIndex:      chunkIndex,
		Metadata: map[string]any{
			"request_id": "bench-request",
			"route":      "openai-responses",
			"attempt":    1,
		},
	}
	rawReq, errMarshal := json.Marshal(req)
	if errMarshal != nil {
		panic(errMarshal)
	}
	return rawReq
}

func benchmarkHistoryChunks() [][]byte {
	chunks := make([][]byte, 0, 32)
	for i := 0; i < 31; i++ {
		chunks = append(chunks, []byte(`data: {"type":"response.output_text.delta","delta":"benchmark text chunk"}`+"\n\n"))
	}
	chunks = append(chunks, []byte(`data: {"type":"response.created","response":{"id":"resp_benchmark","status":"in_progress"}}`+"\n\n"))
	return chunks
}

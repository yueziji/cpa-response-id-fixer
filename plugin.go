package main

import (
	"bytes"
	"encoding/json"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginabi"
	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

type envelope struct {
	OK     bool            `json:"ok"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *envelopeError  `json:"error,omitempty"`
}

type envelopeError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type registration struct {
	SchemaVersion uint32             `json:"schema_version"`
	Metadata      pluginapi.Metadata `json:"metadata"`
	Capabilities  capabilities       `json:"capabilities"`
}

type capabilities struct {
	StreamChunkInterceptor bool `json:"response_stream_interceptor"`
}

type streamChunkPeek struct {
	SourceFormat string          `json:"SourceFormat"`
	ChunkIndex   int             `json:"ChunkIndex"`
	Body         json.RawMessage `json:"Body"`
}

type streamChunkRepairContext struct {
	Model           string   `json:"Model"`
	RequestedModel  string   `json:"RequestedModel"`
	OriginalRequest []byte   `json:"OriginalRequest"`
	RequestBody     []byte   `json:"RequestBody"`
	HistoryChunks   [][]byte `json:"HistoryChunks"`
}

const openAIResponseSourceFormat = "openai-response"

var emptyStreamChunkInterceptResponseEnvelope = mustOKEnvelope(pluginapi.StreamChunkInterceptResponse{})

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodResponseInterceptStreamChunk:
		peek, body, handled, errShouldHandle := shouldHandleStreamChunk(request)
		if errShouldHandle != nil {
			return nil, errShouldHandle
		}
		if !handled {
			return emptyStreamChunkInterceptResponseEnvelope, nil
		}

		var repairContext streamChunkRepairContext
		if errUnmarshal := json.Unmarshal(request, &repairContext); errUnmarshal != nil {
			return nil, errUnmarshal
		}
		req := pluginapi.StreamChunkInterceptRequest{
			SourceFormat:    openAIResponseSourceFormat,
			Model:           repairContext.Model,
			RequestedModel:  repairContext.RequestedModel,
			OriginalRequest: repairContext.OriginalRequest,
			RequestBody:     repairContext.RequestBody,
			Body:            body,
			HistoryChunks:   repairContext.HistoryChunks,
			ChunkIndex:      peek.ChunkIndex,
		}
		repaired := repairStreamChunk(req)
		if len(repaired) == 0 {
			return emptyStreamChunkInterceptResponseEnvelope, nil
		}
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: repaired})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func shouldHandleStreamChunk(request []byte) (streamChunkPeek, []byte, bool, error) {
	var peek streamChunkPeek
	if errUnmarshal := json.Unmarshal(request, &peek); errUnmarshal != nil {
		return streamChunkPeek{}, nil, false, errUnmarshal
	}
	if peek.ChunkIndex == pluginapi.StreamChunkHeaderInitIndex {
		return peek, nil, false, nil
	}
	if peek.SourceFormat != openAIResponseSourceFormat {
		return peek, nil, false, nil
	}

	var body []byte
	if len(peek.Body) > 0 {
		if errUnmarshal := json.Unmarshal(peek.Body, &body); errUnmarshal != nil {
			return peek, nil, false, errUnmarshal
		}
	}
	if !streamChunkBodyMayNeedRepair(body) {
		return peek, nil, false, nil
	}
	return peek, body, true, nil
}

func streamChunkBodyMayNeedRepair(body []byte) bool {
	trimmed := bytes.TrimSpace(body)
	return len(trimmed) > 0 && bytes.Contains(trimmed, responseCompletedMarker)
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             "cpa-response-id-fixer",
			Version:          "0.0.5",
			Author:           "local",
			GitHubRepository: "https://github.com/router-for-me/CLIProxyAPI",
			ConfigFields:     []pluginapi.ConfigField{},
		},
		Capabilities: capabilities{
			StreamChunkInterceptor: true,
		},
	}
}

func okEnvelope(v any) ([]byte, error) {
	raw, errMarshal := json.Marshal(v)
	if errMarshal != nil {
		return nil, errMarshal
	}
	return json.Marshal(envelope{OK: true, Result: raw})
}

func mustOKEnvelope(v any) []byte {
	raw, errMarshal := okEnvelope(v)
	if errMarshal != nil {
		panic(errMarshal)
	}
	return raw
}

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

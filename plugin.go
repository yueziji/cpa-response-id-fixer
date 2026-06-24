package main

import (
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
	SourceFormat string `json:"SourceFormat"`
	ChunkIndex   int    `json:"ChunkIndex"`
}

const openAIResponseSourceFormat = "openai-response"

func handleMethod(method string, request []byte) ([]byte, error) {
	switch method {
	case pluginabi.MethodPluginRegister, pluginabi.MethodPluginReconfigure:
		return okEnvelope(pluginRegistration())
	case pluginabi.MethodResponseInterceptStreamChunk:
		handled, errShouldHandle := shouldHandleStreamChunk(request)
		if errShouldHandle != nil {
			return nil, errShouldHandle
		}
		if !handled {
			return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
		}
		var req pluginapi.StreamChunkInterceptRequest
		if errUnmarshal := json.Unmarshal(request, &req); errUnmarshal != nil {
			return nil, errUnmarshal
		}
		repaired := repairStreamChunk(req)
		if len(repaired) == 0 {
			return okEnvelope(pluginapi.StreamChunkInterceptResponse{})
		}
		return okEnvelope(pluginapi.StreamChunkInterceptResponse{Body: repaired})
	default:
		return errorEnvelope("unknown_method", "unknown method: "+method), nil
	}
}

func shouldHandleStreamChunk(request []byte) (bool, error) {
	var peek streamChunkPeek
	if errUnmarshal := json.Unmarshal(request, &peek); errUnmarshal != nil {
		return false, errUnmarshal
	}
	if peek.ChunkIndex == pluginapi.StreamChunkHeaderInitIndex {
		return false, nil
	}
	return peek.SourceFormat == openAIResponseSourceFormat, nil
}

func pluginRegistration() registration {
	return registration{
		SchemaVersion: pluginabi.SchemaVersion,
		Metadata: pluginapi.Metadata{
			Name:             "cpa-response-id-fixer",
			Version:          "0.0.2",
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

func errorEnvelope(code, message string) []byte {
	raw, _ := json.Marshal(envelope{OK: false, Error: &envelopeError{Code: code, Message: message}})
	return raw
}

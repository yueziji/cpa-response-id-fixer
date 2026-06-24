package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v7/sdk/pluginapi"
)

const fallbackIDPrefix = "resp_cpa_"

func repairStreamChunk(req pluginapi.StreamChunkInterceptRequest) []byte {
	if req.ChunkIndex == pluginapi.StreamChunkHeaderInitIndex || len(bytes.TrimSpace(req.Body)) == 0 {
		return nil
	}
	if repaired, changed := repairResponseCompletedJSON(req.Body, req); changed {
		return repaired
	}
	if repaired, changed := repairSSEDataLines(req.Body, req); changed {
		return repaired
	}
	return nil
}

func repairSSEDataLines(body []byte, req pluginapi.StreamChunkInterceptRequest) ([]byte, bool) {
	remaining := body
	var out bytes.Buffer
	changed := false

	for len(remaining) > 0 {
		line, eol, rest := nextLine(remaining)
		remaining = rest

		next := line
		prefix, data, ok := splitSSEDataLine(line)
		if ok {
			payload := bytes.TrimSpace(data)
			if len(payload) > 0 && !bytes.Equal(payload, []byte("[DONE]")) {
				if repaired, didRepair := repairResponseCompletedJSON(payload, req); didRepair {
					next = append(append(bytes.Clone(prefix), ' '), repaired...)
					changed = true
				}
			}
		}

		out.Write(next)
		out.Write(eol)
	}

	if !changed {
		return nil, false
	}
	return out.Bytes(), true
}

func nextLine(raw []byte) (line, eol, rest []byte) {
	index := bytes.IndexByte(raw, '\n')
	if index < 0 {
		return raw, nil, nil
	}
	line = raw[:index]
	eol = []byte("\n")
	if bytes.HasSuffix(line, []byte("\r")) {
		line = line[:len(line)-1]
		eol = []byte("\r\n")
	}
	return line, eol, raw[index+1:]
}

func splitSSEDataLine(line []byte) (prefix, data []byte, ok bool) {
	trimmedLeft := bytes.TrimLeft(line, " \t")
	leadingLen := len(line) - len(trimmedLeft)
	if !bytes.HasPrefix(trimmedLeft, []byte("data:")) {
		return nil, nil, false
	}
	prefixEnd := leadingLen + len("data:")
	return line[:prefixEnd], line[prefixEnd:], true
}

func repairResponseCompletedJSON(raw []byte, req pluginapi.StreamChunkInterceptRequest) ([]byte, bool) {
	root, ok := decodeJSONObject(raw)
	if !ok {
		return nil, false
	}
	if stringField(root, "type") != "response.completed" {
		return nil, false
	}
	response, ok := root["response"].(map[string]any)
	if !ok {
		return nil, false
	}
	if strings.TrimSpace(stringField(response, "id")) != "" {
		return nil, false
	}

	id := strings.TrimSpace(stringField(root, "response_id"))
	if id == "" {
		id = responseIDFromHistory(req.HistoryChunks)
	}
	if id == "" {
		id = synthesizeResponseID(req, raw)
	}
	response["id"] = id
	root["response"] = response

	repaired, errMarshal := json.Marshal(root)
	if errMarshal != nil {
		return nil, false
	}
	return repaired, true
}

func decodeJSONObject(raw []byte) (map[string]any, bool) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var root map[string]any
	if errDecode := decoder.Decode(&root); errDecode != nil {
		return nil, false
	}
	return root, true
}

func stringField(root map[string]any, key string) string {
	value, ok := root[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func responseIDFromHistory(history [][]byte) string {
	for i := len(history) - 1; i >= 0; i-- {
		if id := responseIDFromChunk(history[i]); id != "" {
			return id
		}
	}
	return ""
}

func responseIDFromChunk(chunk []byte) string {
	if id := responseIDFromJSON(chunk); id != "" {
		return id
	}

	remaining := chunk
	for len(remaining) > 0 {
		line, _, rest := nextLine(remaining)
		remaining = rest
		_, data, ok := splitSSEDataLine(line)
		if !ok {
			continue
		}
		payload := bytes.TrimSpace(data)
		if len(payload) == 0 || bytes.Equal(payload, []byte("[DONE]")) {
			continue
		}
		if id := responseIDFromJSON(payload); id != "" {
			return id
		}
	}
	return ""
}

func responseIDFromJSON(raw []byte) string {
	root, ok := decodeJSONObject(bytes.TrimSpace(raw))
	if !ok {
		return ""
	}
	if response, ok := root["response"].(map[string]any); ok {
		if id := strings.TrimSpace(stringField(response, "id")); id != "" {
			return id
		}
	}
	if id := strings.TrimSpace(stringField(root, "response_id")); id != "" {
		return id
	}
	return ""
}

func synthesizeResponseID(req pluginapi.StreamChunkInterceptRequest, payload []byte) string {
	hash := sha1.New()
	_, _ = hash.Write([]byte(req.Model))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write([]byte(req.RequestedModel))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(req.OriginalRequest)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(req.RequestBody)
	_, _ = hash.Write([]byte{0})
	_, _ = fmt.Fprintf(hash, "%d", req.ChunkIndex)
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(payload)
	sum := hash.Sum(nil)
	return fallbackIDPrefix + hex.EncodeToString(sum)[:16]
}

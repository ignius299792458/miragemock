package miragemock

import (
	"bytes"
	"log"
	"net/http"
)

// ReWriter defines the required behavioral contract for mutating
// request geometries on the fly before they hit the test (target) environment.
type ReWriter interface {
	RewriteBody(body []byte) []byte
	RewriteHeader(header http.Header) http.Header
}

type SanitizingKeyNameType string

const (
	SanitizingHeaderKeys SanitizingKeyNameType = "header"
	SanitizingBodyKeys   SanitizingKeyNameType = "body"
)

type DefaultReWriter struct {
	keysNameByteList map[SanitizingKeyNameType][][]byte // eg: "header" -> list of keys whose values need to be replaced
}

func NewDefaultReWriter(keysNameList map[SanitizingKeyNameType][]string) *DefaultReWriter {
	if len(keysNameList) == 0 {
		return &DefaultReWriter{
			keysNameByteList: make(map[SanitizingKeyNameType][][]byte, len(keysNameList)),
		}
	}

	keysNameStrToByteList := make(map[SanitizingKeyNameType][][]byte, len(keysNameList))
	for key, keyList := range keysNameList {
		for _, element := range keyList {
			keysNameStrToByteList[key] = append(keysNameStrToByteList[key], []byte(element))
		}
	}

	return &DefaultReWriter{
		keysNameByteList: keysNameStrToByteList,
	}
}

// RewriteBody searches for structural JSON keys and replaces their subsequent
// dynamic values with the "<mv>" constant mask.
func (drw *DefaultReWriter) RewriteBody(body []byte) []byte {
	log.Printf("Before RewriteBody %s: ", string(body))
	if len(body) == 0 || len(drw.keysNameByteList[SanitizingBodyKeys]) == 0 {
		return body
	}

	replacementToken := []byte("<mv>")

	// Create a buffer builder to avoid complex in-place pointer math
	// which is highly susceptible to the out-of-bounds issues
	var output []byte

	for _, key := range drw.keysNameByteList[SanitizingBodyKeys] {
		searchToken := append([]byte{'"'}, append(key, '"', ':')...)

		lastPos := 0
		for {
			// Find occurrence
			keyIdx := bytes.Index(body[lastPos:], searchToken)
			if keyIdx == -1 {
				break
			}

			// Absolute index in the body
			absKeyIdx := lastPos + keyIdx
			valStartIdx := absKeyIdx + len(searchToken)

			// Skip whitespace/quotes
			for valStartIdx < len(body) && (body[valStartIdx] == ' ' || body[valStartIdx] == '\t' || body[valStartIdx] == '"') {
				valStartIdx++
			}

			// Find end of value
			valEndIdx := valStartIdx
			for valEndIdx < len(body) && body[valEndIdx] != '"' && body[valEndIdx] != ',' && body[valEndIdx] != '}' && body[valEndIdx] != '\r' && body[valEndIdx] != '\n' {
				valEndIdx++
			}

			// SAFETY GUARD: If we didn't find a valid end or indices are inverted, break
			if valEndIdx < valStartIdx {
				lastPos = absKeyIdx + len(searchToken)
				continue
			}

			// Append everything up to the value, then the token
			output = append(output, body[lastPos:valStartIdx]...)
			output = append(output, replacementToken...)

			// Move the pointer past the original value
			lastPos = valEndIdx
		}
		// Append remainder
		output = append(output, body[lastPos:]...)
		body = output
		output = nil // Reset for next key iteration
	}
	log.Printf("After RewriteBody %s: ", string(body))
	return body
}

// RewriteHeader iterates through the provided http.Header map and updates
// values matching the specified keys to the replacement token.
func (drw *DefaultReWriter) RewriteHeader(header http.Header) http.Header {
	if len(header) == 0 || len(drw.keysNameByteList[SanitizingHeaderKeys]) == 0 {
		return header
	}

	replacementToken := "<mv>"

	// Iterate over the keys we want to rewrite
	for _, keyByte := range drw.keysNameByteList[SanitizingHeaderKeys] {
		// Convert byte key to string (standard library handles canonical capitalization automatically,
		// but using http.CanonicalHeaderKey can enforce it if needed)
		keyStr := string(keyByte)

		// Check if the header map contains this target key
		if values, exists := header[keyStr]; exists {
			// Replace all values associated with this header key
			for i := range values {
				values[i] = replacementToken
			}
		}
	}

	return header
}

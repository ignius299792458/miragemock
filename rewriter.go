package miragemock

import (
	"bytes"
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
	if len(body) == 0 || len(drw.keysNameByteList) == 0 {
		return body
	}

	replacementToken := []byte("<mv>")

	// Iterate through each structural key configured by the programmer
	for _, key := range drw.keysNameByteList[SanitizingBodyKeys] {
		// Formulate a structured JSON key search window token: e.g., "user_id":
		searchToken := make([]byte, 0, len(key)+4)
		searchToken = append(searchToken, '"')
		searchToken = append(searchToken, key...)
		searchToken = append(searchToken, '"', ':')

		for {
			// Find the location of the key structure inside the raw byte slice
			keyIdx := bytes.Index(body, searchToken)
			if keyIdx == -1 {
				break // Key not present in this body slice; move to next configured key
			}

			// Compute the starting offset position of the actual dynamic value string
			valStartIdx := keyIdx + len(searchToken)

			// Fast-forward past any empty whitespace paddings to locate the opening quote
			for valStartIdx < len(body) && (body[valStartIdx] == ' ' || body[valStartIdx] == '\t' || body[valStartIdx] == '"') {
				valStartIdx++
			}

			// Locate the closing quote bounding the dynamic string value
			valEndIdx := valStartIdx
			for valEndIdx < len(body) && body[valEndIdx] != '"' && body[valEndIdx] != ',' && body[valEndIdx] != '}' && body[valEndIdx] != '\r' && body[valEndIdx] != '\n' {
				valEndIdx++
			}

			// Isolate target value metrics to perform length-shifting structural operations
			targetValLen := valEndIdx - valStartIdx
			delta := len(replacementToken) - targetValLen
			newLen := len(body) + delta

			// Memory Shift Path A: Structural replacement fits within buffer boundaries
			if newLen <= cap(body) {
				body = body[:newLen]
				copy(body[valStartIdx+len(replacementToken):], body[valEndIdx:])
				copy(body[valStartIdx:valStartIdx+len(replacementToken)], replacementToken)
				continue
			}

			// Memory Shift Path B: Allocate fallback buffer if length exceeds memory frame capacity
			out := make([]byte, newLen)
			copy(out[:valStartIdx], body[:valStartIdx])
			copy(out[valStartIdx:valStartIdx+len(replacementToken)], replacementToken)
			copy(out[valStartIdx+len(replacementToken):], body[valEndIdx:])
			body = out
		}
	}

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

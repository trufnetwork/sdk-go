package util

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/pkg/errors"
)

// GenerateStreamId is the hash fn to generate a stream id from a string
// it already prepends the "st" prefix to the hash, and returns the first 30 characters, to fit kwil's limit
func GenerateStreamId(s string) StreamId {
	// if the string is already a valid stream id, return it
	if len(s) == 32 && s[:2] == "st" {
		return StreamId{id: s}
	}

	hasher := sha256.New()
	hasher.Write([]byte(s))
	hashBytes := hasher.Sum(nil)
	hashString := hex.EncodeToString(hashBytes)

	streamIdStr := "st" + hashString[:30]

	streamId, _ := NewStreamId(streamIdStr)
	return *streamId
}

type StreamId struct {
	id string
}

func NewStreamId(s string) (*StreamId, error) {
	id := StreamId{
		id: s,
	}

	if err := id.Validate(); err != nil {
		return nil, errors.WithStack(err)
	}

	return &id, nil
}

func NewRawStreamId(s string) *StreamId {
	return &StreamId{id: s}
}

func (s *StreamId) Validate() error {
	// verify if the string is a valid stream id
	if len(s.id) != 32 || s.id[:2] != "st" {
		return errors.New(fmt.Sprintf("invalid stream id '%s'", s))
	}
	return nil
}

func (s *StreamId) String() string {
	return s.id
}

func (s *StreamId) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf("\"%s\"", s.id)), nil
}

func (s *StreamId) UnmarshalJSON(b []byte) error {
	// remove quotes if they exist
	if b[0] == '"' && b[len(b)-1] == '"' {
		s.id = string(b[1 : len(b)-1])
	} else {
		s.id = string(b)
	}

	return nil
}

type StreamIdSlice []StreamId

func (s StreamIdSlice) Strings() []string {
	strs := make([]string, len(s))
	for i, streamId := range s {
		strs[i] = streamId.String()
	}
	return strs
}

func (s StreamIdSlice) Len() int           { return len(s) }
func (s StreamIdSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s StreamIdSlice) Less(i, j int) bool { return s[i].String() < s[j].String() }

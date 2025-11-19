package types

import (
	"errors"
	"io"
)

var errStreamUninitialized = errors.New("types.Stream: next function not configured")

// Stream provides iterator-style access to streaming chat completion chunks.
// It mirrors common Go iterators such as sql.Rows or bufio.Scanner to keep the
// consumption pattern familiar.
type Stream struct {
	current *StreamChunk
	err     error
	closer  io.Closer
	next    func() (*StreamChunk, error)
}

// NewStream constructs a Stream backed by the supplied next function and
// optional closer. Callers should always Close the returned stream when they
// are done consuming data.
func NewStream(next func() (*StreamChunk, error), closer io.Closer) *Stream {
	return &Stream{
		next:   next,
		closer: closer,
	}
}

// Next advances to the next chunk in the stream. It returns false when the
// stream has finished or an error is encountered. After Next returns false,
// inspect Err for the terminal error (if any).
func (s *Stream) Next() bool {
	if s == nil || s.err != nil {
		return false
	}
	if s.next == nil {
		s.err = errStreamUninitialized
		return false
	}

	chunk, err := s.next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return false
		}
		s.err = err
		return false
	}
	if chunk == nil {
		return false
	}

	s.current = chunk
	return true
}

// Chunk returns the chunk produced by the most recent successful call to Next.
func (s *Stream) Chunk() *StreamChunk {
	if s == nil {
		return nil
	}
	return s.current
}

// Err reports the first error encountered by the iterator, if any.
func (s *Stream) Err() error {
	if s == nil {
		return nil
	}
	return s.err
}

// Close releases the underlying streaming resources.
func (s *Stream) Close() error {
	if s == nil {
		return nil
	}
	if s.closer != nil {
		return s.closer.Close()
	}
	return nil
}

// StreamChunk represents a single incremental update from the provider.
type StreamChunk struct {
	ID      string
	Created int64
	Model   string
	Choices []StreamChoice
	Usage   *Usage
}

// StreamChoice holds incremental content for one choice index.
type StreamChoice struct {
	Index        int
	Delta        *MessageDelta
	FinishReason string
}

// MessageDelta captures the incremental fields emitted for a message.
type MessageDelta struct {
	Role      Role
	Content   string
	ToolCalls []ToolCallDelta
	Refusal   string
}

// ToolCallDelta represents partial tool call information for a choice.
type ToolCallDelta struct {
	Index        int
	ID           string
	FunctionName string
	Arguments    string
}

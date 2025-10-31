package types

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// MessageAccumulator incrementally builds a full Message from streaming deltas.
// It is safe for single-goroutine use and intended to be reset or recreated
// per streaming choice.
type MessageAccumulator struct {
	role      Role
	content   strings.Builder
	refusal   strings.Builder
	toolCalls map[int]*toolCallAccumulator
}

type toolCallAccumulator struct {
	id        string
	name      string
	arguments strings.Builder
}

// NewMessageAccumulator constructs a fresh accumulator instance.
func NewMessageAccumulator() *MessageAccumulator {
	return &MessageAccumulator{
		toolCalls: make(map[int]*toolCallAccumulator),
	}
}

// Update merges the supplied delta into the accumulator.
func (ma *MessageAccumulator) Update(delta *MessageDelta) {
	if delta == nil {
		return
	}

	if delta.Role != "" {
		ma.role = delta.Role
	}
	if delta.Content != "" {
		ma.content.WriteString(delta.Content)
	}
	if delta.Refusal != "" {
		ma.refusal.WriteString(delta.Refusal)
	}

	for _, callDelta := range delta.ToolCalls {
		if callDelta == nil {
			continue
		}

		tc := ma.toolCalls[callDelta.Index]
		if tc == nil {
			tc = &toolCallAccumulator{}
			ma.toolCalls[callDelta.Index] = tc
		}

		if callDelta.ID != "" {
			tc.id = callDelta.ID
		}
		if callDelta.FunctionName != "" {
			tc.name = callDelta.FunctionName
		}
		if callDelta.Arguments != "" {
			tc.arguments.WriteString(callDelta.Arguments)
		}
	}
}

// Message materialises the accumulated content into a Message. It returns an
// error when tool call JSON arguments cannot be parsed.
func (ma *MessageAccumulator) Message() (*Message, error) {
	msg := &Message{
		Role:        ma.role,
		ContentPart: make([]ContentPart, 0),
	}

	if ma.content.Len() > 0 {
		msg.ContentPart = append(msg.ContentPart, NewContentPartText(ma.content.String()))
	}

	if ma.refusal.Len() > 0 {
		msg.ContentPart = append(msg.ContentPart, NewContentPartRefusal(ma.refusal.String()))
	}

	if len(ma.toolCalls) > 0 {
		indexes := make([]int, 0, len(ma.toolCalls))
		for idx := range ma.toolCalls {
			indexes = append(indexes, idx)
		}
		sort.Ints(indexes)

		msg.ToolCalls = make([]*ToolCall, 0, len(indexes))
		for _, idx := range indexes {
			tc := ma.toolCalls[idx]
			if tc == nil {
				continue
			}

			argsMap := map[string]any{}
			rawArgs := strings.TrimSpace(tc.arguments.String())
			if rawArgs != "" {
				if err := json.Unmarshal([]byte(rawArgs), &argsMap); err != nil {
					return nil, fmt.Errorf("parse tool call %d arguments: %w", idx, err)
				}
			}

			msg.ToolCalls = append(msg.ToolCalls, &ToolCall{
				ID: tc.id,
				Function: ToolFunction{
					Name:      tc.name,
					Arguments: argsMap,
				},
			})
		}
	}

	return msg, nil
}

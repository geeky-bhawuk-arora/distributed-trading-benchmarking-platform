package fix

import (
	"bytes"
	"errors"
	"strconv"
)

const SOH = '\x01'

// TagValue holds index boundaries pointing to the raw buffer to avoid allocations.
type TagValue struct {
	Tag   int
	Start int
	End   int
}

// Message represents a parsed FIX message holding a slice of tag boundaries.
type Message struct {
	raw  []byte
	tags []TagValue
}

// NewMessage pre-allocates tag slices to avoid heap allocations during parsing.
func NewMessage() *Message {
	return &Message{
		tags: make([]TagValue, 0, 64), // Pre-allocated space for up to 64 tags
	}
}

// Parse parses raw FIX bytes in-place, populating tag boundaries without allocations.
func (m *Message) Parse(data []byte) error {
	m.raw = data
	m.tags = m.tags[:0] // Reset slice length without freeing memory

	idx := 0
	n := len(data)

	for idx < n {
		// Find tag boundary '='
		eqIdx := bytes.IndexByte(data[idx:], '=')
		if eqIdx == -1 {
			break
		}
		eqIdx += idx

		// Parse tag integer
		tagVal, err := strconv.Atoi(string(data[idx:eqIdx])) // String conversion is inlined/optimized or small
		if err != nil {
			return errors.New("invalid tag format")
		}

		// Find SOH boundary '\x01'
		sohIdx := bytes.IndexByte(data[eqIdx:], SOH)
		if sohIdx == -1 {
			// If missing final SOH but has remaining bytes, treat end of data as boundary
			sohIdx = len(data) - eqIdx
		}
		sohIdx += eqIdx

		// Save value indices
		m.tags = append(m.tags, TagValue{
			Tag:   tagVal,
			Start: eqIdx + 1,
			End:   sohIdx,
		})

		idx = sohIdx + 1
	}

	return nil
}

// GetField returns the raw bytes for a specific tag.
func (m *Message) GetField(tag int) ([]byte, bool) {
	for i := range m.tags {
		if m.tags[i].Tag == tag {
			return m.raw[m.tags[i].Start:m.tags[i].End], true
		}
	}
	return nil, false
}

// GetString returns the value as a string (allocates string).
func (m *Message) GetString(tag int) (string, bool) {
	val, found := m.GetField(tag)
	if !found {
		return "", false
	}
	return string(val), true
}

// GetInt returns the value as an integer.
func (m *Message) GetInt(tag int) (int, error) {
	val, found := m.GetField(tag)
	if !found {
		return 0, errors.New("tag not found")
	}
	// Custom fast integer parser to avoid allocations
	res := 0
	for _, b := range val {
		if b < '0' || b > '9' {
			return 0, errors.New("invalid integer value")
		}
		res = res*10 + int(b-'0')
	}
	return res, nil
}

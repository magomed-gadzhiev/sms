package shared

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectEncoding(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected SMSEncoding
	}{
		{
			name:     "empty string is GSM7",
			text:     "",
			expected: EncodingGSM7,
		},
		{
			name:     "pure ASCII letters",
			text:     "Hello World",
			expected: EncodingGSM7,
		},
		{
			name:     "digits and punctuation",
			text:     "12345 !@#%&",
			expected: EncodingGSM7,
		},
		{
			name:     "GSM7 special chars",
			text:     "£$¥èéùìòÇØø",
			expected: EncodingGSM7,
		},
		{
			name:     "GSM7 extended chars",
			text:     "^{}[]~|\\€",
			expected: EncodingGSM7,
		},
		{
			name:     "Cyrillic requires UCS2",
			text:     "Привет мир",
			expected: EncodingUCS2,
		},
		{
			name:     "mixed ASCII and Cyrillic requires UCS2",
			text:     "Hello Привет",
			expected: EncodingUCS2,
		},
		{
			name:     "emoji requires UCS2",
			text:     "Hello 😀",
			expected: EncodingUCS2,
		},
		{
			name:     "Chinese characters require UCS2",
			text:     "你好",
			expected: EncodingUCS2,
		},
		{
			name:     "Arabic requires UCS2",
			text:     "مرحبا",
			expected: EncodingUCS2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetectEncoding(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSegments(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected int
	}{
		{
			name:     "empty string is 1 segment",
			text:     "",
			expected: 1,
		},
		{
			name:     "single GSM7 char",
			text:     "A",
			expected: 1,
		},
		{
			name:     "exactly 160 GSM7 chars is 1 segment",
			text:     strings.Repeat("A", 160),
			expected: 1,
		},
		{
			name:     "161 GSM7 chars is 2 segments",
			text:     strings.Repeat("A", 161),
			expected: 2,
		},
		{
			name:     "306 GSM7 chars is 2 segments (153*2)",
			text:     strings.Repeat("A", 306),
			expected: 2,
		},
		{
			name:     "307 GSM7 chars is 3 segments",
			text:     strings.Repeat("A", 307),
			expected: 3,
		},
		{
			name:     "single UCS2 char is 1 segment",
			text:     "Привет",
			expected: 1,
		},
		{
			name:     "exactly 70 UCS2 chars is 1 segment",
			text:     strings.Repeat("А", 70),
			expected: 1,
		},
		{
			name:     "71 UCS2 chars is 2 segments",
			text:     strings.Repeat("А", 71),
			expected: 2,
		},
		{
			name:     "134 UCS2 chars is 2 segments (67*2)",
			text:     strings.Repeat("А", 134),
			expected: 2,
		},
		{
			name:     "135 UCS2 chars is 3 segments",
			text:     strings.Repeat("А", 135),
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CountSegments(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCountSegmentsExtendedGSM7(t *testing.T) {
	t.Run("extended chars count as 2", func(t *testing.T) {
		// Each '{' is extended and counts as 2.
		// 80 '{' chars = 160 GSM7 units → still 1 segment
		text := strings.Repeat("{", 80)
		assert.Equal(t, 1, CountSegments(text))
	})

	t.Run("81 extended chars exceed single segment", func(t *testing.T) {
		// 81 * 2 = 162 > 160 → 2 segments
		text := strings.Repeat("{", 81)
		assert.Equal(t, 2, CountSegments(text))
	})

	t.Run("mixed normal and extended chars", func(t *testing.T) {
		// 159 normal + 1 extended = 159 + 2 = 161 → 2 segments
		text := strings.Repeat("A", 159) + "{"
		assert.Equal(t, 2, CountSegments(text))
	})
}

func TestSplitMessage(t *testing.T) {
	t.Run("single GSM7 segment has no UDH", func(t *testing.T) {
		segments := SplitMessage("Hello")
		require.Len(t, segments, 1)
		assert.Equal(t, "Hello", segments[0].Text)
		assert.Nil(t, segments[0].UDH)
		assert.Equal(t, 1, segments[0].PartIndex)
		assert.Equal(t, 1, segments[0].TotalParts)
	})

	t.Run("single UCS2 segment has no UDH", func(t *testing.T) {
		segments := SplitMessage("Привет")
		require.Len(t, segments, 1)
		assert.Nil(t, segments[0].UDH)
		assert.Equal(t, 1, segments[0].TotalParts)
	})

	t.Run("multipart GSM7 has UDH on each part", func(t *testing.T) {
		text := strings.Repeat("A", 200)
		segments := SplitMessage(text)
		require.Len(t, segments, 2)

		for i, seg := range segments {
			require.NotNil(t, seg.UDH, "segment %d should have UDH", i+1)
			assert.Len(t, seg.UDH, 6, "UDH must be 6 bytes")
			assert.Equal(t, byte(0x05), seg.UDH[0], "UDH[0] should be 0x05 (length)")
			assert.Equal(t, byte(0x00), seg.UDH[1], "UDH[1] should be 0x00 (IE identifier)")
			assert.Equal(t, byte(0x03), seg.UDH[2], "UDH[2] should be 0x03 (IE data length)")
			assert.Equal(t, byte(2), seg.UDH[4], "UDH[4] should be total parts")
			assert.Equal(t, byte(i+1), seg.UDH[5], "UDH[5] should be part number")
			assert.Equal(t, i+1, seg.PartIndex)
			assert.Equal(t, 2, seg.TotalParts)
		}
	})

	t.Run("multipart UCS2 has UDH on each part", func(t *testing.T) {
		text := strings.Repeat("А", 80) // Cyrillic А
		segments := SplitMessage(text)
		require.Len(t, segments, 2)

		for i, seg := range segments {
			require.NotNil(t, seg.UDH)
			assert.Len(t, seg.UDH, 6)
			assert.Equal(t, i+1, seg.PartIndex)
			assert.Equal(t, 2, seg.TotalParts)
		}
	})

	t.Run("all parts have the same reference number", func(t *testing.T) {
		text := strings.Repeat("A", 300)
		segments := SplitMessage(text)
		require.True(t, len(segments) >= 2)

		refNum := segments[0].RefNumber
		for _, seg := range segments[1:] {
			assert.Equal(t, refNum, seg.RefNumber, "all parts must share the same reference number")
			assert.Equal(t, refNum, seg.UDH[3], "UDH[3] must match reference number")
		}
	})

	t.Run("concatenated GSM7 parts reconstruct original text", func(t *testing.T) {
		text := strings.Repeat("Hello ", 40) // 240 chars → 2 segments
		segments := SplitMessage(text)
		var reconstructed strings.Builder
		for _, seg := range segments {
			reconstructed.WriteString(seg.Text)
		}
		assert.Equal(t, text, reconstructed.String())
	})

	t.Run("concatenated UCS2 parts reconstruct original text", func(t *testing.T) {
		text := strings.Repeat("Привет ", 20) // 140 chars → 3 segments
		segments := SplitMessage(text)
		var reconstructed strings.Builder
		for _, seg := range segments {
			reconstructed.WriteString(seg.Text)
		}
		assert.Equal(t, text, reconstructed.String())
	})

	t.Run("GSM7 multipart segment length does not exceed MultipartSMSMaxGSM7", func(t *testing.T) {
		text := strings.Repeat("A", 400)
		segments := SplitMessage(text)
		for i, seg := range segments {
			length := len([]rune(seg.Text))
			assert.LessOrEqual(t, length, MultipartSMSMaxGSM7,
				"segment %d has %d chars, exceeds max %d", i+1, length, MultipartSMSMaxGSM7)
		}
	})

	t.Run("UCS2 multipart segment length does not exceed MultipartSMSMaxUCS2", func(t *testing.T) {
		text := strings.Repeat("А", 200)
		segments := SplitMessage(text)
		for i, seg := range segments {
			length := len([]rune(seg.Text))
			assert.LessOrEqual(t, length, MultipartSMSMaxUCS2,
				"segment %d has %d chars, exceeds max %d", i+1, length, MultipartSMSMaxUCS2)
		}
	})

	t.Run("extended GSM7 chars split correctly across segments", func(t *testing.T) {
		// 77 normal + 1 extended + 77 normal = 77 + 2 + 77 = 156 + fills first segment (<=153 only 77+1=79 units)
		// Build a text whose first 153 GSM7 units are 152 'A' + 1 '{' (154 units total for 153 A's, so use 76 A + 1 '{' = 78 units, then fill rest)
		// Simpler: 76 A + 1 { = 76 + 2 = 78 units; 75 A + 1 { = 77 units; build crossing boundary
		// Just use enough extended chars to force a split mid-extended-sequence is handled
		text := strings.Repeat("{", 77) + strings.Repeat("A", 80)
		// 77 * 2 = 154 units in first segment attempt, but max is 153 per segment
		// So first segment fits 76 '{' = 152 units, then A's go to next segment
		segments := SplitMessage(text)
		assert.True(t, len(segments) >= 2)
		// Reconstruct and verify no data loss
		var reconstructed strings.Builder
		for _, seg := range segments {
			reconstructed.WriteString(seg.Text)
		}
		assert.Equal(t, text, reconstructed.String())
	})
}

func TestDataCodingForEncoding(t *testing.T) {
	t.Run("GSM7 returns 0x00", func(t *testing.T) {
		assert.Equal(t, byte(0x00), DataCodingForEncoding(EncodingGSM7))
	})

	t.Run("UCS2 returns 0x08", func(t *testing.T) {
		assert.Equal(t, byte(0x08), DataCodingForEncoding(EncodingUCS2))
	})
}

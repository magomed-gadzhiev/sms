package shared

import (
	"math/rand"
	"unicode/utf8"
)

// GSM 7-bit default alphabet characters
const gsm7Chars = "@£$¥èéùìòÇ\nØø\rÅåΔ_ΦΓΛΩΠΨΣΘΞ ÆæßÉ !\"#¤%&'()*+,-./0123456789:;<=>?¡ABCDEFGHIJKLMNOPQRSTUVWXYZÄÖÑÜ§¿abcdefghijklmnopqrstuvwxyzäöñüà"

// GSM 7-bit extended characters (requires escape, counts as 2 chars)
const gsm7Extended = "^{}\\[~]|€"

const (
	// SingleSMSMaxGSM7 is the max chars for a single GSM 7-bit SMS
	SingleSMSMaxGSM7 = 160
	// MultipartSMSMaxGSM7 is the max chars per segment in multipart GSM 7-bit SMS (UDH takes 7 chars)
	MultipartSMSMaxGSM7 = 153
	// SingleSMSMaxUCS2 is the max chars for a single UCS-2 SMS
	SingleSMSMaxUCS2 = 70
	// MultipartSMSMaxUCS2 is the max chars per segment in multipart UCS-2 SMS (UDH takes 3 chars)
	MultipartSMSMaxUCS2 = 67
	// MaxSegments is the maximum number of segments allowed
	MaxSegments = 10

	// UDH length for concatenated SMS
	UDHLength = 6 // 05 00 03 XX MM PP
)

// SMSEncoding represents the encoding type of an SMS
type SMSEncoding int

const (
	EncodingGSM7 SMSEncoding = iota
	EncodingUCS2
)

// DetectEncoding determines whether text can be encoded as GSM 7-bit or requires UCS-2
func DetectEncoding(text string) SMSEncoding {
	for _, r := range text {
		if !isGSM7Char(r) {
			return EncodingUCS2
		}
	}
	return EncodingGSM7
}

// isGSM7Char checks if a rune is in the GSM 7-bit character set
func isGSM7Char(r rune) bool {
	for _, c := range gsm7Chars {
		if r == c {
			return true
		}
	}
	for _, c := range gsm7Extended {
		if r == c {
			return true
		}
	}
	return false
}

// gsm7Length calculates the GSM 7-bit length of text (extended chars count as 2)
func gsm7Length(text string) int {
	length := 0
	for _, r := range text {
		for _, c := range gsm7Extended {
			if r == c {
				length++ // extra count for escape char
				break
			}
		}
		length++
	}
	return length
}

// CountSegments returns the number of SMS segments needed for the given text
func CountSegments(text string) int {
	if text == "" {
		return 1
	}

	encoding := DetectEncoding(text)

	if encoding == EncodingGSM7 {
		charCount := gsm7Length(text)
		if charCount <= SingleSMSMaxGSM7 {
			return 1
		}
		segments := charCount / MultipartSMSMaxGSM7
		if charCount%MultipartSMSMaxGSM7 > 0 {
			segments++
		}
		return segments
	}

	// UCS-2
	charCount := utf8.RuneCountInString(text)
	if charCount <= SingleSMSMaxUCS2 {
		return 1
	}
	segments := charCount / MultipartSMSMaxUCS2
	if charCount%MultipartSMSMaxUCS2 > 0 {
		segments++
	}
	return segments
}

// SMSSegment represents a single segment of a multipart SMS
type SMSSegment struct {
	Text      string
	UDH       []byte // User Data Header (nil for single-part SMS)
	PartIndex int    // 1-based part number
	TotalParts int
	RefNumber  byte  // Concatenation reference number
}

// SplitMessage splits text into SMS segments with UDH headers for multipart messages
func SplitMessage(text string) []SMSSegment {
	segmentCount := CountSegments(text)

	if segmentCount == 1 {
		return []SMSSegment{
			{
				Text:       text,
				UDH:        nil,
				PartIndex:  1,
				TotalParts: 1,
			},
		}
	}

	refNumber := byte(rand.Intn(256))
	encoding := DetectEncoding(text)

	var segments []SMSSegment
	runes := []rune(text)
	offset := 0

	for i := 0; i < segmentCount && offset < len(runes); i++ {
		var maxChars int
		if encoding == EncodingGSM7 {
			maxChars = MultipartSMSMaxGSM7
		} else {
			maxChars = MultipartSMSMaxUCS2
		}

		end := offset + maxChars
		if end > len(runes) {
			end = len(runes)
		}

		// For GSM7, adjust end to account for extended chars counting as 2
		if encoding == EncodingGSM7 {
			charCount := 0
			actualEnd := offset
			for actualEnd < len(runes) && charCount < maxChars {
				isExtended := false
				for _, c := range gsm7Extended {
					if runes[actualEnd] == c {
						isExtended = true
						break
					}
				}
				if isExtended {
					if charCount+2 > maxChars {
						break
					}
					charCount += 2
				} else {
					charCount++
				}
				actualEnd++
			}
			end = actualEnd
		}

		partNum := byte(i + 1)
		udh := []byte{
			0x05,             // UDH length
			0x00,             // Concatenated short messages, 8-bit reference number
			0x03,             // IE data length
			refNumber,        // Reference number
			byte(segmentCount), // Total parts
			partNum,          // Part number
		}

		segments = append(segments, SMSSegment{
			Text:       string(runes[offset:end]),
			UDH:        udh,
			PartIndex:  int(partNum),
			TotalParts: segmentCount,
			RefNumber:  refNumber,
		})

		offset = end
	}

	return segments
}

// DataCodingForEncoding returns the SMPP data_coding value for the given encoding
func DataCodingForEncoding(encoding SMSEncoding) byte {
	if encoding == EncodingUCS2 {
		return 0x08 // UCS2
	}
	return 0x00 // GSM 7-bit default
}

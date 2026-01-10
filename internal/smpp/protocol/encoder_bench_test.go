//go:build !integration

package protocol

import (
	"testing"
)

func BenchmarkEncodePDU(b *testing.B) {
	encoder := NewEncoder()
	pdu := &PDU{
		CommandLength:  PDUHeaderLength,
		CommandID:      EnquireLink,
		CommandStatus:  ESME_ROK,
		SequenceNumber: 1,
		Body:          []byte{},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = encoder.EncodePDU(pdu)
	}
}

func BenchmarkEncodeSubmitSM(b *testing.B) {
	encoder := NewEncoder()
	submit := &SubmitSMPDU{
		SourceAddr:      "12345",
		DestinationAddr: "79001234567",
		SMLength:        11,
		ShortMessage:    []byte("Hello World"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = encoder.EncodeSubmitSM(submit)
	}
}

func BenchmarkEncodeBind(b *testing.B) {
	encoder := NewEncoder()
	bind := &BindPDU{
		SystemID:         "test-system",
		Password:         "test-pass",
		SystemType:       "test-type",
		InterfaceVersion: Version,
		AddrTON:          TON_INTERNATIONAL,
		AddrNPI:          NPI_ISDN,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = encoder.EncodeBind(bind)
	}
}

func BenchmarkDecodePDU(b *testing.B) {
	encoder := NewEncoder()
	pdu := &PDU{
		CommandLength:  PDUHeaderLength,
		CommandID:      EnquireLink,
		CommandStatus:  ESME_ROK,
		SequenceNumber: 1,
		Body:          []byte{},
	}

	data, _ := encoder.EncodePDU(pdu)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := NewDecoder(data)
		_, _ = decoder.DecodePDU()
	}
}

func BenchmarkDecodeSubmitSM(b *testing.B) {
	encoder := NewEncoder()
	submit := &SubmitSMPDU{
		SourceAddr:      "12345",
		DestinationAddr: "79001234567",
		SMLength:        11,
		ShortMessage:    []byte("Hello World"),
	}

	body, _ := encoder.EncodeSubmitSM(submit)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		decoder := NewDecoder(body)
		_, _ = decoder.DecodeSubmitSM(body)
	}
}

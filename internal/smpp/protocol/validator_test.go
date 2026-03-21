package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidator(t *testing.T) {
	t.Run("ValidatePDU", func(t *testing.T) {
		validator := NewValidator()

		tests := []struct {
			name    string
			pdu     *PDU
			wantErr bool
		}{
			{
				name: "valid PDU",
				pdu: &PDU{
					CommandLength:  PDUHeaderLength,
					CommandID:      EnquireLink,
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
					Body:          []byte{},
				},
				wantErr: false,
			},
			{
				name: "invalid command length",
				pdu: &PDU{
					CommandLength:  PDUHeaderLength - 1,
					CommandID:      EnquireLink,
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
				},
				wantErr: true,
			},
			{
				name: "mismatched command length",
				pdu: &PDU{
					CommandLength:  PDUHeaderLength + 10,
					CommandID:      EnquireLink,
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
					Body:          []byte{1, 2, 3}, // только 3 байта
				},
				wantErr: true,
			},
			{
				name: "unknown command ID",
				pdu: &PDU{
					CommandLength:  PDUHeaderLength,
					CommandID:      0xFFFFFFFF, // неизвестная команда
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.ValidatePDU(tt.pdu)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("ValidateBind", func(t *testing.T) {
		validator := NewValidator()

		tests := []struct {
			name    string
			bind    *BindPDU
			wantErr bool
		}{
			{
				name: "valid bind",
				bind: &BindPDU{
					SystemID:         "test-system",
					Password:         "test-pass",
					SystemType:       "test-type",
					InterfaceVersion: Version,
					AddrTON:          TON_INTERNATIONAL,
					AddrNPI:          NPI_ISDN,
				},
				wantErr: false,
			},
			{
				name: "empty system ID",
				bind: &BindPDU{
					SystemID: "",
				},
				wantErr: true,
			},
			{
				name: "system ID too long",
				bind: &BindPDU{
					SystemID: string(make([]byte, MaxSystemIDLength+1)),
				},
				wantErr: true,
			},
			{
				name: "invalid interface version",
				bind: &BindPDU{
					SystemID:         "test",
					Password:         "test",
					InterfaceVersion: 0x00, // неверная версия
				},
				wantErr: true,
			},
			{
				name: "invalid addr TON",
				bind: &BindPDU{
					SystemID:         "test",
					Password:         "test",
					InterfaceVersion: Version,
					AddrTON:          99, // неверное значение
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.ValidateBind(tt.bind)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("ValidateSubmitSM", func(t *testing.T) {
		validator := NewValidator()

		tests := []struct {
			name    string
			submit  *SubmitSMPDU
			wantErr bool
		}{
			{
				name: "valid submit_sm",
				submit: &SubmitSMPDU{
					SourceAddr:      "12345",
					DestinationAddr: "79001234567",
					SMLength:        11,
					ShortMessage:    []byte("Hello World"),
				},
				wantErr: false,
			},
			{
				name: "empty source addr",
				submit: &SubmitSMPDU{
					DestinationAddr: "79001234567",
				},
				wantErr: true,
			},
			{
				name: "mismatched SM length",
				submit: &SubmitSMPDU{
					SourceAddr:      "12345",
					DestinationAddr: "79001234567",
					SMLength:        5,
					ShortMessage:    []byte("Hello World"), // 11 bytes
				},
				wantErr: true,
			},
			{
				name: "invalid priority flag",
				submit: &SubmitSMPDU{
					SourceAddr:      "12345",
					DestinationAddr: "79001234567",
					PriorityFlag:    99, // неверное значение
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := validator.ValidateSubmitSM(tt.submit)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

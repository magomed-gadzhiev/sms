package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncoder(t *testing.T) {
	t.Run("EncodePDU", func(t *testing.T) {
		encoder := NewEncoder()

		tests := []struct {
			name    string
			pdu     *PDU
			wantErr bool
		}{
			{
				name: "valid PDU with body",
				pdu: &PDU{
					CommandLength:  20,
					CommandID:      BindTransceiver,
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
					Body:          []byte{1, 2, 3, 4},
				},
				wantErr: false,
			},
			{
				name: "valid PDU without body",
				pdu: &PDU{
					CommandLength:  PDUHeaderLength,
					CommandID:      EnquireLink,
					CommandStatus:  ESME_ROK,
					SequenceNumber: 1,
					Body:          []byte{},
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := encoder.EncodePDU(tt.pdu)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.NotEmpty(t, data)
					// Проверяем, что command_length был обновлен
					assert.GreaterOrEqual(t, len(data), int(PDUHeaderLength))
				}
			})
		}
	})

	t.Run("EncodeBind", func(t *testing.T) {
		encoder := NewEncoder()

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
					AddressRange:     "",
				},
				wantErr: false,
			},
			{
				name: "system ID too long",
				bind: &BindPDU{
					SystemID: string(make([]byte, MaxSystemIDLength+1)),
				},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := encoder.EncodeBind(tt.bind)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.NotEmpty(t, data)
				}
			})
		}
	})

	t.Run("EncodeSubmitSM", func(t *testing.T) {
		encoder := NewEncoder()

		tests := []struct {
			name    string
			submit  *SubmitSMPDU
			wantErr bool
		}{
			{
				name: "valid submit_sm",
				submit: &SubmitSMPDU{
					ServiceType:          "",
					SourceAddrTON:        TON_INTERNATIONAL,
					SourceAddrNPI:        NPI_ISDN,
					SourceAddr:           "12345",
					DestAddrTON:          TON_INTERNATIONAL,
					DestAddrNPI:          NPI_ISDN,
					DestinationAddr:      "79001234567",
					ESMClass:             0,
					ProtocolID:           0,
					PriorityFlag:         PRIORITY_FLAG_LEVEL_0,
					ScheduleDeliveryTime: "",
					ValidityPeriod:       "",
					RegisteredDelivery:   0,
					ReplaceIfPresent:     0,
					DataCoding:           0,
					SMDefaultMsgID:       0,
					SMLength:             11,
					ShortMessage:         []byte("Hello World"),
				},
				wantErr: false,
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
				name: "with TLV parameters",
				submit: &SubmitSMPDU{
					SourceAddr:      "12345",
					DestinationAddr: "79001234567",
					SMLength:        11,
					ShortMessage:    []byte("Hello World"),
					TLV: map[uint16][]byte{
						0x1400: []byte("test"),
					},
				},
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				data, err := encoder.EncodeSubmitSM(tt.submit)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.NotEmpty(t, data)
				}
			})
		}
	})

	t.Run("EncodeSubmitSMResp", func(t *testing.T) {
		t.Run("encodes submit_sm response", func(t *testing.T) {
			encoder := NewEncoder()

			resp := &SubmitSMRespPDU{
				MessageID: "test-message-id",
			}

			data, err := encoder.EncodeSubmitSMResp(resp)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	})

	t.Run("EncodeDeliverSM", func(t *testing.T) {
		t.Run("encodes deliver_sm PDU", func(t *testing.T) {
			encoder := NewEncoder()

			deliver := &DeliverSMPDU{
				SourceAddr:      "79001234567",
				DestinationAddr: "12345",
				SMLength:        11,
				ShortMessage:    []byte("Hello World"),
			}

			data, err := encoder.EncodeDeliverSM(deliver)
			require.NoError(t, err)
			assert.NotEmpty(t, data)
		})
	})

	t.Run("EncodeEnquireLink", func(t *testing.T) {
		t.Run("encodes enquire_link with empty body", func(t *testing.T) {
			encoder := NewEncoder()

			enquire := &EnquireLinkPDU{}
			data, err := encoder.EncodeEnquireLink(enquire)
			require.NoError(t, err)
			assert.Empty(t, data) // EnquireLink имеет пустое тело
		})
	})

	t.Run("EncodeUnbind", func(t *testing.T) {
		t.Run("encodes unbind with empty body", func(t *testing.T) {
			encoder := NewEncoder()

			unbind := &UnbindPDU{}
			data, err := encoder.EncodeUnbind(unbind)
			require.NoError(t, err)
			assert.Empty(t, data) // Unbind имеет пустое тело
		})
	})

	t.Run("WriteCString", func(t *testing.T) {
		encoder := NewEncoder()

		tests := []struct {
			name    string
			s       string
			maxLen  int
			wantErr bool
		}{
			{
				name:    "valid string",
				s:       "test",
				maxLen:  10,
				wantErr: false,
			},
			{
				name:    "string too long",
				s:       "test",
				maxLen:  2,
				wantErr: true,
			},
			{
				name:    "empty string",
				s:       "",
				maxLen:  10,
				wantErr: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := encoder.writeCString(tt.s, tt.maxLen)

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("WriteTLV", func(t *testing.T) {
		t.Run("writes TLV parameters", func(t *testing.T) {
			encoder := NewEncoder()

			tlv := map[uint16][]byte{
				0x1400: []byte("test-value-1"),
				0x1401: []byte("test-value-2"),
			}

			err := encoder.writeTLV(tlv)
			require.NoError(t, err)
			assert.NotEmpty(t, encoder.buf.Bytes())
		})
	})
}

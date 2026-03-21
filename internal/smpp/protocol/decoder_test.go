package protocol

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecoder(t *testing.T) {
	t.Run("DecodePDU", func(t *testing.T) {
		tests := []struct {
			name    string
			data    []byte
			wantErr bool
		}{
			{
				name: "valid PDU",
				data: func() []byte {
					data := make([]byte, PDUHeaderLength)
					binary.BigEndian.PutUint32(data[0:4], PDUHeaderLength)
					binary.BigEndian.PutUint32(data[4:8], EnquireLink)
					binary.BigEndian.PutUint32(data[8:12], ESME_ROK)
					binary.BigEndian.PutUint32(data[12:16], 1)
					return data
				}(),
				wantErr: false,
			},
			{
				name:    "insufficient data",
				data:    []byte{1, 2, 3},
				wantErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				decoder := NewDecoder(tt.data)
				pdu, err := decoder.DecodePDU()

				if tt.wantErr {
					assert.Error(t, err)
				} else {
					require.NoError(t, err)
					assert.NotNil(t, pdu)
					assert.Equal(t, uint32(EnquireLink), pdu.CommandID)
				}
			})
		}
	})

	t.Run("DecodeBind", func(t *testing.T) {
		t.Run("decodes encoded bind PDU", func(t *testing.T) {
			encoder := NewEncoder()
			bind := &BindPDU{
				SystemID:         "test-system",
				Password:         "test-pass",
				SystemType:       "test-type",
				InterfaceVersion: Version,
				AddrTON:          TON_INTERNATIONAL,
				AddrNPI:          NPI_ISDN,
				AddressRange:     "",
			}

			body, err := encoder.EncodeBind(bind)
			require.NoError(t, err)

			decoder := NewDecoder(body)
			decoded, err := decoder.DecodeBind(body)
			require.NoError(t, err)

			assert.Equal(t, bind.SystemID, decoded.SystemID)
			assert.Equal(t, bind.Password, decoded.Password)
			assert.Equal(t, bind.SystemType, decoded.SystemType)
			assert.Equal(t, bind.InterfaceVersion, decoded.InterfaceVersion)
		})
	})

	t.Run("DecodeSubmitSM", func(t *testing.T) {
		t.Run("decodes encoded submit_sm PDU", func(t *testing.T) {
			encoder := NewEncoder()
			submit := &SubmitSMPDU{
				SourceAddr:      "12345",
				DestinationAddr: "79001234567",
				SMLength:        11,
				ShortMessage:    []byte("Hello World"),
			}

			body, err := encoder.EncodeSubmitSM(submit)
			require.NoError(t, err)

			decoder := NewDecoder(body)
			decoded, err := decoder.DecodeSubmitSM(body)
			require.NoError(t, err)

			assert.Equal(t, submit.SourceAddr, decoded.SourceAddr)
			assert.Equal(t, submit.DestinationAddr, decoded.DestinationAddr)
			assert.Equal(t, submit.ShortMessage, decoded.ShortMessage)
		})
	})

	t.Run("ReadCString", func(t *testing.T) {
		t.Run("reads null-terminated string", func(t *testing.T) {
			encoder := NewEncoder()
			testString := "test-string"
			err := encoder.writeCString(testString, 20)
			require.NoError(t, err)

			decoder := NewDecoder(encoder.buf.Bytes())
			decoded, err := decoder.readCString(20)
			require.NoError(t, err)
			assert.Equal(t, testString, decoded)
		})
	})
}

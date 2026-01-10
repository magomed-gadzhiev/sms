package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// Encoder кодирует PDU структуры в бинарный формат SMPP
type Encoder struct {
	buf *bytes.Buffer
}

// NewEncoder создает новый encoder
func NewEncoder() *Encoder {
	return &Encoder{
		buf: bytes.NewBuffer(nil),
	}
}

// EncodePDU кодирует базовую PDU структуру
func (e *Encoder) EncodePDU(pdu *PDU) ([]byte, error) {
	e.buf.Reset()
	
	// Записываем заголовок
	if err := binary.Write(e.buf, binary.BigEndian, pdu.CommandLength); err != nil {
		return nil, fmt.Errorf("failed to write command_length: %w", err)
	}
	if err := binary.Write(e.buf, binary.BigEndian, pdu.CommandID); err != nil {
		return nil, fmt.Errorf("failed to write command_id: %w", err)
	}
	if err := binary.Write(e.buf, binary.BigEndian, pdu.CommandStatus); err != nil {
		return nil, fmt.Errorf("failed to write command_status: %w", err)
	}
	if err := binary.Write(e.buf, binary.BigEndian, pdu.SequenceNumber); err != nil {
		return nil, fmt.Errorf("failed to write sequence_number: %w", err)
	}
	
	// Записываем тело
	if len(pdu.Body) > 0 {
		if _, err := e.buf.Write(pdu.Body); err != nil {
			return nil, fmt.Errorf("failed to write body: %w", err)
		}
	}
	
	// Обновляем command_length
	result := e.buf.Bytes()
	binary.BigEndian.PutUint32(result[0:4], uint32(len(result)))
	
	return result, nil
}

// EncodeBind кодирует bind PDU
func (e *Encoder) EncodeBind(bind *BindPDU) ([]byte, error) {
	e.buf.Reset()
	
	// System ID (C-Octet String, max 16)
	if err := e.writeCString(bind.SystemID, MaxSystemIDLength); err != nil {
		return nil, fmt.Errorf("failed to write system_id: %w", err)
	}
	
	// Password (C-Octet String, max 9)
	if err := e.writeCString(bind.Password, MaxPasswordLength); err != nil {
		return nil, fmt.Errorf("failed to write password: %w", err)
	}
	
	// System Type (C-Octet String, max 13)
	if err := e.writeCString(bind.SystemType, MaxSystemTypeLength); err != nil {
		return nil, fmt.Errorf("failed to write system_type: %w", err)
	}
	
	// Interface Version
	if err := e.buf.WriteByte(bind.InterfaceVersion); err != nil {
		return nil, fmt.Errorf("failed to write interface_version: %w", err)
	}
	
	// Addr TON
	if err := e.buf.WriteByte(bind.AddrTON); err != nil {
		return nil, fmt.Errorf("failed to write addr_ton: %w", err)
	}
	
	// Addr NPI
	if err := e.buf.WriteByte(bind.AddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write addr_npi: %w", err)
	}
	
	// Address Range (C-Octet String, max 41)
	if err := e.writeCString(bind.AddressRange, 41); err != nil {
		return nil, fmt.Errorf("failed to write address_range: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeBindResp кодирует bind response PDU
func (e *Encoder) EncodeBindResp(resp *BindRespPDU) ([]byte, error) {
	e.buf.Reset()
	
	// System ID (C-Octet String, max 16)
	if err := e.writeCString(resp.SystemID, MaxSystemIDLength); err != nil {
		return nil, fmt.Errorf("failed to write system_id: %w", err)
	}
	
	// Optional Parameters (TLV)
	if resp.TLV != nil && len(resp.TLV) > 0 {
		if err := e.writeTLV(resp.TLV); err != nil {
			return nil, fmt.Errorf("failed to write TLV: %w", err)
		}
	}
	
	return e.buf.Bytes(), nil
}

// EncodeSubmitSM кодирует submit_sm PDU
func (e *Encoder) EncodeSubmitSM(submit *SubmitSMPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Service Type (C-Octet String, max 6)
	if err := e.writeCString(submit.ServiceType, MaxServiceTypeLength); err != nil {
		return nil, fmt.Errorf("failed to write service_type: %w", err)
	}
	
	// Source Addr TON
	if err := e.buf.WriteByte(submit.SourceAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if err := e.buf.WriteByte(submit.SourceAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_npi: %w", err)
	}
	
	// Source Addr (C-Octet String, max 21)
	if err := e.writeCString(submit.SourceAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write source_addr: %w", err)
	}
	
	// Dest Addr TON
	if err := e.buf.WriteByte(submit.DestAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write dest_addr_ton: %w", err)
	}
	
	// Dest Addr NPI
	if err := e.buf.WriteByte(submit.DestAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write dest_addr_npi: %w", err)
	}
	
	// Destination Addr (C-Octet String, max 21)
	if err := e.writeCString(submit.DestinationAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write destination_addr: %w", err)
	}
	
	// ESM Class
	if err := e.buf.WriteByte(submit.ESMClass); err != nil {
		return nil, fmt.Errorf("failed to write esm_class: %w", err)
	}
	
	// Protocol ID
	if err := e.buf.WriteByte(submit.ProtocolID); err != nil {
		return nil, fmt.Errorf("failed to write protocol_id: %w", err)
	}
	
	// Priority Flag
	if err := e.buf.WriteByte(submit.PriorityFlag); err != nil {
		return nil, fmt.Errorf("failed to write priority_flag: %w", err)
	}
	
	// Schedule Delivery Time (C-Octet String, max 17)
	if err := e.writeCString(submit.ScheduleDeliveryTime, 17); err != nil {
		return nil, fmt.Errorf("failed to write schedule_delivery_time: %w", err)
	}
	
	// Validity Period (C-Octet String, max 17)
	if err := e.writeCString(submit.ValidityPeriod, 17); err != nil {
		return nil, fmt.Errorf("failed to write validity_period: %w", err)
	}
	
	// Registered Delivery
	if err := e.buf.WriteByte(submit.RegisteredDelivery); err != nil {
		return nil, fmt.Errorf("failed to write registered_delivery: %w", err)
	}
	
	// Replace If Present
	if err := e.buf.WriteByte(submit.ReplaceIfPresent); err != nil {
		return nil, fmt.Errorf("failed to write replace_if_present: %w", err)
	}
	
	// Data Coding
	if err := e.buf.WriteByte(submit.DataCoding); err != nil {
		return nil, fmt.Errorf("failed to write data_coding: %w", err)
	}
	
	// SM Default Msg ID
	if err := e.buf.WriteByte(submit.SMDefaultMsgID); err != nil {
		return nil, fmt.Errorf("failed to write sm_default_msg_id: %w", err)
	}
	
	// SM Length
	if err := e.buf.WriteByte(submit.SMLength); err != nil {
		return nil, fmt.Errorf("failed to write sm_length: %w", err)
	}
	
	// Short Message (Octet String, length = sm_length)
	if submit.SMLength > 0 && len(submit.ShortMessage) > 0 {
		if int(submit.SMLength) != len(submit.ShortMessage) {
			return nil, fmt.Errorf("sm_length (%d) does not match short_message length (%d)", submit.SMLength, len(submit.ShortMessage))
		}
		if _, err := e.buf.Write(submit.ShortMessage); err != nil {
			return nil, fmt.Errorf("failed to write short_message: %w", err)
		}
	}
	
	// Optional Parameters (TLV)
	if submit.TLV != nil && len(submit.TLV) > 0 {
		if err := e.writeTLV(submit.TLV); err != nil {
			return nil, fmt.Errorf("failed to write TLV: %w", err)
		}
	}
	
	return e.buf.Bytes(), nil
}

// EncodeSubmitSMResp кодирует submit_sm_resp PDU
func (e *Encoder) EncodeSubmitSMResp(resp *SubmitSMRespPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(resp.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeDeliverSM кодирует deliver_sm PDU (аналогично submit_sm)
func (e *Encoder) EncodeDeliverSM(deliver *DeliverSMPDU) ([]byte, error) {
	// DeliverSM имеет такую же структуру как SubmitSM
	submit := &SubmitSMPDU{
		ServiceType:          deliver.ServiceType,
		SourceAddrTON:        deliver.SourceAddrTON,
		SourceAddrNPI:        deliver.SourceAddrNPI,
		SourceAddr:           deliver.SourceAddr,
		DestAddrTON:          deliver.DestAddrTON,
		DestAddrNPI:          deliver.DestAddrNPI,
		DestinationAddr:      deliver.DestinationAddr,
		ESMClass:             deliver.ESMClass,
		ProtocolID:           deliver.ProtocolID,
		PriorityFlag:         deliver.PriorityFlag,
		ScheduleDeliveryTime: deliver.ScheduleDeliveryTime,
		ValidityPeriod:       deliver.ValidityPeriod,
		RegisteredDelivery:   deliver.RegisteredDelivery,
		ReplaceIfPresent:     deliver.ReplaceIfPresent,
		DataCoding:           deliver.DataCoding,
		SMDefaultMsgID:       deliver.SMDefaultMsgID,
		SMLength:             deliver.SMLength,
		ShortMessage:         deliver.ShortMessage,
		TLV:                  deliver.TLV,
	}
	return e.EncodeSubmitSM(submit)
}

// EncodeDeliverSMResp кодирует deliver_sm_resp PDU
func (e *Encoder) EncodeDeliverSMResp(resp *DeliverSMRespPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(resp.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeEnquireLink кодирует enquire_link PDU (пустое тело)
func (e *Encoder) EncodeEnquireLink(enquire *EnquireLinkPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeEnquireLinkResp кодирует enquire_link_resp PDU (пустое тело)
func (e *Encoder) EncodeEnquireLinkResp(resp *EnquireLinkRespPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeUnbind кодирует unbind PDU (пустое тело)
func (e *Encoder) EncodeUnbind(unbind *UnbindPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeUnbindResp кодирует unbind_resp PDU (пустое тело)
func (e *Encoder) EncodeUnbindResp(resp *UnbindRespPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeGenericNack кодирует generic_nack PDU (пустое тело)
func (e *Encoder) EncodeGenericNack(nack *GenericNackPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeQuerySM кодирует query_sm PDU
func (e *Encoder) EncodeQuerySM(query *QuerySMPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(query.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	// Source Addr TON
	if err := e.buf.WriteByte(query.SourceAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if err := e.buf.WriteByte(query.SourceAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_npi: %w", err)
	}
	
	// Source Addr (C-Octet String, max 21)
	if err := e.writeCString(query.SourceAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write source_addr: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeQuerySMResp кодирует query_sm_resp PDU
func (e *Encoder) EncodeQuerySMResp(resp *QuerySMRespPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(resp.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	// Final Date (C-Octet String, max 17)
	if err := e.writeCString(resp.FinalDate, 17); err != nil {
		return nil, fmt.Errorf("failed to write final_date: %w", err)
	}
	
	// Message State
	if err := e.buf.WriteByte(resp.MessageState); err != nil {
		return nil, fmt.Errorf("failed to write message_state: %w", err)
	}
	
	// Error Code
	if err := e.buf.WriteByte(resp.ErrorCode); err != nil {
		return nil, fmt.Errorf("failed to write error_code: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeCancelSM кодирует cancel_sm PDU
func (e *Encoder) EncodeCancelSM(cancel *CancelSMPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Service Type (C-Octet String, max 6)
	if err := e.writeCString(cancel.ServiceType, MaxServiceTypeLength); err != nil {
		return nil, fmt.Errorf("failed to write service_type: %w", err)
	}
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(cancel.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	// Source Addr TON
	if err := e.buf.WriteByte(cancel.SourceAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if err := e.buf.WriteByte(cancel.SourceAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_npi: %w", err)
	}
	
	// Source Addr (C-Octet String, max 21)
	if err := e.writeCString(cancel.SourceAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write source_addr: %w", err)
	}
	
	// Dest Addr TON
	if err := e.buf.WriteByte(cancel.DestAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write dest_addr_ton: %w", err)
	}
	
	// Dest Addr NPI
	if err := e.buf.WriteByte(cancel.DestAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write dest_addr_npi: %w", err)
	}
	
	// Destination Addr (C-Octet String, max 21)
	if err := e.writeCString(cancel.DestinationAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write destination_addr: %w", err)
	}
	
	return e.buf.Bytes(), nil
}

// EncodeCancelSMResp кодирует cancel_sm_resp PDU (пустое тело)
func (e *Encoder) EncodeCancelSMResp(resp *CancelSMRespPDU) ([]byte, error) {
	return []byte{}, nil
}

// EncodeReplaceSM кодирует replace_sm PDU
func (e *Encoder) EncodeReplaceSM(replace *ReplaceSMPDU) ([]byte, error) {
	e.buf.Reset()
	
	// Message ID (C-Octet String, max 65)
	if err := e.writeCString(replace.MessageID, MaxSMPPMessageIDLength); err != nil {
		return nil, fmt.Errorf("failed to write message_id: %w", err)
	}
	
	// Source Addr TON
	if err := e.buf.WriteByte(replace.SourceAddrTON); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if err := e.buf.WriteByte(replace.SourceAddrNPI); err != nil {
		return nil, fmt.Errorf("failed to write source_addr_npi: %w", err)
	}
	
	// Source Addr (C-Octet String, max 21)
	if err := e.writeCString(replace.SourceAddr, MaxAddressLength); err != nil {
		return nil, fmt.Errorf("failed to write source_addr: %w", err)
	}
	
	// Schedule Delivery Time (C-Octet String, max 17)
	if err := e.writeCString(replace.ScheduleDeliveryTime, 17); err != nil {
		return nil, fmt.Errorf("failed to write schedule_delivery_time: %w", err)
	}
	
	// Validity Period (C-Octet String, max 17)
	if err := e.writeCString(replace.ValidityPeriod, 17); err != nil {
		return nil, fmt.Errorf("failed to write validity_period: %w", err)
	}
	
	// Registered Delivery
	if err := e.buf.WriteByte(replace.RegisteredDelivery); err != nil {
		return nil, fmt.Errorf("failed to write registered_delivery: %w", err)
	}
	
	// SM Default Msg ID
	if err := e.buf.WriteByte(replace.SMDefaultMsgID); err != nil {
		return nil, fmt.Errorf("failed to write sm_default_msg_id: %w", err)
	}
	
	// SM Length
	if err := e.buf.WriteByte(replace.SMLength); err != nil {
		return nil, fmt.Errorf("failed to write sm_length: %w", err)
	}
	
	// Short Message (Octet String, length = sm_length)
	if replace.SMLength > 0 && len(replace.ShortMessage) > 0 {
		if int(replace.SMLength) != len(replace.ShortMessage) {
			return nil, fmt.Errorf("sm_length (%d) does not match short_message length (%d)", replace.SMLength, len(replace.ShortMessage))
		}
		if _, err := e.buf.Write(replace.ShortMessage); err != nil {
			return nil, fmt.Errorf("failed to write short_message: %w", err)
		}
	}
	
	return e.buf.Bytes(), nil
}

// EncodeReplaceSMResp кодирует replace_sm_resp PDU (пустое тело)
func (e *Encoder) EncodeReplaceSMResp(resp *ReplaceSMRespPDU) ([]byte, error) {
	return []byte{}, nil
}

// writeCString записывает C-Octet String (null-terminated)
func (e *Encoder) writeCString(s string, maxLen int) error {
	if len(s) > maxLen {
		return fmt.Errorf("string length (%d) exceeds maximum (%d)", len(s), maxLen)
	}
	if _, err := e.buf.WriteString(s); err != nil {
		return err
	}
	// Добавляем null terminator
	return e.buf.WriteByte(0)
}

// writeTLV записывает TLV (Tag-Length-Value) параметры
func (e *Encoder) writeTLV(tlv map[uint16][]byte) error {
	for tag, value := range tlv {
		// Tag (2 bytes)
		if err := binary.Write(e.buf, binary.BigEndian, tag); err != nil {
			return fmt.Errorf("failed to write TLV tag: %w", err)
		}
		// Length (2 bytes)
		length := uint16(len(value))
		if err := binary.Write(e.buf, binary.BigEndian, length); err != nil {
			return fmt.Errorf("failed to write TLV length: %w", err)
		}
		// Value
		if length > 0 {
			if _, err := e.buf.Write(value); err != nil {
				return fmt.Errorf("failed to write TLV value: %w", err)
			}
		}
	}
	return nil
}

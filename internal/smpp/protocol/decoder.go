package protocol

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// Decoder декодирует бинарные данные SMPP в PDU структуры
type Decoder struct {
	reader *bytes.Reader
}

// NewDecoder создает новый decoder
func NewDecoder(data []byte) *Decoder {
	return &Decoder{
		reader: bytes.NewReader(data),
	}
}

// DecodePDU декодирует базовую PDU структуру из бинарных данных
func (d *Decoder) DecodePDU() (*PDU, error) {
	pdu := &PDU{}
	
	// Читаем заголовок (16 байт)
	if d.reader.Len() < PDUHeaderLength {
		return nil, fmt.Errorf("insufficient data for PDU header: got %d bytes, need %d", d.reader.Len(), PDUHeaderLength)
	}
	
	if err := binary.Read(d.reader, binary.BigEndian, &pdu.CommandLength); err != nil {
		return nil, fmt.Errorf("failed to read command_length: %w", err)
	}
	
	if err := binary.Read(d.reader, binary.BigEndian, &pdu.CommandID); err != nil {
		return nil, fmt.Errorf("failed to read command_id: %w", err)
	}
	
	if err := binary.Read(d.reader, binary.BigEndian, &pdu.CommandStatus); err != nil {
		return nil, fmt.Errorf("failed to read command_status: %w", err)
	}
	
	if err := binary.Read(d.reader, binary.BigEndian, &pdu.SequenceNumber); err != nil {
		return nil, fmt.Errorf("failed to read sequence_number: %w", err)
	}
	
	// Проверяем длину
	if pdu.CommandLength < PDUHeaderLength {
		return nil, fmt.Errorf("invalid command_length: %d (must be at least %d)", pdu.CommandLength, PDUHeaderLength)
	}
	
	// Читаем тело
	bodyLength := int(pdu.CommandLength) - PDUHeaderLength
	if bodyLength > 0 {
		pdu.Body = make([]byte, bodyLength)
		if n, err := d.reader.Read(pdu.Body); err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read body: %w", err)
		} else if n < bodyLength {
			return nil, fmt.Errorf("insufficient data for body: got %d bytes, need %d", n, bodyLength)
		}
	}
	
	return pdu, nil
}

// DecodeBind декодирует bind PDU
func (d *Decoder) DecodeBind(body []byte) (*BindPDU, error) {
	d.reader = bytes.NewReader(body)
	bind := &BindPDU{}
	
	// System ID
	systemID, err := d.readCString(MaxSystemIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read system_id: %w", err)
	}
	bind.SystemID = systemID
	
	// Password
	password, err := d.readCString(MaxPasswordLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read password: %w", err)
	}
	bind.Password = password
	
	// System Type
	systemType, err := d.readCString(MaxSystemTypeLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read system_type: %w", err)
	}
	bind.SystemType = systemType
	
	// Interface Version
	if bind.InterfaceVersion, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read interface_version: %w", err)
	}
	
	// Addr TON
	if bind.AddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read addr_ton: %w", err)
	}
	
	// Addr NPI
	if bind.AddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read addr_npi: %w", err)
	}
	
	// Address Range
	addressRange, err := d.readCString(41)
	if err != nil {
		return nil, fmt.Errorf("failed to read address_range: %w", err)
	}
	bind.AddressRange = addressRange
	
	return bind, nil
}

// DecodeBindResp декодирует bind response PDU
func (d *Decoder) DecodeBindResp(body []byte) (*BindRespPDU, error) {
	d.reader = bytes.NewReader(body)
	resp := &BindRespPDU{
		TLV: make(map[uint16][]byte),
	}
	
	// System ID
	systemID, err := d.readCString(MaxSystemIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read system_id: %w", err)
	}
	resp.SystemID = systemID
	
	// Optional Parameters (TLV)
	if d.reader.Len() > 0 {
		tlv, err := d.readTLV()
		if err != nil {
			return nil, fmt.Errorf("failed to read TLV: %w", err)
		}
		resp.TLV = tlv
	}
	
	return resp, nil
}

// DecodeSubmitSM декодирует submit_sm PDU
func (d *Decoder) DecodeSubmitSM(body []byte) (*SubmitSMPDU, error) {
	d.reader = bytes.NewReader(body)
	submit := &SubmitSMPDU{
		TLV: make(map[uint16][]byte),
	}
	
	// Service Type
	serviceType, err := d.readCString(MaxServiceTypeLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read service_type: %w", err)
	}
	submit.ServiceType = serviceType
	
	// Source Addr TON
	if submit.SourceAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if submit.SourceAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_npi: %w", err)
	}
	
	// Source Addr
	sourceAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read source_addr: %w", err)
	}
	submit.SourceAddr = sourceAddr
	
	// Dest Addr TON
	if submit.DestAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read dest_addr_ton: %w", err)
	}
	
	// Dest Addr NPI
	if submit.DestAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read dest_addr_npi: %w", err)
	}
	
	// Destination Addr
	destAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read destination_addr: %w", err)
	}
	submit.DestinationAddr = destAddr
	
	// ESM Class
	if submit.ESMClass, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read esm_class: %w", err)
	}
	
	// Protocol ID
	if submit.ProtocolID, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read protocol_id: %w", err)
	}
	
	// Priority Flag
	if submit.PriorityFlag, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read priority_flag: %w", err)
	}
	
	// Schedule Delivery Time
	scheduleTime, err := d.readCString(17)
	if err != nil {
		return nil, fmt.Errorf("failed to read schedule_delivery_time: %w", err)
	}
	submit.ScheduleDeliveryTime = scheduleTime
	
	// Validity Period
	validityPeriod, err := d.readCString(17)
	if err != nil {
		return nil, fmt.Errorf("failed to read validity_period: %w", err)
	}
	submit.ValidityPeriod = validityPeriod
	
	// Registered Delivery
	if submit.RegisteredDelivery, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read registered_delivery: %w", err)
	}
	
	// Replace If Present
	if submit.ReplaceIfPresent, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read replace_if_present: %w", err)
	}
	
	// Data Coding
	if submit.DataCoding, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read data_coding: %w", err)
	}
	
	// SM Default Msg ID
	if submit.SMDefaultMsgID, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read sm_default_msg_id: %w", err)
	}
	
	// SM Length
	if submit.SMLength, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read sm_length: %w", err)
	}
	
	// Short Message
	if submit.SMLength > 0 {
		submit.ShortMessage = make([]byte, submit.SMLength)
		if n, err := d.reader.Read(submit.ShortMessage); err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read short_message: %w", err)
		} else if n < int(submit.SMLength) {
			return nil, fmt.Errorf("insufficient data for short_message: got %d bytes, need %d", n, submit.SMLength)
		}
	}
	
	// Optional Parameters (TLV)
	if d.reader.Len() > 0 {
		tlv, err := d.readTLV()
		if err != nil {
			return nil, fmt.Errorf("failed to read TLV: %w", err)
		}
		submit.TLV = tlv
	}
	
	return submit, nil
}

// DecodeSubmitSMResp декодирует submit_sm_resp PDU
func (d *Decoder) DecodeSubmitSMResp(body []byte) (*SubmitSMRespPDU, error) {
	d.reader = bytes.NewReader(body)
	resp := &SubmitSMRespPDU{}
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	resp.MessageID = messageID
	
	return resp, nil
}

// DecodeDeliverSM декодирует deliver_sm PDU (аналогично submit_sm)
func (d *Decoder) DecodeDeliverSM(body []byte) (*DeliverSMPDU, error) {
	submit, err := d.DecodeSubmitSM(body)
	if err != nil {
		return nil, err
	}
	
	deliver := &DeliverSMPDU{
		ServiceType:          submit.ServiceType,
		SourceAddrTON:        submit.SourceAddrTON,
		SourceAddrNPI:        submit.SourceAddrNPI,
		SourceAddr:           submit.SourceAddr,
		DestAddrTON:          submit.DestAddrTON,
		DestAddrNPI:          submit.DestAddrNPI,
		DestinationAddr:      submit.DestinationAddr,
		ESMClass:             submit.ESMClass,
		ProtocolID:           submit.ProtocolID,
		PriorityFlag:         submit.PriorityFlag,
		ScheduleDeliveryTime: submit.ScheduleDeliveryTime,
		ValidityPeriod:       submit.ValidityPeriod,
		RegisteredDelivery:   submit.RegisteredDelivery,
		ReplaceIfPresent:     submit.ReplaceIfPresent,
		DataCoding:           submit.DataCoding,
		SMDefaultMsgID:       submit.SMDefaultMsgID,
		SMLength:             submit.SMLength,
		ShortMessage:         submit.ShortMessage,
		TLV:                  submit.TLV,
	}
	
	return deliver, nil
}

// DecodeDeliverSMResp декодирует deliver_sm_resp PDU
func (d *Decoder) DecodeDeliverSMResp(body []byte) (*DeliverSMRespPDU, error) {
	d.reader = bytes.NewReader(body)
	resp := &DeliverSMRespPDU{}
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	resp.MessageID = messageID
	
	return resp, nil
}

// DecodeEnquireLink декодирует enquire_link PDU (пустое тело)
func (d *Decoder) DecodeEnquireLink(body []byte) (*EnquireLinkPDU, error) {
	return &EnquireLinkPDU{}, nil
}

// DecodeEnquireLinkResp декодирует enquire_link_resp PDU (пустое тело)
func (d *Decoder) DecodeEnquireLinkResp(body []byte) (*EnquireLinkRespPDU, error) {
	return &EnquireLinkRespPDU{}, nil
}

// DecodeUnbind декодирует unbind PDU (пустое тело)
func (d *Decoder) DecodeUnbind(body []byte) (*UnbindPDU, error) {
	return &UnbindPDU{}, nil
}

// DecodeUnbindResp декодирует unbind_resp PDU (пустое тело)
func (d *Decoder) DecodeUnbindResp(body []byte) (*UnbindRespPDU, error) {
	return &UnbindRespPDU{}, nil
}

// DecodeGenericNack декодирует generic_nack PDU (пустое тело)
func (d *Decoder) DecodeGenericNack(body []byte) (*GenericNackPDU, error) {
	return &GenericNackPDU{}, nil
}

// DecodeQuerySM декодирует query_sm PDU
func (d *Decoder) DecodeQuerySM(body []byte) (*QuerySMPDU, error) {
	d.reader = bytes.NewReader(body)
	query := &QuerySMPDU{}
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	query.MessageID = messageID
	
	// Source Addr TON
	if query.SourceAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if query.SourceAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_npi: %w", err)
	}
	
	// Source Addr
	sourceAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read source_addr: %w", err)
	}
	query.SourceAddr = sourceAddr
	
	return query, nil
}

// DecodeQuerySMResp декодирует query_sm_resp PDU
func (d *Decoder) DecodeQuerySMResp(body []byte) (*QuerySMRespPDU, error) {
	d.reader = bytes.NewReader(body)
	resp := &QuerySMRespPDU{}
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	resp.MessageID = messageID
	
	// Final Date
	finalDate, err := d.readCString(17)
	if err != nil {
		return nil, fmt.Errorf("failed to read final_date: %w", err)
	}
	resp.FinalDate = finalDate
	
	// Message State
	if resp.MessageState, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read message_state: %w", err)
	}
	
	// Error Code
	if resp.ErrorCode, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read error_code: %w", err)
	}
	
	return resp, nil
}

// DecodeCancelSM декодирует cancel_sm PDU
func (d *Decoder) DecodeCancelSM(body []byte) (*CancelSMPDU, error) {
	d.reader = bytes.NewReader(body)
	cancel := &CancelSMPDU{}
	
	// Service Type
	serviceType, err := d.readCString(MaxServiceTypeLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read service_type: %w", err)
	}
	cancel.ServiceType = serviceType
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	cancel.MessageID = messageID
	
	// Source Addr TON
	if cancel.SourceAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if cancel.SourceAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_npi: %w", err)
	}
	
	// Source Addr
	sourceAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read source_addr: %w", err)
	}
	cancel.SourceAddr = sourceAddr
	
	// Dest Addr TON
	if cancel.DestAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read dest_addr_ton: %w", err)
	}
	
	// Dest Addr NPI
	if cancel.DestAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read dest_addr_npi: %w", err)
	}
	
	// Destination Addr
	destAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read destination_addr: %w", err)
	}
	cancel.DestinationAddr = destAddr
	
	return cancel, nil
}

// DecodeCancelSMResp декодирует cancel_sm_resp PDU (пустое тело)
func (d *Decoder) DecodeCancelSMResp(body []byte) (*CancelSMRespPDU, error) {
	return &CancelSMRespPDU{}, nil
}

// DecodeReplaceSM декодирует replace_sm PDU
func (d *Decoder) DecodeReplaceSM(body []byte) (*ReplaceSMPDU, error) {
	d.reader = bytes.NewReader(body)
	replace := &ReplaceSMPDU{}
	
	// Message ID
	messageID, err := d.readCString(MaxSMPPMessageIDLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read message_id: %w", err)
	}
	replace.MessageID = messageID
	
	// Source Addr TON
	if replace.SourceAddrTON, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_ton: %w", err)
	}
	
	// Source Addr NPI
	if replace.SourceAddrNPI, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read source_addr_npi: %w", err)
	}
	
	// Source Addr
	sourceAddr, err := d.readCString(MaxAddressLength)
	if err != nil {
		return nil, fmt.Errorf("failed to read source_addr: %w", err)
	}
	replace.SourceAddr = sourceAddr
	
	// Schedule Delivery Time
	scheduleTime, err := d.readCString(17)
	if err != nil {
		return nil, fmt.Errorf("failed to read schedule_delivery_time: %w", err)
	}
	replace.ScheduleDeliveryTime = scheduleTime
	
	// Validity Period
	validityPeriod, err := d.readCString(17)
	if err != nil {
		return nil, fmt.Errorf("failed to read validity_period: %w", err)
	}
	replace.ValidityPeriod = validityPeriod
	
	// Registered Delivery
	if replace.RegisteredDelivery, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read registered_delivery: %w", err)
	}
	
	// SM Default Msg ID
	if replace.SMDefaultMsgID, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read sm_default_msg_id: %w", err)
	}
	
	// SM Length
	if replace.SMLength, err = d.reader.ReadByte(); err != nil {
		return nil, fmt.Errorf("failed to read sm_length: %w", err)
	}
	
	// Short Message
	if replace.SMLength > 0 {
		replace.ShortMessage = make([]byte, replace.SMLength)
		if n, err := d.reader.Read(replace.ShortMessage); err != nil && err != io.EOF {
			return nil, fmt.Errorf("failed to read short_message: %w", err)
		} else if n < int(replace.SMLength) {
			return nil, fmt.Errorf("insufficient data for short_message: got %d bytes, need %d", n, replace.SMLength)
		}
	}
	
	return replace, nil
}

// DecodeReplaceSMResp декодирует replace_sm_resp PDU (пустое тело)
func (d *Decoder) DecodeReplaceSMResp(body []byte) (*ReplaceSMRespPDU, error) {
	return &ReplaceSMRespPDU{}, nil
}

// readCString читает C-Octet String (null-terminated)
func (d *Decoder) readCString(maxLen int) (string, error) {
	var buf bytes.Buffer
	for i := 0; i < maxLen+1; i++ { // +1 для null terminator
		b, err := d.reader.ReadByte()
		if err != nil {
			return "", fmt.Errorf("failed to read C-String: %w", err)
		}
		if b == 0 {
			break // null terminator найден
		}
		buf.WriteByte(b)
	}
	return buf.String(), nil
}

// readTLV читает TLV (Tag-Length-Value) параметры
func (d *Decoder) readTLV() (map[uint16][]byte, error) {
	tlv := make(map[uint16][]byte)
	
	for d.reader.Len() >= 4 { // минимум Tag (2) + Length (2)
		// Tag (2 bytes)
		var tag uint16
		if err := binary.Read(d.reader, binary.BigEndian, &tag); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("failed to read TLV tag: %w", err)
		}
		
		// Length (2 bytes)
		var length uint16
		if err := binary.Read(d.reader, binary.BigEndian, &length); err != nil {
			return nil, fmt.Errorf("failed to read TLV length: %w", err)
		}
		
		// Value
		if length > 0 {
			if d.reader.Len() < int(length) {
				return nil, fmt.Errorf("insufficient data for TLV value: got %d bytes, need %d", d.reader.Len(), length)
			}
			value := make([]byte, length)
			if n, err := d.reader.Read(value); err != nil {
				return nil, fmt.Errorf("failed to read TLV value: %w", err)
			} else if n < int(length) {
				return nil, fmt.Errorf("insufficient data for TLV value: got %d bytes, need %d", n, length)
			}
			tlv[tag] = value
		} else {
			tlv[tag] = []byte{}
		}
	}
	
	return tlv, nil
}

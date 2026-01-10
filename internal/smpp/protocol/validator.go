package protocol

import (
	"fmt"
)

// Validator валидирует PDU структуры на соответствие спецификации SMPP
type Validator struct{}

// NewValidator создает новый валидатор
func NewValidator() *Validator {
	return &Validator{}
}

// ValidatePDU валидирует базовую PDU структуру
func (v *Validator) ValidatePDU(pdu *PDU) error {
	if pdu.CommandLength < PDUHeaderLength {
		return fmt.Errorf("invalid command_length: %d (must be at least %d)", pdu.CommandLength, PDUHeaderLength)
	}
	
	if pdu.CommandLength != uint32(PDUHeaderLength+len(pdu.Body)) {
		return fmt.Errorf("command_length (%d) does not match header + body length (%d)", 
			pdu.CommandLength, PDUHeaderLength+len(pdu.Body))
	}
	
	// Проверяем, что CommandID известен
	if GetCommandName(pdu.CommandID) == fmt.Sprintf("unknown_0x%08X", pdu.CommandID) {
		return fmt.Errorf("unknown command_id: 0x%08X", pdu.CommandID)
	}
	
	return nil
}

// ValidateBind валидирует bind PDU
func (v *Validator) ValidateBind(bind *BindPDU) error {
	if len(bind.SystemID) == 0 || len(bind.SystemID) > MaxSystemIDLength {
		return fmt.Errorf("invalid system_id: length must be between 1 and %d", MaxSystemIDLength)
	}
	
	if len(bind.Password) == 0 || len(bind.Password) > MaxPasswordLength {
		return fmt.Errorf("invalid password: length must be between 1 and %d", MaxPasswordLength)
	}
	
	if len(bind.SystemType) > MaxSystemTypeLength {
		return fmt.Errorf("invalid system_type: length must be at most %d", MaxSystemTypeLength)
	}
	
	if bind.InterfaceVersion != Version {
		return fmt.Errorf("invalid interface_version: got 0x%02X, expected 0x%02X (SMPP v3.4)", bind.InterfaceVersion, Version)
	}
	
	if bind.AddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid addr_ton: %d (must be 0-6)", bind.AddrTON)
	}
	
	if bind.AddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid addr_npi: %d (must be 0-18)", bind.AddrNPI)
	}
	
	if len(bind.AddressRange) > 41 {
		return fmt.Errorf("invalid address_range: length must be at most 41")
	}
	
	return nil
}

// ValidateBindResp валидирует bind response PDU
func (v *Validator) ValidateBindResp(resp *BindRespPDU) error {
	if len(resp.SystemID) == 0 || len(resp.SystemID) > MaxSystemIDLength {
		return fmt.Errorf("invalid system_id: length must be between 1 and %d", MaxSystemIDLength)
	}
	
	return nil
}

// ValidateSubmitSM валидирует submit_sm PDU
func (v *Validator) ValidateSubmitSM(submit *SubmitSMPDU) error {
	if len(submit.ServiceType) > MaxServiceTypeLength {
		return fmt.Errorf("invalid service_type: length must be at most %d", MaxServiceTypeLength)
	}
	
	if submit.SourceAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid source_addr_ton: %d (must be 0-6)", submit.SourceAddrTON)
	}
	
	if submit.SourceAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid source_addr_npi: %d (must be 0-18)", submit.SourceAddrNPI)
	}
	
	if len(submit.SourceAddr) == 0 || len(submit.SourceAddr) > MaxAddressLength {
		return fmt.Errorf("invalid source_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	if submit.DestAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid dest_addr_ton: %d (must be 0-6)", submit.DestAddrTON)
	}
	
	if submit.DestAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid dest_addr_npi: %d (must be 0-18)", submit.DestAddrNPI)
	}
	
	if len(submit.DestinationAddr) == 0 || len(submit.DestinationAddr) > MaxAddressLength {
		return fmt.Errorf("invalid destination_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	if len(submit.ScheduleDeliveryTime) > 17 {
		return fmt.Errorf("invalid schedule_delivery_time: length must be at most 17")
	}
	
	if len(submit.ValidityPeriod) > 17 {
		return fmt.Errorf("invalid validity_period: length must be at most 17")
	}
	
	if submit.ReplaceIfPresent > REPLACE_IF_PRESENT_YES {
		return fmt.Errorf("invalid replace_if_present: %d (must be 0 or 1)", submit.ReplaceIfPresent)
	}
	
	if submit.PriorityFlag > PRIORITY_FLAG_LEVEL_3 {
		return fmt.Errorf("invalid priority_flag: %d (must be 0-3)", submit.PriorityFlag)
	}
	
	if int(submit.SMLength) != len(submit.ShortMessage) {
		return fmt.Errorf("invalid sm_length: %d does not match short_message length %d", 
			submit.SMLength, len(submit.ShortMessage))
	}
	
	if len(submit.ShortMessage) > MaxMessageLength {
		return fmt.Errorf("invalid short_message: length %d exceeds maximum %d", 
			len(submit.ShortMessage), MaxMessageLength)
	}
	
	return nil
}

// ValidateSubmitSMResp валидирует submit_sm_resp PDU
func (v *Validator) ValidateSubmitSMResp(resp *SubmitSMRespPDU) error {
	if len(resp.MessageID) == 0 || len(resp.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	return nil
}

// ValidateDeliverSM валидирует deliver_sm PDU (аналогично submit_sm)
func (v *Validator) ValidateDeliverSM(deliver *DeliverSMPDU) error {
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
	return v.ValidateSubmitSM(submit)
}

// ValidateDeliverSMResp валидирует deliver_sm_resp PDU
func (v *Validator) ValidateDeliverSMResp(resp *DeliverSMRespPDU) error {
	if len(resp.MessageID) == 0 || len(resp.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	return nil
}

// ValidateQuerySM валидирует query_sm PDU
func (v *Validator) ValidateQuerySM(query *QuerySMPDU) error {
	if len(query.MessageID) == 0 || len(query.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	if query.SourceAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid source_addr_ton: %d (must be 0-6)", query.SourceAddrTON)
	}
	
	if query.SourceAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid source_addr_npi: %d (must be 0-18)", query.SourceAddrNPI)
	}
	
	if len(query.SourceAddr) == 0 || len(query.SourceAddr) > MaxAddressLength {
		return fmt.Errorf("invalid source_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	return nil
}

// ValidateQuerySMResp валидирует query_sm_resp PDU
func (v *Validator) ValidateQuerySMResp(resp *QuerySMRespPDU) error {
	if len(resp.MessageID) == 0 || len(resp.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	if len(resp.FinalDate) > 17 {
		return fmt.Errorf("invalid final_date: length must be at most 17")
	}
	
	return nil
}

// ValidateCancelSM валидирует cancel_sm PDU
func (v *Validator) ValidateCancelSM(cancel *CancelSMPDU) error {
	if len(cancel.ServiceType) > MaxServiceTypeLength {
		return fmt.Errorf("invalid service_type: length must be at most %d", MaxServiceTypeLength)
	}
	
	if len(cancel.MessageID) == 0 || len(cancel.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	if cancel.SourceAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid source_addr_ton: %d (must be 0-6)", cancel.SourceAddrTON)
	}
	
	if cancel.SourceAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid source_addr_npi: %d (must be 0-18)", cancel.SourceAddrNPI)
	}
	
	if len(cancel.SourceAddr) == 0 || len(cancel.SourceAddr) > MaxAddressLength {
		return fmt.Errorf("invalid source_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	if cancel.DestAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid dest_addr_ton: %d (must be 0-6)", cancel.DestAddrTON)
	}
	
	if cancel.DestAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid dest_addr_npi: %d (must be 0-18)", cancel.DestAddrNPI)
	}
	
	if len(cancel.DestinationAddr) == 0 || len(cancel.DestinationAddr) > MaxAddressLength {
		return fmt.Errorf("invalid destination_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	return nil
}

// ValidateReplaceSM валидирует replace_sm PDU
func (v *Validator) ValidateReplaceSM(replace *ReplaceSMPDU) error {
	if len(replace.MessageID) == 0 || len(replace.MessageID) > MaxSMPPMessageIDLength {
		return fmt.Errorf("invalid message_id: length must be between 1 and %d", MaxSMPPMessageIDLength)
	}
	
	if replace.SourceAddrTON > TON_ABBREVIATED {
		return fmt.Errorf("invalid source_addr_ton: %d (must be 0-6)", replace.SourceAddrTON)
	}
	
	if replace.SourceAddrNPI > NPI_WAP_CLIENT {
		return fmt.Errorf("invalid source_addr_npi: %d (must be 0-18)", replace.SourceAddrNPI)
	}
	
	if len(replace.SourceAddr) == 0 || len(replace.SourceAddr) > MaxAddressLength {
		return fmt.Errorf("invalid source_addr: length must be between 1 and %d", MaxAddressLength)
	}
	
	if len(replace.ScheduleDeliveryTime) > 17 {
		return fmt.Errorf("invalid schedule_delivery_time: length must be at most 17")
	}
	
	if len(replace.ValidityPeriod) > 17 {
		return fmt.Errorf("invalid validity_period: length must be at most 17")
	}
	
	if int(replace.SMLength) != len(replace.ShortMessage) {
		return fmt.Errorf("invalid sm_length: %d does not match short_message length %d", 
			replace.SMLength, len(replace.ShortMessage))
	}
	
	if len(replace.ShortMessage) > MaxMessageLength {
		return fmt.Errorf("invalid short_message: length %d exceeds maximum %d", 
			len(replace.ShortMessage), MaxMessageLength)
	}
	
	return nil
}

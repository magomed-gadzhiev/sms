package protocol

import (
	"fmt"
)

// ParsePDU парсит бинарные данные в PDU структуру
func ParsePDU(data []byte) (*PDU, error) {
	if len(data) < PDUHeaderLength {
		return nil, fmt.Errorf("insufficient data for PDU header: got %d bytes, need %d", len(data), PDUHeaderLength)
	}
	
	decoder := NewDecoder(data)
	pdu, err := decoder.DecodePDU()
	if err != nil {
		return nil, fmt.Errorf("failed to decode PDU: %w", err)
	}
	
	// Валидация
	validator := NewValidator()
	if err := validator.ValidatePDU(pdu); err != nil {
		return nil, fmt.Errorf("PDU validation failed: %w", err)
	}
	
	return pdu, nil
}

// BuildPDU создает PDU структуру с телом
func BuildPDU(commandID, commandStatus, sequenceNumber uint32, body []byte) *PDU {
	return &PDU{
		CommandLength:  uint32(PDUHeaderLength + len(body)),
		CommandID:      commandID,
		CommandStatus:  commandStatus,
		SequenceNumber: sequenceNumber,
		Body:           body,
	}
}

// BuildResponsePDU создает response PDU на основе request PDU
func BuildResponsePDU(request *PDU, commandID uint32, commandStatus uint32, body []byte) *PDU {
	return &PDU{
		CommandLength:  uint32(PDUHeaderLength + len(body)),
		CommandID:      commandID,
		CommandStatus:  commandStatus,
		SequenceNumber: request.SequenceNumber,
		Body:           body,
	}
}

// BuildErrorResponse создает error response PDU
func BuildErrorResponse(request *PDU, errorCode uint32) *PDU {
	responseCommandID := request.CommandID | 0x80000000 // Устанавливаем бит response
	return BuildResponsePDU(request, responseCommandID, errorCode, []byte{})
}

// BuildSuccessResponse создает success response PDU
func BuildSuccessResponse(request *PDU, responseCommandID uint32, body []byte) *PDU {
	if (responseCommandID & 0x80000000) == 0 {
		responseCommandID |= 0x80000000 // Устанавливаем бит response
	}
	return BuildResponsePDU(request, responseCommandID, ESME_ROK, body)
}

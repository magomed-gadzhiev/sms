package protocol

import (
	"fmt"
	"time"
)

// PDU представляет базовую структуру Protocol Data Unit
type PDU struct {
	CommandLength  uint32
	CommandID      uint32
	CommandStatus  uint32
	SequenceNumber uint32
	Body           []byte
}

// BindPDU представляет базовую структуру для bind команд
type BindPDU struct {
	SystemID     string
	Password     string
	SystemType   string
	InterfaceVersion byte
	AddrTON      byte
	AddrNPI      byte
	AddressRange string
}

// BindRespPDU представляет ответ на bind команду
type BindRespPDU struct {
	SystemID string
	TLV      map[uint16][]byte // Optional Parameters
}

// SubmitSMPDU представляет команду submit_sm
type SubmitSMPDU struct {
	ServiceType          string
	SourceAddrTON        byte
	SourceAddrNPI        byte
	SourceAddr           string
	DestAddrTON          byte
	DestAddrNPI          byte
	DestinationAddr      string
	ESMClass             byte
	ProtocolID           byte
	PriorityFlag         byte
	ScheduleDeliveryTime string
	ValidityPeriod       string
	RegisteredDelivery   byte
	ReplaceIfPresent     byte
	DataCoding           byte
	SMDefaultMsgID       byte
	SMLength             byte
	ShortMessage         []byte
	TLV                  map[uint16][]byte // Optional Parameters
}

// SubmitSMRespPDU представляет ответ на submit_sm
type SubmitSMRespPDU struct {
	MessageID string
}

// DeliverSMPDU представляет команду deliver_sm (DLR от SMSC)
type DeliverSMPDU struct {
	ServiceType          string
	SourceAddrTON        byte
	SourceAddrNPI        byte
	SourceAddr           string
	DestAddrTON          byte
	DestAddrNPI          byte
	DestinationAddr      string
	ESMClass             byte
	ProtocolID           byte
	PriorityFlag         byte
	ScheduleDeliveryTime string
	ValidityPeriod       string
	RegisteredDelivery   byte
	ReplaceIfPresent     byte
	DataCoding           byte
	SMDefaultMsgID       byte
	SMLength             byte
	ShortMessage         []byte
	TLV                  map[uint16][]byte // Optional Parameters
}

// DeliverSMRespPDU представляет ответ на deliver_sm
type DeliverSMRespPDU struct {
	MessageID string
}

// EnquireLinkPDU представляет команду enquire_link
type EnquireLinkPDU struct {
	// Пустое тело
}

// EnquireLinkRespPDU представляет ответ на enquire_link
type EnquireLinkRespPDU struct {
	// Пустое тело
}

// UnbindPDU представляет команду unbind
type UnbindPDU struct {
	// Пустое тело
}

// UnbindRespPDU представляет ответ на unbind
type UnbindRespPDU struct {
	// Пустое тело
}

// GenericNackPDU представляет generic_nack ответ
type GenericNackPDU struct {
	// Пустое тело
}

// QuerySMPDU представляет команду query_sm
type QuerySMPDU struct {
	MessageID  string
	SourceAddrTON byte
	SourceAddrNPI byte
	SourceAddr   string
}

// QuerySMRespPDU представляет ответ на query_sm
type QuerySMRespPDU struct {
	MessageID      string
	FinalDate      string
	MessageState   byte
	ErrorCode      byte
}

// CancelSMPDU представляет команду cancel_sm
type CancelSMPDU struct {
	ServiceType   string
	MessageID     string
	SourceAddrTON byte
	SourceAddrNPI byte
	SourceAddr    string
	DestAddrTON   byte
	DestAddrNPI   byte
	DestinationAddr string
}

// CancelSMRespPDU представляет ответ на cancel_sm
type CancelSMRespPDU struct {
	// Пустое тело
}

// ReplaceSMPDU представляет команду replace_sm
type ReplaceSMPDU struct {
	MessageID           string
	SourceAddrTON       byte
	SourceAddrNPI       byte
	SourceAddr          string
	ScheduleDeliveryTime string
	ValidityPeriod      string
	RegisteredDelivery  byte
	SMDefaultMsgID      byte
	SMLength            byte
	ShortMessage        []byte
}

// ReplaceSMRespPDU представляет ответ на replace_sm
type ReplaceSMRespPDU struct {
	// Пустое тело
}

// GetCommandName возвращает имя команды по CommandID
func GetCommandName(commandID uint32) string {
	names := map[uint32]string{
		BindReceiver:         "bind_receiver",
		BindTransmitter:      "bind_transmitter",
		BindTransceiver:      "bind_transceiver",
		Unbind:               "unbind",
		UnbindResp:           "unbind_resp",
		GenericNack:          "generic_nack",
		BindReceiverResp:     "bind_receiver_resp",
		BindTransmitterResp:  "bind_transmitter_resp",
		BindTransceiverResp:  "bind_transceiver_resp",
		SubmitSM:             "submit_sm",
		SubmitSMResp:         "submit_sm_resp",
		SubmitMultiSM:        "submit_multi_sm",
		SubmitMultiSMResp:    "submit_multi_sm_resp",
		DeliverSM:            "deliver_sm",
		DeliverSMResp:        "deliver_sm_resp",
		DataSM:               "data_sm",
		DataSMResp:           "data_sm_resp",
		QuerySM:              "query_sm",
		QuerySMResp:          "query_sm_resp",
		EnquireLink:          "enquire_link",
		EnquireLinkResp:      "enquire_link_resp",
		CancelSM:             "cancel_sm",
		CancelSMResp:         "cancel_sm_resp",
		ReplaceSM:            "replace_sm",
		ReplaceSMResp:        "replace_sm_resp",
	}
	if name, ok := names[commandID]; ok {
		return name
	}
	return fmt.Sprintf("unknown_0x%08X", commandID)
}

// IsResponse проверяет, является ли команда ответом
func IsResponse(commandID uint32) bool {
	return (commandID & 0x80000000) != 0
}

// GetStatusName возвращает имя статуса по коду
func GetStatusName(status uint32) string {
	names := map[uint32]string{
		ESME_ROK:            "ESME_ROK",
		ESME_RINVMSGLEN:     "ESME_RINVMSGLEN",
		ESME_RINVCMDLEN:     "ESME_RINVCMDLEN",
		ESME_RINVCMDID:      "ESME_RINVCMDID",
		ESME_RINVBNDSTS:     "ESME_RINVBNDSTS",
		ESME_RALYBND:        "ESME_RALYBND",
		ESME_RINVPRTFLG:     "ESME_RINVPRTFLG",
		ESME_RINVREGDLVFLG:  "ESME_RINVREGDLVFLG",
		ESME_RSYSERR:        "ESME_RSYSERR",
		ESME_RINVSRCADR:     "ESME_RINVSRCADR",
		ESME_RINVDSTADR:     "ESME_RINVDSTADR",
		ESME_RINVMSGID:      "ESME_RINVMSGID",
		ESME_RINVDLNAME:     "ESME_RINVDLNAME",
		ESME_RINVPASWD:      "ESME_RINVPASWD",
		ESME_RINVSYSID:      "ESME_RINVSYSID",
		ESME_RCANCELFAIL:    "ESME_RCANCELFAIL",
		ESME_RREPLACEFAIL:   "ESME_RREPLACEFAIL",
		ESME_RMSGQFUL:       "ESME_RMSGQFUL",
		ESME_RINVSERTYP:     "ESME_RINVSERTYP",
		ESME_RINVNUMDESTS:   "ESME_RINVNUMDESTS",
		ESME_RINVDESTFLAG:   "ESME_RINVDESTFLAG",
		ESME_RINVSUBREPFLG:  "ESME_RINVSUBREPFLG",
		ESME_RINVESMCLASS:   "ESME_RINVESMCLASS",
		ESME_RCNTSUBDL:      "ESME_RCNTSUBDL",
		ESME_RSUBMITFAIL:    "ESME_RSUBMITFAIL",
		ESME_RINVSRCTON:     "ESME_RINVSRCTON",
		ESME_RINVSRCNPI:     "ESME_RINVSRCNPI",
		ESME_RINVDSTTON:     "ESME_RINVDSTTON",
		ESME_RINVDSTNPI:     "ESME_RINVDSTNPI",
		ESME_RINVSYSTYP:     "ESME_RINVSYSTYP",
		ESME_RINVREPFLG:     "ESME_RINVREPFLG",
		ESME_RINVNUMMSGS:    "ESME_RINVNUMMSGS",
		ESME_RTHROTTLED:     "ESME_RTHROTTLED",
		ESME_RINVSCHED:      "ESME_RINVSCHED",
		ESME_RINVEXPIRY:     "ESME_RINVEXPIRY",
		ESME_RINVDFTMSGID:   "ESME_RINVDFTMSGID",
		ESME_RX_T_APPN:      "ESME_RX_T_APPN",
		ESME_RX_P_APPN:      "ESME_RX_P_APPN",
		ESME_RX_R_APPN:      "ESME_RX_R_APPN",
		ESME_RQUERYFAIL:     "ESME_RQUERYFAIL",
	}
	if name, ok := names[status]; ok {
		return name
	}
	return fmt.Sprintf("UNKNOWN_0x%08X", status)
}

// FormatSMPPTime форматирует время в формат SMPP (YYMMDDHHmmss)
func FormatSMPPTime(t time.Time) string {
	return t.Format("060102150405")
}

// ParseSMPPTime парсит время из формата SMPP (YYMMDDHHmmss)
func ParseSMPPTime(s string) (time.Time, error) {
	if len(s) == 0 {
		return time.Time{}, nil
	}
	// SMPP формат: YYMMDDHHmmss
	if len(s) != 12 {
		return time.Time{}, fmt.Errorf("invalid SMPP time format: %s", s)
	}
	// Парсим с учетом того, что YY может быть относительным
	layout := "060102150405"
	t, err := time.Parse(layout, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to parse SMPP time: %w", err)
	}
	// Если год меньше 50, считаем что это 20XX, иначе 19XX
	if t.Year() < 50 {
		t = t.AddDate(2000, 0, 0)
	} else {
		t = t.AddDate(1900, 0, 0)
	}
	return t, nil
}

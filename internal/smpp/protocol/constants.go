package protocol

// SMPP версия протокола
const (
	Version = 0x34 // SMPP v3.4
)

// Command ID - типы PDU команд
const (
	// Bind команды
	BindReceiver       = 0x00000001
	BindTransmitter    = 0x00000002
	BindTransceiver    = 0x00000009
	Unbind             = 0x00000006
	UnbindResp         = 0x80000006
	GenericNack        = 0x80000000
	BindReceiverResp   = 0x80000001
	BindTransmitterResp = 0x80000002
	BindTransceiverResp = 0x80000009

	// Submit команды
	SubmitSM    = 0x00000004
	SubmitSMResp = 0x80000004
	SubmitMultiSM = 0x00000021
	SubmitMultiSMResp = 0x80000021

	// Delivery команды
	DeliverSM    = 0x00000005
	DeliverSMResp = 0x80000005
	DataSM       = 0x00000103
	DataSMResp   = 0x80000103

	// Query команды
	QuerySM     = 0x00000003
	QuerySMResp = 0x80000003

	// Enquire команды
	EnquireLink     = 0x00000015
	EnquireLinkResp = 0x80000015

	// Cancel команды
	CancelSM     = 0x00000008
	CancelSMResp = 0x80000008

	// Replace команды
	ReplaceSM     = 0x00000007
	ReplaceSMResp = 0x80000007
)

// Command Status - статусы ответов
const (
	ESME_ROK            = 0x00000000 // No Error
	ESME_RINVMSGLEN     = 0x00000001 // Message Length is invalid
	ESME_RINVCMDLEN     = 0x00000002 // Command Length is invalid
	ESME_RINVCMDID      = 0x00000003 // Invalid Command ID
	ESME_RINVBNDSTS     = 0x00000004 // Incorrect BIND Status for given command
	ESME_RALYBND        = 0x00000005 // ESME Already in Bound State
	ESME_RINVPRTFLG     = 0x00000006 // Invalid Priority Flag
	ESME_RINVREGDLVFLG  = 0x00000007 // Invalid Registered Delivery Flag
	ESME_RSYSERR        = 0x00000008 // System Error
	ESME_RINVSRCADR     = 0x0000000A // Invalid Source Address
	ESME_RINVDSTADR     = 0x0000000B // Invalid Dest Addr
	ESME_RINVMSGID      = 0x0000000C // Message ID is invalid
	ESME_RINVDLNAME     = 0x0000000D // Invalid Distribution List name
	ESME_RINVPASWD      = 0x0000000E // Invalid Password
	ESME_RINVSYSID      = 0x0000000F // Invalid System ID
	ESME_RCANCELFAIL    = 0x00000011 // Cancel SM Failed
	ESME_RREPLACEFAIL   = 0x00000013 // Replace SM Failed
	ESME_RMSGQFUL       = 0x00000014 // Message Queue Full
	ESME_RINVSERTYP     = 0x00000015 // Invalid Service Type
	ESME_RINVNUMDESTS   = 0x00000033 // Invalid number of destinations
	ESME_RINVDESTFLAG   = 0x00000040 // Destination flag is invalid
	ESME_RINVSUBREPFLG  = 0x00000042 // Invalid submit with replace request flag
	ESME_RINVESMCLASS   = 0x00000043 // Invalid esm_class field data
	ESME_RCNTSUBDL      = 0x00000044 // Cannot Submit to Distribution List
	ESME_RSUBMITFAIL    = 0x00000045 // submit_sm or submit_multi failed
	ESME_RINVSRCTON     = 0x00000048 // Invalid Source address TON
	ESME_RINVSRCNPI     = 0x00000049 // Invalid Source address NPI
	ESME_RINVDSTTON     = 0x00000050 // Invalid Destination address TON
	ESME_RINVDSTNPI     = 0x00000051 // Invalid Destination address NPI
	ESME_RINVSYSTYP     = 0x00000053 // Invalid system_type field
	ESME_RINVREPFLG     = 0x00000054 // Invalid replace_if_present flag
	ESME_RINVNUMMSGS    = 0x00000055 // Invalid number of messages
	ESME_RTHROTTLED     = 0x00000058 // Throttling error
	ESME_RINVSCHED      = 0x00000061 // Invalid Scheduled Delivery Time
	ESME_RINVEXPIRY     = 0x00000062 // Invalid message validity period (Expiry time)
	ESME_RINVDFTMSGID   = 0x00000063 // Predefined Message Invalid or Not Found
	ESME_RX_T_APPN      = 0x00000064 // ESME Receiver Temporary App Error Code
	ESME_RX_P_APPN      = 0x00000065 // ESME Receiver Permanent App Error Code
	ESME_RX_R_APPN      = 0x00000066 // ESME Receiver Reject Message Error Code
	ESME_RQUERYFAIL     = 0x00000067 // query_sm request failed
)

// Type of Number (TON)
const (
	TON_UNKNOWN = 0x00
	TON_INTERNATIONAL = 0x01
	TON_NATIONAL = 0x02
	TON_NETWORK_SPECIFIC = 0x03
	TON_SUBSCRIBER_NUMBER = 0x04
	TON_ALPHANUMERIC = 0x05
	TON_ABBREVIATED = 0x06
)

// Numeric Plan Indicator (NPI)
const (
	NPI_UNKNOWN = 0x00
	NPI_ISDN = 0x01
	NPI_DATA = 0x03
	NPI_TELEX = 0x04
	NPI_LAND_MOBILE = 0x06
	NPI_NATIONAL = 0x08
	NPI_PRIVATE = 0x09
	NPI_ERMES = 0x0A
	NPI_INTERNET = 0x0E
	NPI_WAP_CLIENT = 0x12
)

// Data Coding Scheme
const (
	DC_DEFAULT = 0x00 // Default alphabet (GSM 7-bit)
	DC_IA5 = 0x01     // IA5 (CCITT T.50)/ASCII
	DC_OCTET_UNSPECIFIED = 0x02 // Octet unspecified (8-bit binary)
	DC_LATIN1 = 0x03  // Latin 1 (ISO-8859-1)
	DC_OCTET_UNSPECIFIED_COMMON = 0x04 // Octet unspecified (8-bit binary)
	DC_JIS = 0x05     // JIS (X 0208-1990)
	DC_CYRILLIC = 0x06 // Cyrillic (ISO-8859-5)
	DC_HEBREW = 0x07  // Hebrew (ISO-8859-8)
	DC_UCS2 = 0x08    // UCS2 (ISO/IEC-10646)
	DC_PICTOGRAM = 0x09 // Pictogram Encoding
	DC_MUSIC_CODES = 0x0A // ISO-2022-JP (Music Codes)
	DC_KANJI = 0x0D    // Extended Kanji JIS(X 0212-1990)
	DC_KSC5601 = 0x0E  // KS C 5601
)

// ESM Class flags
const (
	ESM_CLASS_DEFAULT = 0x00
	ESM_CLASS_UDHI = 0x40 // User Data Header Indicator
	ESM_CLASS_REPLY_PATH = 0x80 // Reply Path
	ESM_CLASS_UDHI_REPLY_PATH = 0xC0 // UDHI + Reply Path
)

// Registered Delivery flags
const (
	RD_NO_SMSC_DELIVERY_RECEIPT = 0x00
	RD_SMSC_DELIV_RECEIPT_REQUESTED = 0x01
	RD_SMSC_DELIV_RECEIPT_REQUESTED_FOR_FAILURE = 0x02
	RD_SMSC_DELIV_RECEIPT_REQUESTED_FOR_SUCCESS = 0x04
	RD_SMSC_DELIV_RECEIPT_REQUESTED_FOR_BOTH = 0x06
	RD_SME_DELIVERY_ACK_REQUESTED = 0x08
	RD_SME_MANUAL_ACK_REQUESTED = 0x10
	RD_SME_DELIVERY_ACK_REQUESTED_FOR_FAILURE = 0x20
	RD_SME_DELIVERY_ACK_REQUESTED_FOR_SUCCESS = 0x40
	RD_SME_DELIVERY_ACK_REQUESTED_FOR_BOTH = 0x60
)

// Priority Flag
const (
	PRIORITY_FLAG_LEVEL_0 = 0x00
	PRIORITY_FLAG_LEVEL_1 = 0x01
	PRIORITY_FLAG_LEVEL_2 = 0x02
	PRIORITY_FLAG_LEVEL_3 = 0x03
)

// Replace If Present Flag
const (
	REPLACE_IF_PRESENT_NO = 0x00
	REPLACE_IF_PRESENT_YES = 0x01
)

// Protocol ID
const (
	PROTOCOL_ID_DEFAULT = 0x00
)

// Service Type
const (
	SERVICE_TYPE_DEFAULT = ""
)

// Bind Type
const (
	BIND_TYPE_RECEIVER = "receiver"
	BIND_TYPE_TRANSMITTER = "transmitter"
	BIND_TYPE_TRANSCEIVER = "transceiver"
)

// SMPP Message State (используется в query_sm_resp)
const (
	MSG_STATE_ENROUTE      byte = 1 // Message is in enroute state
	MSG_STATE_DELIVERED    byte = 2 // Message is delivered to destination
	MSG_STATE_EXPIRED      byte = 3 // Message validity period has expired
	MSG_STATE_DELETED      byte = 4 // Message has been deleted
	MSG_STATE_UNDELIVERABLE byte = 5 // Message is undeliverable
	MSG_STATE_ACCEPTED     byte = 6 // Message is in accepted state
	MSG_STATE_UNKNOWN      byte = 7 // Message is in invalid state
	MSG_STATE_REJECTED     byte = 8 // Message is in a rejected state
	MSG_STATE_SCHEDULED    byte = 9 // Message is in scheduled state (vendor extension)
)

// PDU Header размер
const (
	PDUHeaderLength = 16 // 4 байта command_length + 4 байта command_id + 4 байта command_status + 4 байта sequence_number
)

// Максимальные длины полей
const (
	MaxSystemIDLength = 16
	MaxPasswordLength = 9
	MaxSystemTypeLength = 13
	MaxServiceTypeLength = 6
	MaxAddressLength = 21
	MaxMessageLength = 254 // для GSM 7-bit, для UCS2 меньше
	MaxSMPPMessageIDLength = 65
)

# SMPP Протокол

## Обзор

Модуль `internal/smpp/protocol` реализует поддержку SMPP (Short Message Peer-to-Peer) протокола версии 3.4. Модуль предоставляет структуры данных, кодирование/декодирование и валидацию PDU (Protocol Data Unit).

## Структура модуля

### Файлы

- `constants.go` - константы протокола (Command ID, статусы, типы адресов и т.д.)
- `pdu.go` - структуры данных для всех типов PDU
- `encoder.go` - кодирование PDU структур в бинарный формат SMPP
- `decoder.go` - декодирование бинарных данных SMPP в PDU структуры
- `validator.go` - валидация PDU структур на соответствие спецификации
- `pdu_helper.go` - вспомогательные функции для работы с PDU

## Поддерживаемые команды

### Bind команды
- `bind_receiver` / `bind_receiver_resp`
- `bind_transmitter` / `bind_transmitter_resp`
- `bind_transceiver` / `bind_transceiver_resp`
- `unbind` / `unbind_resp`

### Submit команды
- `submit_sm` / `submit_sm_resp`
- `submit_multi_sm` / `submit_multi_sm_resp`

### Delivery команды
- `deliver_sm` / `deliver_sm_resp`
- `data_sm` / `data_sm_resp`

### Query команды
- `query_sm` / `query_sm_resp`

### Enquire команды
- `enquire_link` / `enquire_link_resp`

### Cancel команды
- `cancel_sm` / `cancel_sm_resp`

### Replace команды
- `replace_sm` / `replace_sm_resp`

### Другие
- `generic_nack`

## Использование

### Кодирование PDU

```go
import "github.com/smpp-server/smpp-server/internal/smpp/protocol"

// Создание submit_sm PDU
submit := &protocol.SubmitSMPDU{
    ServiceType:     "",
    SourceAddrTON:   protocol.TON_INTERNATIONAL,
    SourceAddrNPI:   protocol.NPI_ISDN,
    SourceAddr:      "1234567890",
    DestAddrTON:     protocol.TON_INTERNATIONAL,
    DestAddrNPI:     protocol.NPI_ISDN,
    DestinationAddr: "0987654321",
    ESMClass:        protocol.ESM_CLASS_DEFAULT,
    ProtocolID:      protocol.PROTOCOL_ID_DEFAULT,
    PriorityFlag:    protocol.PRIORITY_FLAG_LEVEL_0,
    RegisteredDelivery: protocol.RD_SMSC_DELIV_RECEIPT_REQUESTED_FOR_BOTH,
    DataCoding:      protocol.DC_DEFAULT,
    SMLength:        byte(len(message)),
    ShortMessage:    []byte(message),
}

// Кодирование тела PDU
encoder := protocol.NewEncoder()
body, err := encoder.EncodeSubmitSM(submit)
if err != nil {
    return err
}

// Создание полной PDU структуры
pdu := protocol.BuildPDU(
    protocol.SubmitSM,
    protocol.ESME_ROK,
    sequenceNumber,
    body,
)

// Кодирование полной PDU
data, err := encoder.EncodePDU(pdu)
if err != nil {
    return err
}
```

### Декодирование PDU

```go
// Парсинг бинарных данных
pdu, err := protocol.ParsePDU(data)
if err != nil {
    return err
}

// Декодирование тела в зависимости от типа команды
decoder := protocol.NewDecoder(pdu.Body)

switch pdu.CommandID {
case protocol.SubmitSM:
    submit, err := decoder.DecodeSubmitSM(pdu.Body)
    if err != nil {
        return err
    }
    // Использование submit PDU
    
case protocol.SubmitSMResp:
    resp, err := decoder.DecodeSubmitSMResp(pdu.Body)
    if err != nil {
        return err
    }
    // Использование response PDU
}
```

### Валидация PDU

```go
validator := protocol.NewValidator()

// Валидация базовой PDU
if err := validator.ValidatePDU(pdu); err != nil {
    return err
}

// Валидация конкретного типа PDU
if err := validator.ValidateSubmitSM(submit); err != nil {
    return err
}
```

### Создание ответов

```go
// Успешный ответ
response := protocol.BuildSuccessResponse(
    request,
    protocol.SubmitSMResp,
    responseBody,
)

// Ответ с ошибкой
errorResponse := protocol.BuildErrorResponse(
    request,
    protocol.ESME_RINVMSGLEN,
)
```

## Константы

### Command Status

Основные статусы ошибок:
- `ESME_ROK` (0x00000000) - No Error
- `ESME_RINVMSGLEN` (0x00000001) - Message Length is invalid
- `ESME_RINVCMDLEN` (0x00000002) - Command Length is invalid
- `ESME_RINVCMDID` (0x00000003) - Invalid Command ID
- `ESME_RINVBNDSTS` (0x00000004) - Incorrect BIND Status
- `ESME_RINVPASWD` (0x0000000E) - Invalid Password
- `ESME_RINVSYSID` (0x0000000F) - Invalid System ID

### Type of Number (TON)

- `TON_UNKNOWN` (0x00)
- `TON_INTERNATIONAL` (0x01)
- `TON_NATIONAL` (0x02)
- `TON_NETWORK_SPECIFIC` (0x03)
- `TON_SUBSCRIBER_NUMBER` (0x04)
- `TON_ALPHANUMERIC` (0x05)
- `TON_ABBREVIATED` (0x06)

### Numeric Plan Indicator (NPI)

- `NPI_UNKNOWN` (0x00)
- `NPI_ISDN` (0x01)
- `NPI_DATA` (0x03)
- `NPI_INTERNET` (0x0E)
- `NPI_WAP_CLIENT` (0x12)

### Data Coding

- `DC_DEFAULT` (0x00) - Default alphabet (GSM 7-bit)
- `DC_IA5` (0x01) - IA5 (CCITT T.50)/ASCII
- `DC_UCS2` (0x08) - UCS2 (ISO/IEC-10646)
- `DC_LATIN1` (0x03) - Latin 1 (ISO-8859-1)

## Ограничения

### Максимальные длины полей

- System ID: 16 символов
- Password: 9 символов
- System Type: 13 символов
- Service Type: 6 символов
- Address: 21 символ
- Message: 254 байта (для GSM 7-bit)
- SMPP Message ID: 65 символов

## Формат времени SMPP

Время в SMPP форматируется как `YYMMDDHHmmss`:
- YY - год (2 цифры)
- MM - месяц (01-12)
- DD - день (01-31)
- HH - час (00-23)
- mm - минута (00-59)
- ss - секунда (00-59)

Пример: `250101120000` = 2025-01-01 12:00:00

Использование:
```go
import "time"

// Форматирование
t := time.Now()
smppTime := protocol.FormatSMPPTime(t)

// Парсинг
t, err := protocol.ParseSMPPTime("250101120000")
```

## TLV (Tag-Length-Value) параметры

Опциональные параметры передаются в формате TLV:
- Tag (2 байта) - идентификатор параметра
- Length (2 байта) - длина значения
- Value (N байт) - значение параметра

Пример:
```go
tlv := map[uint16][]byte{
    0x0005: []byte("value1"), // receipted_message_id
    0x0424: []byte("value2"), // message_payload
}
```

## Валидация

Валидатор проверяет:
- Длины полей (не превышают максимумы)
- Корректность значений (TON, NPI, статусы)
- Соответствие длин (например, sm_length и short_message)
- Формат данных (C-Octet Strings должны быть null-terminated)

## Обработка ошибок

Все функции кодирования/декодирования возвращают ошибки с описанием проблемы. Рекомендуется всегда проверять ошибки и логировать их для отладки.

## Совместимость

Модуль реализует SMPP версии 3.4 согласно спецификации SMPP v3.4 Issue 1.2.

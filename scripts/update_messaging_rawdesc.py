"""
Adds ListScheduledMessagesRequest and ListScheduledMessagesResponse messages
plus ListScheduledMessages RPC to messaging.pb.go rawDesc.
"""
from google.protobuf import descriptor_pb2

# Read current messaging.pb.go
with open('api/proto/messagingv1/messaging.pb.go', 'r', encoding='utf-8') as f:
    content = f.read()

# Extract rawDesc (look for 'const file_messaging_messaging_proto_rawDesc = "" +')
start = content.find('const file_messaging_messaging_proto_rawDesc = "" +')
end = content.find('\nvar file_messaging_messaging_proto_rawDescOnce', start)
raw_section = content[start:end]

# Parse the Go string literal to bytes (same pattern as update_rawdesc.py)
BACKSLASH = chr(92)
raw_bytes = b''
for line in raw_section.split('\n'):
    line = line.strip()
    if line.startswith('"') and (line.endswith('" +') or line.endswith('"')):
        inner = line[1:]
        if inner.endswith('" +'):
            inner = inner[:-3]
        elif inner.endswith('"'):
            inner = inner[:-1]
        i = 0
        while i < len(inner):
            c = inner[i]
            if c != BACKSLASH:
                raw_bytes += c.encode('latin-1')
                i += 1
            else:
                i += 1
                nc = inner[i]
                if nc == 'n': raw_bytes += b'\n'
                elif nc == 't': raw_bytes += b'\t'
                elif nc == 'r': raw_bytes += b'\r'
                elif nc == 'v': raw_bytes += bytes([11])
                elif nc == 'a': raw_bytes += bytes([7])
                elif nc == 'b': raw_bytes += bytes([8])
                elif nc == 'f': raw_bytes += bytes([12])
                elif nc == BACKSLASH: raw_bytes += bytes([92])
                elif nc == '"': raw_bytes += bytes([34])
                elif nc == 'x':
                    raw_bytes += bytes([int(inner[i+1:i+3], 16)])
                    i += 2
                elif nc.isdigit():
                    raw_bytes += bytes([int(inner[i:i+3], 8)])
                    i += 2
                else:
                    raw_bytes += nc.encode('latin-1')
                i += 1

fd = descriptor_pb2.FileDescriptorProto()
fd.ParseFromString(raw_bytes)
print(f"Parsed OK. {len(fd.message_type)} messages, {len(fd.service)} services")
for i, m in enumerate(fd.message_type):
    print(f"  [{i}] {m.name}")
print(f"Service: {fd.service[0].name}, {len(fd.service[0].method)} methods")

# --- Add ListScheduledMessagesRequest (message index 13) ---
existing_names = {m.name for m in fd.message_type}
if 'ListScheduledMessagesRequest' not in existing_names:
    req = fd.message_type.add()
    req.name = 'ListScheduledMessagesRequest'

    f = req.field.add()
    f.name = 'client_id'; f.number = 1
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    f.json_name = 'clientId'

    f = req.field.add()
    f.name = 'limit'; f.number = 2
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT32
    f.json_name = 'limit'

    f = req.field.add()
    f.name = 'offset'; f.number = 3
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT32
    f.json_name = 'offset'
    print("Added ListScheduledMessagesRequest")

# --- Add ListScheduledMessagesResponse (message index 14) ---
if 'ListScheduledMessagesResponse' not in existing_names:
    resp = fd.message_type.add()
    resp.name = 'ListScheduledMessagesResponse'

    # messages: repeated MessageInfo = 1
    f = resp.field.add()
    f.name = 'messages'; f.number = 1
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_REPEATED
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_MESSAGE
    f.type_name = '.messaging.v1.MessageInfo'
    f.json_name = 'messages'

    f = resp.field.add()
    f.name = 'total'; f.number = 2
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT32
    f.json_name = 'total'

    f = resp.field.add()
    f.name = 'limit'; f.number = 3
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT32
    f.json_name = 'limit'

    f = resp.field.add()
    f.name = 'offset'; f.number = 4
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_INT32
    f.json_name = 'offset'
    print("Added ListScheduledMessagesResponse")

# --- Add ListScheduledMessages RPC to service ---
svc = fd.service[0]
existing_rpcs = {m.name for m in svc.method}
if 'ListScheduledMessages' not in existing_rpcs:
    m = svc.method.add()
    m.name = 'ListScheduledMessages'
    m.input_type = '.messaging.v1.ListScheduledMessagesRequest'
    m.output_type = '.messaging.v1.ListScheduledMessagesResponse'
    print("Added ListScheduledMessages RPC to service")

# Serialize
new_raw = fd.SerializeToString()
print(f"New rawDesc: {len(new_raw)} bytes (was {len(raw_bytes)})")

def bytes_to_go_string(data):
    result = ''
    line = ''
    for b in data:
        if b == 10: line += BACKSLASH + 'n'
        elif b == 9: line += BACKSLASH + 't'
        elif b == 11: line += BACKSLASH + 'v'
        elif b == 7: line += BACKSLASH + 'a'
        elif b == 8: line += BACKSLASH + 'b'
        elif b == 12: line += BACKSLASH + 'f'
        elif b == 13: line += BACKSLASH + 'r'
        elif b == 34: line += BACKSLASH + '"'
        elif b == 92: line += BACKSLASH + BACKSLASH
        elif 32 <= b < 127: line += chr(b)
        else: line += BACKSLASH + 'x' + format(b, '02x')
        if len(line) >= 100:
            result += '\t"' + line + '" +\n'
            line = ''
    if line:
        result += '\t"' + line + '"\n'
    return result

new_go_str = bytes_to_go_string(new_raw)
old_const = content[start:end]
new_const = 'const file_messaging_messaging_proto_rawDesc = "" +\n' + new_go_str
new_content = content[:start] + new_const + content[end:]
print(f"Content size: {len(content)} -> {len(new_content)}")

with open('api/proto/messagingv1/messaging.pb.go', 'w', encoding='utf-8') as f:
    f.write(new_content)
print("Updated rawDesc in messaging.pb.go")

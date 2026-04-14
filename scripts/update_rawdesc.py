"""
Script to update the template.pb.go rawDesc to add missing fields:
- UpdateTemplateRequest: traffic_type (5), sender_name_id (6)
- TemplateInfo: traffic_type (15)
"""
from google.protobuf import descriptor_pb2

with open('api/proto/templatev1/template.pb.go', 'r', encoding='utf-8') as f:
    content = f.read()

start = content.find('const file_template_proto_rawDesc = "" +')
end = content.find('\nfunc file_template_proto_rawDescGZIP', start)
raw_section = content[start:end]

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
            if c != chr(92):
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
                elif nc == chr(92): raw_bytes += bytes([92])
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
print(f"Parsed OK. {len(fd.message_type)} messages")

# --- Patch UpdateTemplateRequest (index 2) ---
update_req = fd.message_type[2]
assert update_req.name == 'UpdateTemplateRequest'

existing = {f.number for f in update_req.field}
print(f"UpdateTemplateRequest existing fields: {sorted(existing)}")

if 5 not in existing:
    f = update_req.field.add()
    f.name = 'traffic_type'
    f.number = 5
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    f.json_name = 'trafficType'
    f.proto3_optional = True
    f.oneof_index = len(update_req.oneof_decl)
    oo = update_req.oneof_decl.add()
    oo.name = '_traffic_type'
    print("  Added traffic_type (5)")

if 6 not in existing:
    f = update_req.field.add()
    f.name = 'sender_name_id'
    f.number = 6
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    f.json_name = 'senderNameId'
    f.proto3_optional = True
    f.oneof_index = len(update_req.oneof_decl)
    oo = update_req.oneof_decl.add()
    oo.name = '_sender_name_id'
    print("  Added sender_name_id (6)")

# --- Patch TemplateInfo (index 18) ---
tinfo = fd.message_type[18]
assert tinfo.name == 'TemplateInfo'

existing_ti = {fld.number for fld in tinfo.field}
print(f"TemplateInfo existing fields: {sorted(existing_ti)}")

if 15 not in existing_ti:
    f = tinfo.field.add()
    f.name = 'traffic_type'
    f.number = 15
    f.label = descriptor_pb2.FieldDescriptorProto.LABEL_OPTIONAL
    f.type = descriptor_pb2.FieldDescriptorProto.TYPE_STRING
    f.json_name = 'trafficType'
    print("  Added traffic_type (15) to TemplateInfo")

new_raw = fd.SerializeToString()
print(f"New rawDesc: {len(new_raw)} bytes (was {len(raw_bytes)})")

BACKSLASH = chr(92)

def bytes_to_go_string(data):
    result = ''
    line = ''
    for b in data:
        if b == 10:
            line += BACKSLASH + 'n'
        elif b == 9:
            line += BACKSLASH + 't'
        elif b == 11:
            line += BACKSLASH + 'v'
        elif b == 7:
            line += BACKSLASH + 'a'
        elif b == 8:
            line += BACKSLASH + 'b'
        elif b == 12:
            line += BACKSLASH + 'f'
        elif b == 13:
            line += BACKSLASH + 'r'
        elif b == 34:
            line += BACKSLASH + '"'
        elif b == 92:
            line += BACKSLASH + BACKSLASH
        elif 32 <= b < 127:
            line += chr(b)
        else:
            line += BACKSLASH + 'x' + format(b, '02x')

        if len(line) >= 100:
            result += '\t"' + line + '" +\n'
            line = ''

    if line:
        result += '\t"' + line + '"\n'
    return result

new_go_str = bytes_to_go_string(new_raw)

# Replace in content
old_const = content[start:end]
new_const = 'const file_template_proto_rawDesc = "" +\n' + new_go_str

new_content = content[:start] + new_const + content[end:]
print(f"Content size: {len(content)} -> {len(new_content)}")

with open('api/proto/templatev1/template.pb.go', 'w', encoding='utf-8') as f:
    f.write(new_content)

print("Updated api/proto/templatev1/template.pb.go")

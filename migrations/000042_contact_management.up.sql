-- migrations/000042_contact_management.up.sql

-- Contact Lists
CREATE TABLE contact_lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    contacts_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_contact_lists_client ON contact_lists(client_id);

-- Contact List Attributes (metadata about custom fields)
CREATE TABLE contact_list_attributes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_list_id UUID NOT NULL REFERENCES contact_lists(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    display_name TEXT NOT NULL,
    type TEXT NOT NULL CHECK (type IN ('string', 'number', 'date', 'boolean')),
    required BOOLEAN NOT NULL DEFAULT false,
    position INT NOT NULL DEFAULT 0,
    UNIQUE (contact_list_id, name)
);

CREATE INDEX idx_cla_list ON contact_list_attributes(contact_list_id);

-- Contacts
CREATE TABLE contacts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_list_id UUID NOT NULL REFERENCES contact_lists(id) ON DELETE CASCADE,
    phone TEXT NOT NULL,
    attributes JSONB NOT NULL DEFAULT '{}',
    tags TEXT[] NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (contact_list_id, phone)
);

CREATE INDEX idx_contacts_list ON contacts(contact_list_id);
CREATE INDEX idx_contacts_list_phone ON contacts(contact_list_id, phone);
CREATE INDEX idx_contacts_attributes_gin ON contacts USING gin(attributes jsonb_path_ops);
CREATE INDEX idx_contacts_tags_gin ON contacts USING gin(tags);

-- Contact Imports
CREATE TABLE contact_imports (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    contact_list_id UUID NOT NULL REFERENCES contact_lists(id) ON DELETE CASCADE,
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    file_name TEXT NOT NULL DEFAULT '',
    file_size BIGINT NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    total_rows INT NOT NULL DEFAULT 0,
    imported_count INT NOT NULL DEFAULT 0,
    updated_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,
    errors JSONB,
    column_mapping JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE INDEX idx_contact_imports_list ON contact_imports(contact_list_id);
CREATE INDEX idx_contact_imports_client ON contact_imports(client_id);

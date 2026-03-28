# Broadcast Campaigns & Contact Management — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Реализовать систему рассылок (campaigns) с контактными базами, сегментацией, A/B тестированием, auto-retry и аналитикой — два новых микросервиса (contact-service, campaign-service), интеграция с gateway и frontend.

**Architecture:** Два новых gRPC-сервиса: contact-service (порт 5012) управляет контактными базами, контактами и импортом; campaign-service (порт 5013) управляет кампаниями, A/B тестами, retry и аналитикой. campaign-service стримит контакты от contact-service через server-side streaming gRPC, публикует сообщения в Kafka `sms.outgoing` и потребляет статусы из `sms.status`. Portal-gateway проксирует HTTP → gRPC. Frontend добавляет страницы контактов и кампаний.

**Tech Stack:** Go 1.24.0, sqlx + lib/pq, gorilla/mux, google.golang.org/grpc, IBM/sarama, zerolog, prometheus, React 19 + Vite + TypeScript + Tailwind CSS

---

## File Structure

### Database Migrations
- Create: `migrations/000042_contact_management.up.sql` — contact_lists, contact_list_attributes, contacts, contact_imports
- Create: `migrations/000042_contact_management.down.sql`
- Create: `migrations/000043_campaigns.up.sql` — campaigns, campaign_variants, campaign_ab_config, campaign_recipients (hash-partitioned), campaign_retry_log, campaign_stats_snapshots
- Create: `migrations/000043_campaigns.down.sql`

### Proto Definitions
- Create: `api/proto/contact/contact.proto` — ContactService definition
- Create: `api/proto/campaign/campaign.proto` — CampaignService definition

### contact-service
- Create: `internal/services/contact/domain/models.go` — domain types: ContactList, Contact, ContactAttribute, ImportJob, SegmentRule
- Create: `internal/services/contact/domain/segment.go` — segmentation rule → SQL translation
- Create: `internal/services/contact/application/contact_service.go` — business logic
- Create: `internal/services/contact/application/import_worker.go` — async CSV/Excel import worker
- Create: `internal/services/contact/infrastructure/repository/contact_list_repository.go`
- Create: `internal/services/contact/infrastructure/repository/contact_repository.go`
- Create: `internal/services/contact/infrastructure/repository/import_repository.go`
- Create: `internal/services/contact/grpc/server.go` — gRPC handlers
- Create: `internal/services/contact/grpc/mappers.go` — proto ↔ domain converters
- Create: `cmd/services/contact-service/main.go` — service entrypoint

### campaign-service
- Create: `internal/services/campaign/domain/models.go` — Campaign, Variant, ABConfig, Recipient, RetryConfig, StatsSnapshot
- Create: `internal/services/campaign/application/campaign_service.go` — CRUD + lifecycle
- Create: `internal/services/campaign/application/materializer.go` — audience materialization via streaming
- Create: `internal/services/campaign/application/sender.go` — batched Kafka publishing with rate control
- Create: `internal/services/campaign/application/status_consumer.go` — sms.status Kafka consumer
- Create: `internal/services/campaign/application/retry_ticker.go` — auto-retry background worker
- Create: `internal/services/campaign/application/stats_ticker.go` — stats snapshot ticker (30s)
- Create: `internal/services/campaign/application/analytics_service.go` — timeline, heatmap, optimal time
- Create: `internal/services/campaign/infrastructure/repository/campaign_repository.go`
- Create: `internal/services/campaign/infrastructure/repository/recipient_repository.go`
- Create: `internal/services/campaign/infrastructure/repository/stats_repository.go`
- Create: `internal/services/campaign/grpc/server.go` — gRPC handlers
- Create: `internal/services/campaign/grpc/mappers.go` — proto ↔ domain converters
- Create: `cmd/services/campaign-service/main.go` — service entrypoint

### Gateway Integration
- Create: `internal/gateway/portal/handlers/contacts.go` — contact list & contact HTTP handlers
- Create: `internal/gateway/portal/handlers/campaigns.go` — campaign HTTP handlers
- Modify: `internal/gateway/portal/clients.go` — add ContactClient, CampaignClient
- Modify: `internal/gateway/portal/router/router.go` — add contact-lists & campaigns routes
- Modify: `cmd/portal-gateway/main.go` — wire new handlers

### Docker & Infrastructure
- Modify: `deployments/docker-compose.yml` — add contact-service, campaign-service containers
- Create: `deployments/docker/contact-service.Dockerfile` (or use service-base.Dockerfile pattern)

### Frontend
- Create: `portal-frontend/src/api/contacts.ts` — contact lists & contacts API client
- Create: `portal-frontend/src/api/campaigns.ts` — campaigns API client
- Create: `portal-frontend/src/pages/contacts/ContactListsPage.tsx` — list of contact lists
- Create: `portal-frontend/src/pages/contacts/ContactListDetailPage.tsx` — contacts table + import
- Create: `portal-frontend/src/pages/contacts/ImportWizardPage.tsx` — CSV upload wizard
- Create: `portal-frontend/src/pages/campaigns/CampaignsPage.tsx` — campaigns list
- Create: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx` — 5-step creation wizard
- Create: `portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx` — stats, timeline, A/B
- Create: `portal-frontend/src/components/SegmentBuilder.tsx` — AND/OR rule builder
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx` — add nav items
- Modify: `portal-frontend/src/App.tsx` — add routes

---

## Task 1: Database Migration — Contact Management

**Files:**
- Create: `migrations/000042_contact_management.up.sql`
- Create: `migrations/000042_contact_management.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
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
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000042_contact_management.down.sql
DROP TABLE IF EXISTS contact_imports;
DROP TABLE IF EXISTS contacts;
DROP TABLE IF EXISTS contact_list_attributes;
DROP TABLE IF EXISTS contact_lists;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000042_contact_management.up.sql migrations/000042_contact_management.down.sql
git commit -m "feat(contacts): add contact management database migrations"
```

---

## Task 2: Database Migration — Campaigns

**Files:**
- Create: `migrations/000043_campaigns.up.sql`
- Create: `migrations/000043_campaigns.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/000043_campaigns.up.sql

-- Campaigns
CREATE TABLE campaigns (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_id UUID NOT NULL REFERENCES clients(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'scheduled', 'materializing', 'running', 'paused', 'completed', 'cancelled')),
    contact_list_id UUID NOT NULL REFERENCES contact_lists(id),
    template_id UUID REFERENCES templates(id),
    source TEXT NOT NULL DEFAULT '',
    segment_rules JSONB,
    segment_tags TEXT[],
    send_rate INT NOT NULL DEFAULT 0,
    scheduled_at TIMESTAMPTZ,
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    retry_config JSONB,
    total_recipients INT NOT NULL DEFAULT 0,
    sent_count INT NOT NULL DEFAULT 0,
    delivered_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_campaigns_client ON campaigns(client_id);
CREATE INDEX idx_campaigns_status ON campaigns(client_id, status);

-- Campaign Variants (A/B testing)
CREATE TABLE campaign_variants (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    template_id UUID REFERENCES templates(id),
    percentage INT NOT NULL DEFAULT 0,
    is_winner BOOLEAN NOT NULL DEFAULT false,
    is_control BOOLEAN NOT NULL DEFAULT false,
    sent_count INT NOT NULL DEFAULT 0,
    delivered_count INT NOT NULL DEFAULT 0,
    failed_count INT NOT NULL DEFAULT 0
);

CREATE INDEX idx_cv_campaign ON campaign_variants(campaign_id);

-- Campaign A/B Config
CREATE TABLE campaign_ab_config (
    campaign_id UUID PRIMARY KEY REFERENCES campaigns(id) ON DELETE CASCADE,
    metric TEXT NOT NULL DEFAULT 'delivery_rate' CHECK (metric IN ('delivery_rate')),
    test_duration_hours INT NOT NULL DEFAULT 1,
    auto_select_winner BOOLEAN NOT NULL DEFAULT false,
    winner_variant_id UUID REFERENCES campaign_variants(id),
    winner_selected_at TIMESTAMPTZ
);

-- Campaign Recipients (hash-partitioned by campaign_id, 16 partitions)
CREATE TABLE campaign_recipients (
    id UUID NOT NULL DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL,
    contact_id UUID NOT NULL,
    phone TEXT NOT NULL,
    variant_id UUID,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'delivered', 'failed', 'retry', 'cancelled')),
    message_id UUID,
    retry_count INT NOT NULL DEFAULT 0,
    last_retry_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
) PARTITION BY HASH (campaign_id);

-- Create 16 hash partitions
CREATE TABLE campaign_recipients_p0 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 0);
CREATE TABLE campaign_recipients_p1 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 1);
CREATE TABLE campaign_recipients_p2 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 2);
CREATE TABLE campaign_recipients_p3 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 3);
CREATE TABLE campaign_recipients_p4 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 4);
CREATE TABLE campaign_recipients_p5 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 5);
CREATE TABLE campaign_recipients_p6 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 6);
CREATE TABLE campaign_recipients_p7 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 7);
CREATE TABLE campaign_recipients_p8 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 8);
CREATE TABLE campaign_recipients_p9 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 9);
CREATE TABLE campaign_recipients_p10 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 10);
CREATE TABLE campaign_recipients_p11 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 11);
CREATE TABLE campaign_recipients_p12 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 12);
CREATE TABLE campaign_recipients_p13 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 13);
CREATE TABLE campaign_recipients_p14 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 14);
CREATE TABLE campaign_recipients_p15 PARTITION OF campaign_recipients FOR VALUES WITH (MODULUS 16, REMAINDER 15);

CREATE INDEX idx_cr_campaign_status ON campaign_recipients(campaign_id, status);
CREATE INDEX idx_cr_campaign_variant ON campaign_recipients(campaign_id, variant_id);
CREATE INDEX idx_cr_message_id ON campaign_recipients(message_id);

-- Campaign Retry Log
CREATE TABLE campaign_retry_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    recipient_id UUID NOT NULL,
    retry_number INT NOT NULL,
    provider_id UUID,
    status TEXT NOT NULL,
    error_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_crl_campaign ON campaign_retry_log(campaign_id);
CREATE INDEX idx_crl_recipient ON campaign_retry_log(recipient_id);

-- Campaign Stats Snapshots
CREATE TABLE campaign_stats_snapshots (
    campaign_id UUID NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
    variant_id UUID,
    snapshot_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent INT NOT NULL DEFAULT 0,
    delivered INT NOT NULL DEFAULT 0,
    failed INT NOT NULL DEFAULT 0,
    pending INT NOT NULL DEFAULT 0,
    avg_delivery_time_ms INT NOT NULL DEFAULT 0,
    cost DECIMAL(12,4) NOT NULL DEFAULT 0,
    PRIMARY KEY (campaign_id, variant_id, snapshot_at)
);

CREATE INDEX idx_css_campaign ON campaign_stats_snapshots(campaign_id, snapshot_at DESC);
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/000043_campaigns.down.sql
DROP TABLE IF EXISTS campaign_stats_snapshots;
DROP TABLE IF EXISTS campaign_retry_log;
DROP TABLE IF EXISTS campaign_recipients_p0, campaign_recipients_p1, campaign_recipients_p2, campaign_recipients_p3,
    campaign_recipients_p4, campaign_recipients_p5, campaign_recipients_p6, campaign_recipients_p7,
    campaign_recipients_p8, campaign_recipients_p9, campaign_recipients_p10, campaign_recipients_p11,
    campaign_recipients_p12, campaign_recipients_p13, campaign_recipients_p14, campaign_recipients_p15;
DROP TABLE IF EXISTS campaign_recipients;
DROP TABLE IF EXISTS campaign_ab_config;
DROP TABLE IF EXISTS campaign_variants;
DROP TABLE IF EXISTS campaigns;
```

- [ ] **Step 3: Commit**

```bash
git add migrations/000043_campaigns.up.sql migrations/000043_campaigns.down.sql
git commit -m "feat(campaigns): add campaign database migrations with hash-partitioned recipients"
```

---

## Task 3: Proto Definition — ContactService

**Files:**
- Create: `api/proto/contact/contact.proto`

- [ ] **Step 1: Write the proto file**

```protobuf
// api/proto/contact/contact.proto
syntax = "proto3";

package contact.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/contactv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/struct.proto";

service ContactService {
  // Contact Lists
  rpc CreateContactList(CreateContactListRequest) returns (ContactList);
  rpc ListContactLists(ListContactListsRequest) returns (ContactListPage);
  rpc GetContactList(GetContactListRequest) returns (ContactList);
  rpc UpdateContactList(UpdateContactListRequest) returns (ContactList);
  rpc DeleteContactList(DeleteContactListRequest) returns (google.protobuf.Empty);

  // Attributes
  rpc SetListAttributes(SetListAttributesRequest) returns (AttributeList);
  rpc GetListAttributes(GetListAttributesRequest) returns (AttributeList);

  // Contacts
  rpc CreateContact(CreateContactRequest) returns (Contact);
  rpc UpdateContact(UpdateContactRequest) returns (Contact);
  rpc DeleteContact(DeleteContactRequest) returns (google.protobuf.Empty);
  rpc ListContacts(ListContactsRequest) returns (ContactPage);
  rpc BatchUpsertContacts(BatchUpsertContactsRequest) returns (BatchResult);

  // Tags
  rpc AddTags(AddTagsRequest) returns (google.protobuf.Empty);
  rpc RemoveTags(RemoveTagsRequest) returns (google.protobuf.Empty);
  rpc ListTags(ListTagsRequest) returns (TagList);

  // Import
  rpc StartImport(StartImportRequest) returns (ImportJob);
  rpc GetImportStatus(GetImportStatusRequest) returns (ImportJob);
  rpc ListImports(ListImportsRequest) returns (ImportJobPage);

  // Segmentation
  rpc PreviewSegment(PreviewSegmentRequest) returns (SegmentPreview);
  rpc StreamSegment(StreamSegmentRequest) returns (stream ContactBatch);
}

// === Contact List ===

message ContactList {
  string id = 1;
  string client_id = 2;
  string name = 3;
  string description = 4;
  int32 contacts_count = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
}

message CreateContactListRequest {
  string client_id = 1;
  string name = 2;
  string description = 3;
}

message ListContactListsRequest {
  string client_id = 1;
  int32 limit = 2;
  int32 offset = 3;
}

message ContactListPage {
  repeated ContactList items = 1;
  int32 total = 2;
}

message GetContactListRequest {
  string id = 1;
  string client_id = 2;
}

message UpdateContactListRequest {
  string id = 1;
  string client_id = 2;
  string name = 3;
  string description = 4;
}

message DeleteContactListRequest {
  string id = 1;
  string client_id = 2;
}

// === Attributes ===

message Attribute {
  string id = 1;
  string name = 2;
  string display_name = 3;
  string type = 4;
  bool required = 5;
  int32 position = 6;
}

message AttributeList {
  repeated Attribute attributes = 1;
}

message SetListAttributesRequest {
  string contact_list_id = 1;
  string client_id = 2;
  repeated Attribute attributes = 3;
}

message GetListAttributesRequest {
  string contact_list_id = 1;
  string client_id = 2;
}

// === Contacts ===

message Contact {
  string id = 1;
  string contact_list_id = 2;
  string phone = 3;
  google.protobuf.Struct attributes = 4;
  repeated string tags = 5;
  google.protobuf.Timestamp created_at = 6;
  google.protobuf.Timestamp updated_at = 7;
}

message CreateContactRequest {
  string contact_list_id = 1;
  string client_id = 2;
  string phone = 3;
  google.protobuf.Struct attributes = 4;
  repeated string tags = 5;
}

message UpdateContactRequest {
  string id = 1;
  string contact_list_id = 2;
  string client_id = 3;
  string phone = 4;
  google.protobuf.Struct attributes = 5;
  repeated string tags = 6;
}

message DeleteContactRequest {
  string id = 1;
  string contact_list_id = 2;
  string client_id = 3;
}

message ListContactsRequest {
  string contact_list_id = 1;
  string client_id = 2;
  int32 limit = 3;
  int32 offset = 4;
  string search = 5;
  repeated string tags = 6;
}

message ContactPage {
  repeated Contact contacts = 1;
  int32 total = 2;
}

message BatchUpsertContactsRequest {
  string contact_list_id = 1;
  string client_id = 2;
  repeated ContactInput contacts = 3;
}

message ContactInput {
  string phone = 1;
  google.protobuf.Struct attributes = 2;
  repeated string tags = 3;
}

message BatchResult {
  int32 created = 1;
  int32 updated = 2;
  int32 errors = 3;
  repeated string error_messages = 4;
}

// === Tags ===

message AddTagsRequest {
  string contact_list_id = 1;
  string client_id = 2;
  repeated string contact_ids = 3;
  repeated string tags = 4;
}

message RemoveTagsRequest {
  string contact_list_id = 1;
  string client_id = 2;
  repeated string contact_ids = 3;
  repeated string tags = 4;
}

message ListTagsRequest {
  string contact_list_id = 1;
  string client_id = 2;
}

message TagList {
  repeated string tags = 1;
}

// === Import ===

message StartImportRequest {
  string contact_list_id = 1;
  string client_id = 2;
  string import_id = 3;
  string file_name = 4;
  int64 file_size = 5;
  string column_mapping = 6; // JSON string
}

message ImportJob {
  string id = 1;
  string contact_list_id = 2;
  string client_id = 3;
  string file_name = 4;
  int64 file_size = 5;
  string status = 6;
  int32 total_rows = 7;
  int32 imported_count = 8;
  int32 updated_count = 9;
  int32 error_count = 10;
  string errors = 11; // JSON string
  string column_mapping = 12; // JSON string
  google.protobuf.Timestamp created_at = 13;
  google.protobuf.Timestamp completed_at = 14;
}

message GetImportStatusRequest {
  string id = 1;
  string contact_list_id = 2;
  string client_id = 3;
}

message ListImportsRequest {
  string contact_list_id = 1;
  string client_id = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message ImportJobPage {
  repeated ImportJob imports = 1;
  int32 total = 2;
}

// === Segmentation ===

message SegmentRule {
  string operator = 1; // "AND" or "OR"
  repeated SegmentCondition conditions = 2;
  repeated SegmentRule nested = 3;
}

message SegmentCondition {
  string field = 1;
  string op = 2; // eq, neq, gt, gte, lt, lte, contains, starts_with, ends_with, in, not_in, between, is_empty, is_not_empty
  string value = 3;
  string value2 = 4; // for "between"
}

message PreviewSegmentRequest {
  string contact_list_id = 1;
  string client_id = 2;
  SegmentRule rules = 3;
  repeated string tags = 4;
}

message SegmentPreview {
  int32 count = 1;
}

message StreamSegmentRequest {
  string contact_list_id = 1;
  string client_id = 2;
  SegmentRule rules = 3;
  repeated string tags = 4;
}

message ContactBatch {
  repeated Contact contacts = 1;
}
```

- [ ] **Step 2: Generate Go code from proto**

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  api/proto/contact/contact.proto
```

If protoc is not available or generates into wrong path, manually create the output directory:
```bash
mkdir -p api/proto/contactv1
```
Then run protoc targeting that directory, or copy generated files there.

- [ ] **Step 3: Commit**

```bash
git add api/proto/contact/ api/proto/contactv1/
git commit -m "feat(contacts): add ContactService proto definition"
```

---

## Task 4: Proto Definition — CampaignService

**Files:**
- Create: `api/proto/campaign/campaign.proto`

- [ ] **Step 1: Write the proto file**

```protobuf
// api/proto/campaign/campaign.proto
syntax = "proto3";

package campaign.v1;

option go_package = "github.com/smpp-server/smpp-server/api/proto/campaignv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";
import "api/proto/contact/contact.proto";

service CampaignService {
  // CRUD
  rpc CreateCampaign(CreateCampaignRequest) returns (Campaign);
  rpc GetCampaign(GetCampaignRequest) returns (Campaign);
  rpc ListCampaigns(ListCampaignsRequest) returns (CampaignPage);
  rpc UpdateCampaign(UpdateCampaignRequest) returns (Campaign);
  rpc DeleteCampaign(DeleteCampaignRequest) returns (google.protobuf.Empty);

  // Lifecycle
  rpc LaunchCampaign(LaunchCampaignRequest) returns (Campaign);
  rpc PauseCampaign(PauseCampaignRequest) returns (Campaign);
  rpc ResumeCampaign(ResumeCampaignRequest) returns (Campaign);
  rpc CancelCampaign(CancelCampaignRequest) returns (Campaign);

  // A/B
  rpc SetVariants(SetVariantsRequest) returns (VariantList);
  rpc SetABConfig(SetABConfigRequest) returns (ABConfig);
  rpc SelectWinner(SelectWinnerRequest) returns (Campaign);

  // Retry
  rpc SetRetryConfig(SetRetryConfigRequest) returns (RetryConfig);
  rpc RetryFailed(RetryFailedRequest) returns (Campaign);

  // Analytics
  rpc GetCampaignStats(GetCampaignStatsRequest) returns (CampaignStats);
  rpc GetCampaignTimeline(GetCampaignTimelineRequest) returns (TimelineData);
  rpc GetVariantComparison(GetVariantComparisonRequest) returns (VariantComparisonData);
  rpc GetDeliveryHeatmap(GetDeliveryHeatmapRequest) returns (HeatmapData);
  rpc GetOptimalSendTime(GetOptimalSendTimeRequest) returns (SendTimeRecommendation);
  rpc ExportReport(ExportReportRequest) returns (ReportFile);
}

// === Campaign ===

message Campaign {
  string id = 1;
  string client_id = 2;
  string name = 3;
  string status = 4;
  string contact_list_id = 5;
  string template_id = 6;
  string source = 7;
  string segment_rules = 8; // JSON
  repeated string segment_tags = 9;
  int32 send_rate = 10;
  google.protobuf.Timestamp scheduled_at = 11;
  google.protobuf.Timestamp started_at = 12;
  google.protobuf.Timestamp completed_at = 13;
  string retry_config = 14; // JSON
  int32 total_recipients = 15;
  int32 sent_count = 16;
  int32 delivered_count = 17;
  int32 failed_count = 18;
  google.protobuf.Timestamp created_at = 19;
  google.protobuf.Timestamp updated_at = 20;
  repeated Variant variants = 21;
  ABConfig ab_config = 22;
}

message CreateCampaignRequest {
  string client_id = 1;
  string name = 2;
  string contact_list_id = 3;
  string template_id = 4;
  string source = 5;
  string segment_rules = 6; // JSON
  repeated string segment_tags = 7;
  int32 send_rate = 8;
  google.protobuf.Timestamp scheduled_at = 9;
}

message GetCampaignRequest {
  string id = 1;
  string client_id = 2;
}

message ListCampaignsRequest {
  string client_id = 1;
  string status = 2;
  int32 limit = 3;
  int32 offset = 4;
}

message CampaignPage {
  repeated Campaign campaigns = 1;
  int32 total = 2;
}

message UpdateCampaignRequest {
  string id = 1;
  string client_id = 2;
  string name = 3;
  string contact_list_id = 4;
  string template_id = 5;
  string source = 6;
  string segment_rules = 7; // JSON
  repeated string segment_tags = 8;
  int32 send_rate = 9;
  google.protobuf.Timestamp scheduled_at = 10;
}

message DeleteCampaignRequest {
  string id = 1;
  string client_id = 2;
}

message LaunchCampaignRequest {
  string id = 1;
  string client_id = 2;
}

message PauseCampaignRequest {
  string id = 1;
  string client_id = 2;
}

message ResumeCampaignRequest {
  string id = 1;
  string client_id = 2;
}

message CancelCampaignRequest {
  string id = 1;
  string client_id = 2;
}

// === Variants ===

message Variant {
  string id = 1;
  string campaign_id = 2;
  string name = 3;
  string template_id = 4;
  int32 percentage = 5;
  bool is_winner = 6;
  bool is_control = 7;
  int32 sent_count = 8;
  int32 delivered_count = 9;
  int32 failed_count = 10;
}

message VariantList {
  repeated Variant variants = 1;
}

message SetVariantsRequest {
  string campaign_id = 1;
  string client_id = 2;
  repeated VariantInput variants = 3;
}

message VariantInput {
  string name = 1;
  string template_id = 2;
  int32 percentage = 3;
  bool is_control = 4;
}

// === A/B Config ===

message ABConfig {
  string campaign_id = 1;
  string metric = 2;
  int32 test_duration_hours = 3;
  bool auto_select_winner = 4;
  string winner_variant_id = 5;
  google.protobuf.Timestamp winner_selected_at = 6;
}

message SetABConfigRequest {
  string campaign_id = 1;
  string client_id = 2;
  string metric = 3;
  int32 test_duration_hours = 4;
  bool auto_select_winner = 5;
}

message SelectWinnerRequest {
  string campaign_id = 1;
  string client_id = 2;
  string variant_id = 3;
}

// === Retry ===

message RetryConfig {
  bool enabled = 1;
  int32 delay_hours = 2;
  int32 max_retries = 3;
  string alternative_template_id = 4;
}

message SetRetryConfigRequest {
  string campaign_id = 1;
  string client_id = 2;
  RetryConfig config = 3;
}

message RetryFailedRequest {
  string campaign_id = 1;
  string client_id = 2;
  string alternative_template_id = 3;
}

// === Analytics ===

message GetCampaignStatsRequest {
  string campaign_id = 1;
  string client_id = 2;
}

message CampaignStats {
  int32 total_recipients = 1;
  int32 sent = 2;
  int32 delivered = 3;
  int32 failed = 4;
  int32 pending = 5;
  int32 retry = 6;
  double delivery_rate = 7;
  double total_cost = 8;
  int32 avg_delivery_time_ms = 9;
  repeated VariantStats per_variant = 10;
}

message VariantStats {
  string variant_id = 1;
  string variant_name = 2;
  int32 sent = 3;
  int32 delivered = 4;
  int32 failed = 5;
  double delivery_rate = 6;
  bool is_winner = 7;
}

message GetCampaignTimelineRequest {
  string campaign_id = 1;
  string client_id = 2;
  string interval = 3; // 1min, 5min, 1hour
  string metric = 4; // sent, delivered, failed
}

message TimelineData {
  repeated TimelinePoint points = 1;
}

message TimelinePoint {
  google.protobuf.Timestamp timestamp = 1;
  int32 value = 2;
}

message GetVariantComparisonRequest {
  string campaign_id = 1;
  string client_id = 2;
}

message VariantComparisonData {
  repeated VariantComparisonRow rows = 1;
  string winner_variant_id = 2;
}

message VariantComparisonRow {
  string variant_id = 1;
  string variant_name = 2;
  int32 audience_size = 3;
  int32 sent = 4;
  int32 delivered = 5;
  int32 failed = 6;
  double delivery_rate = 7;
  int32 avg_delivery_time_ms = 8;
  double cost = 9;
}

message GetDeliveryHeatmapRequest {
  string campaign_id = 1;
  string client_id = 2;
}

message HeatmapData {
  repeated HeatmapCell cells = 1;
}

message HeatmapCell {
  int32 day_of_week = 1; // 0-6
  int32 hour = 2; // 0-23
  int32 delivered_count = 3;
  double delivery_rate = 4;
}

message GetOptimalSendTimeRequest {
  string client_id = 1;
}

message SendTimeRecommendation {
  repeated TimeSlot slots = 1;
}

message TimeSlot {
  int32 day_of_week = 1;
  int32 hour = 2;
  double delivery_rate = 3;
  int32 avg_delivery_time_ms = 4;
  double score = 5;
}

message ExportReportRequest {
  string campaign_id = 1;
  string client_id = 2;
  string format = 3; // csv, pdf
}

message ReportFile {
  bytes data = 1;
  string filename = 2;
  string content_type = 3;
}
```

- [ ] **Step 2: Generate Go code from proto**

```bash
protoc --go_out=. --go_opt=paths=source_relative \
  --go-grpc_out=. --go-grpc_opt=paths=source_relative \
  -I. \
  api/proto/campaign/campaign.proto
```

Create output directory if needed:
```bash
mkdir -p api/proto/campaignv1
```

- [ ] **Step 3: Commit**

```bash
git add api/proto/campaign/ api/proto/campaignv1/
git commit -m "feat(campaigns): add CampaignService proto definition"
```

---

## Task 5: contact-service — Domain Models

**Files:**
- Create: `internal/services/contact/domain/models.go`

- [ ] **Step 1: Write domain models**

```go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrContactListNotFound   = errors.New("contact list not found")
	ErrContactNotFound       = errors.New("contact not found")
	ErrImportNotFound        = errors.New("import not found")
	ErrDuplicatePhone        = errors.New("contact with this phone already exists")
	ErrDuplicateListName     = errors.New("contact list with this name already exists")
	ErrInvalidPhone          = errors.New("invalid phone number")
	ErrInvalidSegmentRules   = errors.New("invalid segment rules")
	ErrSegmentDepthExceeded  = errors.New("segment rule nesting exceeds maximum depth of 3")
	ErrImportAlreadyStarted  = errors.New("import already started")
)

// ContactList represents a contact database owned by a client.
type ContactList struct {
	ID            uuid.UUID
	ClientID      uuid.UUID
	Name          string
	Description   string
	ContactsCount int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ContactAttribute describes a custom field in a contact list.
type ContactAttribute struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	Name          string
	DisplayName   string
	Type          string // string, number, date, boolean
	Required      bool
	Position      int
}

// Contact represents a single contact in a list.
type Contact struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	Phone         string
	Attributes    map[string]interface{}
	Tags          []string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// ImportJob represents a CSV/Excel import operation.
type ImportJob struct {
	ID            uuid.UUID
	ContactListID uuid.UUID
	ClientID      uuid.UUID
	FileName      string
	FileSize      int64
	Status        string // pending, processing, completed, failed
	TotalRows     int
	ImportedCount int
	UpdatedCount  int
	ErrorCount    int
	Errors        []ImportError
	ColumnMapping *ColumnMapping
	CreatedAt     time.Time
	CompletedAt   *time.Time
}

// ImportError describes a single row error during import.
type ImportError struct {
	Row    int    `json:"row"`
	Reason string `json:"reason"`
}

// ColumnMapping describes how CSV columns map to contact attributes.
type ColumnMapping struct {
	Columns   map[string]ColumnTarget `json:"columns"`
	HasHeader bool                    `json:"has_header"`
	Delimiter string                  `json:"delimiter"`
}

// ColumnTarget describes what a single CSV column maps to.
type ColumnTarget struct {
	Target string `json:"target"` // attribute name or "phone"
	Type   string `json:"type"`   // phone, string, number, date, boolean
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/contact/domain/models.go
git commit -m "feat(contacts): add contact domain models"
```

---

## Task 6: contact-service — Segmentation Engine

**Files:**
- Create: `internal/services/contact/domain/segment.go`

- [ ] **Step 1: Write segmentation rule → SQL translator**

```go
package domain

import (
	"fmt"
	"strings"
)

// SegmentRule represents an AND/OR tree of conditions.
type SegmentRule struct {
	Operator   string             `json:"operator"` // "AND" or "OR"
	Conditions []SegmentCondition `json:"conditions"`
	Nested     []SegmentRule      `json:"nested"`
}

// SegmentCondition represents a single filter condition on an attribute.
type SegmentCondition struct {
	Field  string      `json:"field"`
	Op     string      `json:"op"`
	Value  interface{} `json:"value"`
	Value2 interface{} `json:"value2,omitempty"` // for "between"
}

// SegmentToSQL translates a SegmentRule into a parameterized WHERE clause.
// Returns the clause (without leading "WHERE") and a list of parameters.
// contactListID is always $1; params start at $2.
func SegmentToSQL(rule *SegmentRule, tags []string) (string, []interface{}, error) {
	if rule == nil && len(tags) == 0 {
		return "", nil, nil
	}

	params := []interface{}{}
	paramIdx := 2 // $1 = contact_list_id

	var parts []string

	if rule != nil {
		clause, p, idx, err := ruleToSQL(rule, paramIdx, 1)
		if err != nil {
			return "", nil, err
		}
		if clause != "" {
			parts = append(parts, clause)
		}
		params = append(params, p...)
		paramIdx = idx
	}

	if len(tags) > 0 {
		parts = append(parts, fmt.Sprintf("tags && $%d", paramIdx))
		params = append(params, tags)
		paramIdx++
	}

	if len(parts) == 0 {
		return "", nil, nil
	}

	return strings.Join(parts, " AND "), params, nil
}

func ruleToSQL(rule *SegmentRule, paramIdx int, depth int) (string, []interface{}, int, error) {
	if depth > 3 {
		return "", nil, paramIdx, ErrSegmentDepthExceeded
	}

	op := strings.ToUpper(rule.Operator)
	if op != "AND" && op != "OR" {
		return "", nil, paramIdx, ErrInvalidSegmentRules
	}

	var clauses []string
	var params []interface{}

	for _, cond := range rule.Conditions {
		clause, p, nextIdx, err := conditionToSQL(&cond, paramIdx)
		if err != nil {
			return "", nil, paramIdx, err
		}
		clauses = append(clauses, clause)
		params = append(params, p...)
		paramIdx = nextIdx
	}

	for _, nested := range rule.Nested {
		clause, p, nextIdx, err := ruleToSQL(&nested, paramIdx, depth+1)
		if err != nil {
			return "", nil, paramIdx, err
		}
		clauses = append(clauses, "("+clause+")")
		params = append(params, p...)
		paramIdx = nextIdx
	}

	if len(clauses) == 0 {
		return "", nil, paramIdx, nil
	}

	joiner := " " + op + " "
	return strings.Join(clauses, joiner), params, paramIdx, nil
}

func conditionToSQL(cond *SegmentCondition, paramIdx int) (string, []interface{}, int, error) {
	field := cond.Field
	jsonAccess := fmt.Sprintf("attributes->>'%s'", field)

	switch cond.Op {
	case "eq":
		return fmt.Sprintf("%s = $%d", jsonAccess, paramIdx), []interface{}{fmt.Sprint(cond.Value)}, paramIdx + 1, nil
	case "neq":
		return fmt.Sprintf("(%s IS NULL OR %s != $%d)", jsonAccess, jsonAccess, paramIdx), []interface{}{fmt.Sprint(cond.Value)}, paramIdx + 1, nil
	case "gt":
		return fmt.Sprintf("(%s)::numeric > $%d", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "gte":
		return fmt.Sprintf("(%s)::numeric >= $%d", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "lt":
		return fmt.Sprintf("(%s)::numeric < $%d", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "lte":
		return fmt.Sprintf("(%s)::numeric <= $%d", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "contains":
		return fmt.Sprintf("%s ILIKE $%d", jsonAccess, paramIdx), []interface{}{"%" + fmt.Sprint(cond.Value) + "%"}, paramIdx + 1, nil
	case "starts_with":
		return fmt.Sprintf("%s ILIKE $%d", jsonAccess, paramIdx), []interface{}{fmt.Sprint(cond.Value) + "%"}, paramIdx + 1, nil
	case "ends_with":
		return fmt.Sprintf("%s ILIKE $%d", jsonAccess, paramIdx), []interface{}{"%" + fmt.Sprint(cond.Value)}, paramIdx + 1, nil
	case "in":
		// Value should be a comma-separated list or array
		return fmt.Sprintf("%s = ANY($%d)", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "not_in":
		return fmt.Sprintf("NOT (%s = ANY($%d))", jsonAccess, paramIdx), []interface{}{cond.Value}, paramIdx + 1, nil
	case "between":
		clause := fmt.Sprintf("(%s)::numeric BETWEEN $%d AND $%d", jsonAccess, paramIdx, paramIdx+1)
		return clause, []interface{}{cond.Value, cond.Value2}, paramIdx + 2, nil
	case "is_empty":
		return fmt.Sprintf("(%s IS NULL OR %s = '')", jsonAccess, jsonAccess), nil, paramIdx, nil
	case "is_not_empty":
		return fmt.Sprintf("(%s IS NOT NULL AND %s != '')", jsonAccess, jsonAccess), nil, paramIdx, nil
	default:
		return "", nil, paramIdx, fmt.Errorf("%w: unknown operator %s", ErrInvalidSegmentRules, cond.Op)
	}
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/contact/domain/segment.go
git commit -m "feat(contacts): add segmentation rule-to-SQL translator"
```

---

## Task 7: contact-service — Repositories

**Files:**
- Create: `internal/services/contact/infrastructure/repository/contact_list_repository.go`
- Create: `internal/services/contact/infrastructure/repository/contact_repository.go`
- Create: `internal/services/contact/infrastructure/repository/import_repository.go`

- [ ] **Step 1: Write ContactListRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type ContactListRepository struct {
	db *sqlx.DB
}

func NewContactListRepository(db *sqlx.DB) *ContactListRepository {
	return &ContactListRepository{db: db}
}

type contactListRow struct {
	ID            uuid.UUID    `db:"id"`
	ClientID      uuid.UUID    `db:"client_id"`
	Name          string       `db:"name"`
	Description   string       `db:"description"`
	ContactsCount int          `db:"contacts_count"`
	CreatedAt     sql.NullTime `db:"created_at"`
	UpdatedAt     sql.NullTime `db:"updated_at"`
}

func (r *contactListRow) toDomain() *domain.ContactList {
	cl := &domain.ContactList{
		ID:            r.ID,
		ClientID:      r.ClientID,
		Name:          r.Name,
		Description:   r.Description,
		ContactsCount: r.ContactsCount,
	}
	if r.CreatedAt.Valid {
		cl.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		cl.UpdatedAt = r.UpdatedAt.Time
	}
	return cl
}

func (r *ContactListRepository) Create(ctx context.Context, cl *domain.ContactList) (*domain.ContactList, error) {
	query := `INSERT INTO contact_lists (id, client_id, name, description)
		VALUES ($1, $2, $3, $4)
		RETURNING id, client_id, name, description, contacts_count, created_at, updated_at`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, cl.ID, cl.ClientID, cl.Name, cl.Description).StructScan(&row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicateListName
		}
		return nil, fmt.Errorf("failed to create contact list: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactListRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error) {
	query := `SELECT id, client_id, name, description, contacts_count, created_at, updated_at
		FROM contact_lists WHERE id = $1 AND client_id = $2`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactListNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact list: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactListRepository) List(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contact_lists WHERE client_id = $1`, clientID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count contact lists: %w", err)
	}

	query := `SELECT id, client_id, name, description, contacts_count, created_at, updated_at
		FROM contact_lists WHERE client_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, clientID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list contact lists: %w", err)
	}
	defer rows.Close()

	var lists []*domain.ContactList
	for rows.Next() {
		var row contactListRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan contact list: %w", err)
		}
		lists = append(lists, row.toDomain())
	}
	return lists, total, nil
}

func (r *ContactListRepository) Update(ctx context.Context, cl *domain.ContactList) (*domain.ContactList, error) {
	query := `UPDATE contact_lists SET name = $1, description = $2, updated_at = now()
		WHERE id = $3 AND client_id = $4
		RETURNING id, client_id, name, description, contacts_count, created_at, updated_at`

	var row contactListRow
	err := r.db.QueryRowxContext(ctx, query, cl.Name, cl.Description, cl.ID, cl.ClientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactListNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update contact list: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactListRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM contact_lists WHERE id = $1 AND client_id = $2`, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete contact list: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrContactListNotFound
	}
	return nil
}

func (r *ContactListRepository) UpdateContactsCount(ctx context.Context, listID uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE contact_lists SET contacts_count = (SELECT COUNT(*) FROM contacts WHERE contact_list_id = $1), updated_at = now() WHERE id = $1`,
		listID)
	return err
}

// === Attributes ===

type attributeRow struct {
	ID            uuid.UUID `db:"id"`
	ContactListID uuid.UUID `db:"contact_list_id"`
	Name          string    `db:"name"`
	DisplayName   string    `db:"display_name"`
	Type          string    `db:"type"`
	Required      bool      `db:"required"`
	Position      int       `db:"position"`
}

func (r *attributeRow) toDomain() *domain.ContactAttribute {
	return &domain.ContactAttribute{
		ID:            r.ID,
		ContactListID: r.ContactListID,
		Name:          r.Name,
		DisplayName:   r.DisplayName,
		Type:          r.Type,
		Required:      r.Required,
		Position:      r.Position,
	}
}

func (r *ContactListRepository) SetAttributes(ctx context.Context, listID, clientID uuid.UUID, attrs []*domain.ContactAttribute) ([]*domain.ContactAttribute, error) {
	// Verify ownership
	if _, err := r.GetByID(ctx, listID, clientID); err != nil {
		return nil, err
	}

	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete existing attributes
	if _, err := tx.ExecContext(ctx, `DELETE FROM contact_list_attributes WHERE contact_list_id = $1`, listID); err != nil {
		return nil, fmt.Errorf("failed to delete old attributes: %w", err)
	}

	// Insert new attributes
	var result []*domain.ContactAttribute
	for _, attr := range attrs {
		attr.ID = uuid.New()
		attr.ContactListID = listID
		query := `INSERT INTO contact_list_attributes (id, contact_list_id, name, display_name, type, required, position)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id, contact_list_id, name, display_name, type, required, position`
		var row attributeRow
		if err := tx.QueryRowxContext(ctx, query, attr.ID, listID, attr.Name, attr.DisplayName, attr.Type, attr.Required, attr.Position).StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to insert attribute: %w", err)
		}
		result = append(result, row.toDomain())
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit: %w", err)
	}
	return result, nil
}

func (r *ContactListRepository) GetAttributes(ctx context.Context, listID, clientID uuid.UUID) ([]*domain.ContactAttribute, error) {
	if _, err := r.GetByID(ctx, listID, clientID); err != nil {
		return nil, err
	}

	query := `SELECT id, contact_list_id, name, display_name, type, required, position
		FROM contact_list_attributes WHERE contact_list_id = $1 ORDER BY position`

	rows, err := r.db.QueryxContext(ctx, query, listID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attributes: %w", err)
	}
	defer rows.Close()

	var attrs []*domain.ContactAttribute
	for rows.Next() {
		var row attributeRow
		if err := rows.StructScan(&row); err != nil {
			return nil, fmt.Errorf("failed to scan attribute: %w", err)
		}
		attrs = append(attrs, row.toDomain())
	}
	return attrs, nil
}
```

- [ ] **Step 2: Write ContactRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type ContactRepository struct {
	db *sqlx.DB
}

func NewContactRepository(db *sqlx.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

type contactRow struct {
	ID            uuid.UUID      `db:"id"`
	ContactListID uuid.UUID      `db:"contact_list_id"`
	Phone         string         `db:"phone"`
	Attributes    []byte         `db:"attributes"`
	Tags          pq.StringArray `db:"tags"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	UpdatedAt     sql.NullTime   `db:"updated_at"`
}

func (r *contactRow) toDomain() *domain.Contact {
	c := &domain.Contact{
		ID:            r.ID,
		ContactListID: r.ContactListID,
		Phone:         r.Phone,
		Tags:          []string(r.Tags),
	}
	if r.Attributes != nil {
		json.Unmarshal(r.Attributes, &c.Attributes)
	}
	if c.Attributes == nil {
		c.Attributes = make(map[string]interface{})
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		c.UpdatedAt = r.UpdatedAt.Time
	}
	return c
}

func (r *ContactRepository) Create(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	attrsJSON, _ := json.Marshal(c.Attributes)
	query := `INSERT INTO contacts (id, contact_list_id, phone, attributes, tags)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, contact_list_id, phone, attributes, tags, created_at, updated_at`

	var row contactRow
	err := r.db.QueryRowxContext(ctx, query, c.ID, c.ContactListID, c.Phone, attrsJSON, pq.StringArray(c.Tags)).StructScan(&row)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return nil, domain.ErrDuplicatePhone
		}
		return nil, fmt.Errorf("failed to create contact: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactRepository) GetByID(ctx context.Context, id, listID uuid.UUID) (*domain.Contact, error) {
	query := `SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at
		FROM contacts WHERE id = $1 AND contact_list_id = $2`

	var row contactRow
	err := r.db.QueryRowxContext(ctx, query, id, listID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactRepository) List(ctx context.Context, listID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error) {
	countQuery := `SELECT COUNT(*) FROM contacts WHERE contact_list_id = $1`
	listQuery := `SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at FROM contacts WHERE contact_list_id = $1`
	args := []interface{}{listID}
	argIdx := 2

	if search != "" {
		clause := fmt.Sprintf(" AND phone ILIKE $%d", argIdx)
		countQuery += clause
		listQuery += clause
		args = append(args, "%"+search+"%")
		argIdx++
	}

	if len(tags) > 0 {
		clause := fmt.Sprintf(" AND tags && $%d", argIdx)
		countQuery += clause
		listQuery += clause
		args = append(args, pq.StringArray(tags))
		argIdx++
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("failed to count contacts: %w", err)
	}

	listQuery += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list contacts: %w", err)
	}
	defer rows.Close()

	var contacts []*domain.Contact
	for rows.Next() {
		var row contactRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, fmt.Errorf("failed to scan contact: %w", err)
		}
		contacts = append(contacts, row.toDomain())
	}
	return contacts, total, nil
}

func (r *ContactRepository) Update(ctx context.Context, c *domain.Contact) (*domain.Contact, error) {
	attrsJSON, _ := json.Marshal(c.Attributes)
	query := `UPDATE contacts SET phone = $1, attributes = $2, tags = $3, updated_at = now()
		WHERE id = $4 AND contact_list_id = $5
		RETURNING id, contact_list_id, phone, attributes, tags, created_at, updated_at`

	var row contactRow
	err := r.db.QueryRowxContext(ctx, query, c.Phone, attrsJSON, pq.StringArray(c.Tags), c.ID, c.ContactListID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrContactNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update contact: %w", err)
	}
	return row.toDomain(), nil
}

func (r *ContactRepository) Delete(ctx context.Context, id, listID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM contacts WHERE id = $1 AND contact_list_id = $2`, id, listID)
	if err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrContactNotFound
	}
	return nil
}

func (r *ContactRepository) BatchUpsert(ctx context.Context, listID uuid.UUID, contacts []*domain.Contact) (created, updated, errCount int, errMsgs []string) {
	for _, c := range contacts {
		attrsJSON, _ := json.Marshal(c.Attributes)
		query := `INSERT INTO contacts (id, contact_list_id, phone, attributes, tags)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (contact_list_id, phone) DO UPDATE SET attributes = $4, tags = $5, updated_at = now()
			RETURNING (xmax = 0) AS is_insert`

		var isInsert bool
		err := r.db.QueryRowContext(ctx, query, uuid.New(), listID, c.Phone, attrsJSON, pq.StringArray(c.Tags)).Scan(&isInsert)
		if err != nil {
			errCount++
			errMsgs = append(errMsgs, fmt.Sprintf("phone %s: %v", c.Phone, err))
			continue
		}
		if isInsert {
			created++
		} else {
			updated++
		}
	}
	return
}

// AddTags adds tags to specified contacts
func (r *ContactRepository) AddTags(ctx context.Context, listID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	query := `UPDATE contacts SET tags = array_cat(tags, $1), updated_at = now()
		WHERE contact_list_id = $2 AND id = ANY($3)`
	_, err := r.db.ExecContext(ctx, query, pq.StringArray(tags), listID, pq.Array(contactIDs))
	return err
}

// RemoveTags removes tags from specified contacts
func (r *ContactRepository) RemoveTags(ctx context.Context, listID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	query := `UPDATE contacts SET tags = array_remove_all(tags, $1), updated_at = now()
		WHERE contact_list_id = $2 AND id = ANY($3)`
	// array_remove works one element at a time, so loop
	for _, tag := range tags {
		if _, err := r.db.ExecContext(ctx, `UPDATE contacts SET tags = array_remove(tags, $1), updated_at = now() WHERE contact_list_id = $2 AND id = ANY($3)`, tag, listID, pq.Array(contactIDs)); err != nil {
			return err
		}
	}
	return nil
}

// ListTags returns unique tags for a contact list
func (r *ContactRepository) ListTags(ctx context.Context, listID uuid.UUID) ([]string, error) {
	query := `SELECT DISTINCT unnest(tags) AS tag FROM contacts WHERE contact_list_id = $1 ORDER BY tag`
	var tags []string
	err := r.db.SelectContext(ctx, &tags, query, listID)
	return tags, err
}

// StreamSegment returns contacts matching segment rules, in batches.
func (r *ContactRepository) StreamSegment(ctx context.Context, listID uuid.UUID, rules *domain.SegmentRule, tags []string, batchSize int, callback func([]*domain.Contact) error) error {
	whereClause, params, err := domain.SegmentToSQL(rules, tags)
	if err != nil {
		return err
	}

	baseQuery := `SELECT id, contact_list_id, phone, attributes, tags, created_at, updated_at FROM contacts WHERE contact_list_id = $1`
	allParams := []interface{}{listID}

	if whereClause != "" {
		baseQuery += " AND " + whereClause
		allParams = append(allParams, params...)
	}

	baseQuery += " ORDER BY id"

	rows, err := r.db.QueryxContext(ctx, baseQuery, allParams...)
	if err != nil {
		return fmt.Errorf("failed to stream segment: %w", err)
	}
	defer rows.Close()

	batch := make([]*domain.Contact, 0, batchSize)
	for rows.Next() {
		var row contactRow
		if err := rows.StructScan(&row); err != nil {
			return fmt.Errorf("failed to scan contact: %w", err)
		}
		batch = append(batch, row.toDomain())
		if len(batch) >= batchSize {
			if err := callback(batch); err != nil {
				return err
			}
			batch = batch[:0]
		}
	}
	if len(batch) > 0 {
		return callback(batch)
	}
	return nil
}

// PreviewSegmentCount returns the count of contacts matching segment rules.
func (r *ContactRepository) PreviewSegmentCount(ctx context.Context, listID uuid.UUID, rules *domain.SegmentRule, tags []string) (int, error) {
	whereClause, params, err := domain.SegmentToSQL(rules, tags)
	if err != nil {
		return 0, err
	}

	query := `SELECT COUNT(*) FROM contacts WHERE contact_list_id = $1`
	allParams := []interface{}{listID}

	if whereClause != "" {
		query += " AND " + whereClause
		allParams = append(allParams, params...)
	}

	var count int
	err = r.db.QueryRowContext(ctx, query, allParams...).Scan(&count)
	return count, err
}
```

- [ ] **Step 3: Write ImportRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type ImportRepository struct {
	db *sqlx.DB
}

func NewImportRepository(db *sqlx.DB) *ImportRepository {
	return &ImportRepository{db: db}
}

func (r *ImportRepository) Create(ctx context.Context, job *domain.ImportJob) (*domain.ImportJob, error) {
	mappingJSON, _ := json.Marshal(job.ColumnMapping)
	query := `INSERT INTO contact_imports (id, contact_list_id, client_id, file_name, file_size, status, column_mapping)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at`

	row := r.db.QueryRowxContext(ctx, query, job.ID, job.ContactListID, job.ClientID, job.FileName, job.FileSize, job.Status, mappingJSON)

	return r.scanImportJob(row)
}

func (r *ImportRepository) GetByID(ctx context.Context, id, listID uuid.UUID) (*domain.ImportJob, error) {
	query := `SELECT id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at
		FROM contact_imports WHERE id = $1 AND contact_list_id = $2`

	row := r.db.QueryRowxContext(ctx, query, id, listID)
	return r.scanImportJob(row)
}

func (r *ImportRepository) List(ctx context.Context, listID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error) {
	var total int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM contact_imports WHERE contact_list_id = $1`, listID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT id, contact_list_id, client_id, file_name, file_size, status, total_rows, imported_count, updated_count, error_count, errors, column_mapping, created_at, completed_at
		FROM contact_imports WHERE contact_list_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := r.db.QueryxContext(ctx, query, listID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var jobs []*domain.ImportJob
	for rows.Next() {
		job, err := r.scanImportJobRows(rows)
		if err != nil {
			return nil, 0, err
		}
		jobs = append(jobs, job)
	}
	return jobs, total, nil
}

func (r *ImportRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, totalRows, imported, updated, errorCount int, errors []domain.ImportError) error {
	errJSON, _ := json.Marshal(errors)
	query := `UPDATE contact_imports SET status = $1, total_rows = $2, imported_count = $3, updated_count = $4, error_count = $5, errors = $6`
	args := []interface{}{status, totalRows, imported, updated, errorCount, errJSON}

	if status == "completed" || status == "failed" {
		query += `, completed_at = now()`
	}
	query += fmt.Sprintf(` WHERE id = $%d`, len(args)+1)
	args = append(args, id)

	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

type importJobRow struct {
	ID            uuid.UUID      `db:"id"`
	ContactListID uuid.UUID      `db:"contact_list_id"`
	ClientID      uuid.UUID      `db:"client_id"`
	FileName      string         `db:"file_name"`
	FileSize      int64          `db:"file_size"`
	Status        string         `db:"status"`
	TotalRows     int            `db:"total_rows"`
	ImportedCount int            `db:"imported_count"`
	UpdatedCount  int            `db:"updated_count"`
	ErrorCount    int            `db:"error_count"`
	Errors        sql.NullString `db:"errors"`
	ColumnMapping sql.NullString `db:"column_mapping"`
	CreatedAt     sql.NullTime   `db:"created_at"`
	CompletedAt   sql.NullTime   `db:"completed_at"`
}

func (r *ImportRepository) scanImportJob(row *sqlx.Row) (*domain.ImportJob, error) {
	var ir importJobRow
	if err := row.StructScan(&ir); err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrImportNotFound
		}
		return nil, fmt.Errorf("failed to scan import job: %w", err)
	}
	return ir.toDomain(), nil
}

func (r *ImportRepository) scanImportJobRows(rows *sqlx.Rows) (*domain.ImportJob, error) {
	var ir importJobRow
	if err := rows.StructScan(&ir); err != nil {
		return nil, fmt.Errorf("failed to scan import job: %w", err)
	}
	return ir.toDomain(), nil
}

func (ir *importJobRow) toDomain() *domain.ImportJob {
	job := &domain.ImportJob{
		ID:            ir.ID,
		ContactListID: ir.ContactListID,
		ClientID:      ir.ClientID,
		FileName:      ir.FileName,
		FileSize:      ir.FileSize,
		Status:        ir.Status,
		TotalRows:     ir.TotalRows,
		ImportedCount: ir.ImportedCount,
		UpdatedCount:  ir.UpdatedCount,
		ErrorCount:    ir.ErrorCount,
	}
	if ir.Errors.Valid {
		json.Unmarshal([]byte(ir.Errors.String), &job.Errors)
	}
	if ir.ColumnMapping.Valid {
		json.Unmarshal([]byte(ir.ColumnMapping.String), &job.ColumnMapping)
	}
	if ir.CreatedAt.Valid {
		job.CreatedAt = ir.CreatedAt.Time
	}
	if ir.CompletedAt.Valid {
		t := ir.CompletedAt.Time
		job.CompletedAt = &t
	}
	return job
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/contact/infrastructure/repository/
git commit -m "feat(contacts): add contact list, contact, and import repositories"
```

---

## Task 8: contact-service — Application Layer

**Files:**
- Create: `internal/services/contact/application/contact_service.go`
- Create: `internal/services/contact/application/import_worker.go`

- [ ] **Step 1: Write ContactService (business logic)**

```go
package application

import (
	"context"
	"fmt"
	"regexp"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
	"github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
)

var phoneRegex = regexp.MustCompile(`^\+?[1-9]\d{6,14}$`)

type ContactService struct {
	listRepo    *repository.ContactListRepository
	contactRepo *repository.ContactRepository
	importRepo  *repository.ImportRepository
	logger      zerolog.Logger
}

func NewContactService(
	listRepo *repository.ContactListRepository,
	contactRepo *repository.ContactRepository,
	importRepo *repository.ImportRepository,
) *ContactService {
	return &ContactService{
		listRepo:    listRepo,
		contactRepo: contactRepo,
		importRepo:  importRepo,
		logger:      log.With().Str("component", "contact-service").Logger(),
	}
}

// === Contact Lists ===

func (s *ContactService) CreateContactList(ctx context.Context, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	cl := &domain.ContactList{
		ID:          uuid.New(),
		ClientID:    clientID,
		Name:        name,
		Description: description,
	}
	return s.listRepo.Create(ctx, cl)
}

func (s *ContactService) GetContactList(ctx context.Context, id, clientID uuid.UUID) (*domain.ContactList, error) {
	return s.listRepo.GetByID(ctx, id, clientID)
}

func (s *ContactService) ListContactLists(ctx context.Context, clientID uuid.UUID, limit, offset int) ([]*domain.ContactList, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.listRepo.List(ctx, clientID, limit, offset)
}

func (s *ContactService) UpdateContactList(ctx context.Context, id, clientID uuid.UUID, name, description string) (*domain.ContactList, error) {
	existing, err := s.listRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	existing.Name = name
	existing.Description = description
	return s.listRepo.Update(ctx, existing)
}

func (s *ContactService) DeleteContactList(ctx context.Context, id, clientID uuid.UUID) error {
	return s.listRepo.Delete(ctx, id, clientID)
}

// === Attributes ===

func (s *ContactService) SetListAttributes(ctx context.Context, listID, clientID uuid.UUID, attrs []*domain.ContactAttribute) ([]*domain.ContactAttribute, error) {
	return s.listRepo.SetAttributes(ctx, listID, clientID, attrs)
}

func (s *ContactService) GetListAttributes(ctx context.Context, listID, clientID uuid.UUID) ([]*domain.ContactAttribute, error) {
	return s.listRepo.GetAttributes(ctx, listID, clientID)
}

// === Contacts ===

func (s *ContactService) CreateContact(ctx context.Context, listID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return nil, err
	}
	phone = normalizePhone(phone)
	if !phoneRegex.MatchString(phone) {
		return nil, domain.ErrInvalidPhone
	}
	c := &domain.Contact{
		ID:            uuid.New(),
		ContactListID: listID,
		Phone:         phone,
		Attributes:    attrs,
		Tags:          tags,
	}
	created, err := s.contactRepo.Create(ctx, c)
	if err != nil {
		return nil, err
	}
	s.listRepo.UpdateContactsCount(ctx, listID)
	return created, nil
}

func (s *ContactService) UpdateContact(ctx context.Context, id, listID, clientID uuid.UUID, phone string, attrs map[string]interface{}, tags []string) (*domain.Contact, error) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return nil, err
	}
	existing, err := s.contactRepo.GetByID(ctx, id, listID)
	if err != nil {
		return nil, err
	}
	if phone != "" {
		phone = normalizePhone(phone)
		if !phoneRegex.MatchString(phone) {
			return nil, domain.ErrInvalidPhone
		}
		existing.Phone = phone
	}
	if attrs != nil {
		existing.Attributes = attrs
	}
	if tags != nil {
		existing.Tags = tags
	}
	return s.contactRepo.Update(ctx, existing)
}

func (s *ContactService) DeleteContact(ctx context.Context, id, listID, clientID uuid.UUID) error {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return err
	}
	err := s.contactRepo.Delete(ctx, id, listID)
	if err == nil {
		s.listRepo.UpdateContactsCount(ctx, listID)
	}
	return err
}

func (s *ContactService) ListContacts(ctx context.Context, listID, clientID uuid.UUID, limit, offset int, search string, tags []string) ([]*domain.Contact, int, error) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return nil, 0, err
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.contactRepo.List(ctx, listID, limit, offset, search, tags)
}

func (s *ContactService) BatchUpsertContacts(ctx context.Context, listID, clientID uuid.UUID, contacts []*domain.Contact) (created, updated, errCount int, errMsgs []string) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return 0, 0, len(contacts), []string{err.Error()}
	}
	for _, c := range contacts {
		c.Phone = normalizePhone(c.Phone)
		c.ContactListID = listID
	}
	created, updated, errCount, errMsgs = s.contactRepo.BatchUpsert(ctx, listID, contacts)
	s.listRepo.UpdateContactsCount(ctx, listID)
	return
}

// === Tags ===

func (s *ContactService) AddTags(ctx context.Context, listID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return err
	}
	return s.contactRepo.AddTags(ctx, listID, contactIDs, tags)
}

func (s *ContactService) RemoveTags(ctx context.Context, listID, clientID uuid.UUID, contactIDs []uuid.UUID, tags []string) error {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return err
	}
	return s.contactRepo.RemoveTags(ctx, listID, contactIDs, tags)
}

func (s *ContactService) ListTags(ctx context.Context, listID, clientID uuid.UUID) ([]string, error) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return nil, err
	}
	return s.contactRepo.ListTags(ctx, listID)
}

// === Segmentation ===

func (s *ContactService) PreviewSegment(ctx context.Context, listID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string) (int, error) {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return 0, err
	}
	return s.contactRepo.PreviewSegmentCount(ctx, listID, rules, tags)
}

func (s *ContactService) StreamSegment(ctx context.Context, listID, clientID uuid.UUID, rules *domain.SegmentRule, tags []string, batchSize int, callback func([]*domain.Contact) error) error {
	if _, err := s.listRepo.GetByID(ctx, listID, clientID); err != nil {
		return err
	}
	return s.contactRepo.StreamSegment(ctx, listID, rules, tags, batchSize, callback)
}

// === Import ===

func (s *ContactService) StartImport(ctx context.Context, job *domain.ImportJob) (*domain.ImportJob, error) {
	if _, err := s.listRepo.GetByID(ctx, job.ContactListID, job.ClientID); err != nil {
		return nil, err
	}
	job.Status = "processing"
	created, err := s.importRepo.Create(ctx, job)
	if err != nil {
		return nil, err
	}
	// Launch async import worker
	go s.processImport(created)
	return created, nil
}

func (s *ContactService) GetImportStatus(ctx context.Context, id, listID uuid.UUID) (*domain.ImportJob, error) {
	return s.importRepo.GetByID(ctx, id, listID)
}

func (s *ContactService) ListImports(ctx context.Context, listID uuid.UUID, limit, offset int) ([]*domain.ImportJob, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.importRepo.List(ctx, listID, limit, offset)
}

func normalizePhone(phone string) string {
	// Strip spaces and dashes, ensure + prefix
	cleaned := ""
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' || ch == '+' {
			cleaned += string(ch)
		}
	}
	if len(cleaned) > 0 && cleaned[0] != '+' {
		cleaned = "+" + cleaned
	}
	return cleaned
}

func (s *ContactService) processImport(job *domain.ImportJob) {
	// Implementation in import_worker.go
	s.logger.Info().Str("import_id", job.ID.String()).Msg("import worker started (placeholder)")
}
```

- [ ] **Step 2: Write ImportWorker (async CSV processing)**

```go
package application

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

const importBatchSize = 1000

// processImportFile reads CSV file and upserts contacts in batches.
func (s *ContactService) processImportFile(job *domain.ImportJob) {
	ctx := context.Background()
	logger := s.logger.With().Str("import_id", job.ID.String()).Logger()

	filePath := fmt.Sprintf("uploads/%s/%s/%s", job.ClientID.String(), job.ID.String(), job.FileName)
	file, err := os.Open(filePath)
	if err != nil {
		logger.Error().Err(err).Msg("failed to open import file")
		s.importRepo.UpdateStatus(ctx, job.ID, "failed", 0, 0, 0, 1, []domain.ImportError{{Row: 0, Reason: "failed to open file"}})
		return
	}
	defer file.Close()

	mapping := job.ColumnMapping
	if mapping == nil {
		s.importRepo.UpdateStatus(ctx, job.ID, "failed", 0, 0, 0, 1, []domain.ImportError{{Row: 0, Reason: "no column mapping"}})
		return
	}

	delimiter := ','
	if mapping.Delimiter != "" {
		delimiter = rune(mapping.Delimiter[0])
	}

	reader := csv.NewReader(bufio.NewReader(file))
	reader.Comma = delimiter
	reader.LazyQuotes = true

	var totalRows, imported, updated, errorCount int
	var importErrors []domain.ImportError
	rowNum := 0

	// Skip header if present
	if mapping.HasHeader {
		if _, err := reader.Read(); err != nil {
			s.importRepo.UpdateStatus(ctx, job.ID, "failed", 0, 0, 0, 1, []domain.ImportError{{Row: 0, Reason: "failed to read header"}})
			return
		}
		rowNum++
	}

	batch := make([]*domain.Contact, 0, importBatchSize)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		totalRows++

		if err != nil {
			errorCount++
			importErrors = append(importErrors, domain.ImportError{Row: rowNum, Reason: err.Error()})
			continue
		}

		contact, parseErr := parseCSVRow(record, mapping, job.ContactListID)
		if parseErr != nil {
			errorCount++
			importErrors = append(importErrors, domain.ImportError{Row: rowNum, Reason: parseErr.Error()})
			continue
		}

		batch = append(batch, contact)

		if len(batch) >= importBatchSize {
			c, u, e, msgs := s.contactRepo.BatchUpsert(ctx, job.ContactListID, batch)
			imported += c
			updated += u
			errorCount += e
			for _, m := range msgs {
				importErrors = append(importErrors, domain.ImportError{Row: rowNum, Reason: m})
			}
			batch = batch[:0]

			// Update progress
			s.importRepo.UpdateStatus(ctx, job.ID, "processing", totalRows, imported, updated, errorCount, importErrors)
		}
	}

	// Process remaining batch
	if len(batch) > 0 {
		c, u, e, msgs := s.contactRepo.BatchUpsert(ctx, job.ContactListID, batch)
		imported += c
		updated += u
		errorCount += e
		for _, m := range msgs {
			importErrors = append(importErrors, domain.ImportError{Row: rowNum, Reason: m})
		}
	}

	// Finalize
	s.listRepo.UpdateContactsCount(ctx, job.ContactListID)
	s.importRepo.UpdateStatus(ctx, job.ID, "completed", totalRows, imported, updated, errorCount, importErrors)
	logger.Info().Int("total", totalRows).Int("imported", imported).Int("updated", updated).Int("errors", errorCount).Msg("import completed")
}

func parseCSVRow(record []string, mapping *domain.ColumnMapping, listID uuid.UUID) (*domain.Contact, error) {
	contact := &domain.Contact{
		ID:            uuid.New(),
		ContactListID: listID,
		Attributes:    make(map[string]interface{}),
	}

	for colIdxStr, target := range mapping.Columns {
		colIdx, err := strconv.Atoi(colIdxStr)
		if err != nil || colIdx >= len(record) {
			continue
		}
		value := strings.TrimSpace(record[colIdx])

		if target.Target == "phone" {
			contact.Phone = normalizePhone(value)
			continue
		}

		switch target.Type {
		case "number":
			if n, err := strconv.ParseFloat(value, 64); err == nil {
				contact.Attributes[target.Target] = n
			} else {
				contact.Attributes[target.Target] = value
			}
		case "boolean":
			b := strings.ToLower(value) == "true" || value == "1"
			contact.Attributes[target.Target] = b
		default:
			contact.Attributes[target.Target] = value
		}
	}

	if contact.Phone == "" {
		return nil, fmt.Errorf("missing phone number")
	}
	if !phoneRegex.MatchString(contact.Phone) {
		return nil, fmt.Errorf("invalid phone: %s", contact.Phone)
	}

	return contact, nil
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/contact/application/
git commit -m "feat(contacts): add contact application service with import worker"
```

---

## Task 9: contact-service — gRPC Server

**Files:**
- Create: `internal/services/contact/grpc/server.go`
- Create: `internal/services/contact/grpc/mappers.go`

- [ ] **Step 1: Write gRPC server**

```go
package grpc

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

type Server struct {
	contactv1.UnimplementedContactServiceServer
	contactService *application.ContactService
	logger         zerolog.Logger
}

func NewServer(contactService *application.ContactService) *Server {
	return &Server{
		contactService: contactService,
		logger:         log.With().Str("component", "contact-grpc-server").Logger(),
	}
}

// === Contact Lists ===

func (s *Server) CreateContactList(ctx context.Context, req *contactv1.CreateContactListRequest) (*contactv1.ContactList, error) {
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	cl, err := s.contactService.CreateContactList(ctx, clientID, req.Name, req.Description)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) ListContactLists(ctx context.Context, req *contactv1.ListContactListsRequest) (*contactv1.ContactListPage, error) {
	clientID, err := parseUUID(req.ClientId, "client_id")
	if err != nil {
		return nil, err
	}

	lists, total, err := s.contactService.ListContactLists(ctx, clientID, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, s.mapError(err)
	}

	protoLists := make([]*contactv1.ContactList, len(lists))
	for i, cl := range lists {
		protoLists[i] = contactListToProto(cl)
	}
	return &contactv1.ContactListPage{Items: protoLists, Total: int32(total)}, nil
}

func (s *Server) GetContactList(ctx context.Context, req *contactv1.GetContactListRequest) (*contactv1.ContactList, error) {
	id, clientID, err := parseTwoUUIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	cl, err := s.contactService.GetContactList(ctx, id, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) UpdateContactList(ctx context.Context, req *contactv1.UpdateContactListRequest) (*contactv1.ContactList, error) {
	id, clientID, err := parseTwoUUIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	cl, err := s.contactService.UpdateContactList(ctx, id, clientID, req.Name, req.Description)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactListToProto(cl), nil
}

func (s *Server) DeleteContactList(ctx context.Context, req *contactv1.DeleteContactListRequest) (*emptypb.Empty, error) {
	id, clientID, err := parseTwoUUIDs(req.Id, req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.contactService.DeleteContactList(ctx, id, clientID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

// === Attributes ===

func (s *Server) SetListAttributes(ctx context.Context, req *contactv1.SetListAttributesRequest) (*contactv1.AttributeList, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	attrs := make([]*domain.ContactAttribute, len(req.Attributes))
	for i, a := range req.Attributes {
		attrs[i] = &domain.ContactAttribute{
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Type:        a.Type,
			Required:    a.Required,
			Position:    int(a.Position),
		}
	}

	result, err := s.contactService.SetListAttributes(ctx, listID, clientID, attrs)
	if err != nil {
		return nil, s.mapError(err)
	}
	return attributeListToProto(result), nil
}

func (s *Server) GetListAttributes(ctx context.Context, req *contactv1.GetListAttributesRequest) (*contactv1.AttributeList, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	attrs, err := s.contactService.GetListAttributes(ctx, listID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return attributeListToProto(attrs), nil
}

// === Contacts ===

func (s *Server) CreateContact(ctx context.Context, req *contactv1.CreateContactRequest) (*contactv1.Contact, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}
	if req.Phone == "" {
		return nil, status.Error(codes.InvalidArgument, "phone is required")
	}

	attrs := structToMap(req.Attributes)
	c, err := s.contactService.CreateContact(ctx, listID, clientID, req.Phone, attrs, req.Tags)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactToProto(c), nil
}

func (s *Server) UpdateContact(ctx context.Context, req *contactv1.UpdateContactRequest) (*contactv1.Contact, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	attrs := structToMap(req.Attributes)
	c, err := s.contactService.UpdateContact(ctx, id, listID, clientID, req.Phone, attrs, req.Tags)
	if err != nil {
		return nil, s.mapError(err)
	}
	return contactToProto(c), nil
}

func (s *Server) DeleteContact(ctx context.Context, req *contactv1.DeleteContactRequest) (*emptypb.Empty, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	if err := s.contactService.DeleteContact(ctx, id, listID, clientID); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListContacts(ctx context.Context, req *contactv1.ListContactsRequest) (*contactv1.ContactPage, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	contacts, total, err := s.contactService.ListContacts(ctx, listID, clientID, int(req.Limit), int(req.Offset), req.Search, req.Tags)
	if err != nil {
		return nil, s.mapError(err)
	}

	protoContacts := make([]*contactv1.Contact, len(contacts))
	for i, c := range contacts {
		protoContacts[i] = contactToProto(c)
	}
	return &contactv1.ContactPage{Contacts: protoContacts, Total: int32(total)}, nil
}

func (s *Server) BatchUpsertContacts(ctx context.Context, req *contactv1.BatchUpsertContactsRequest) (*contactv1.BatchResult, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	contacts := make([]*domain.Contact, len(req.Contacts))
	for i, c := range req.Contacts {
		contacts[i] = &domain.Contact{
			Phone:      c.Phone,
			Attributes: structToMap(c.Attributes),
			Tags:       c.Tags,
		}
	}

	created, updated, errCount, errMsgs := s.contactService.BatchUpsertContacts(ctx, listID, clientID, contacts)
	return &contactv1.BatchResult{
		Created:       int32(created),
		Updated:       int32(updated),
		Errors:        int32(errCount),
		ErrorMessages: errMsgs,
	}, nil
}

// === Tags ===

func (s *Server) AddTags(ctx context.Context, req *contactv1.AddTagsRequest) (*emptypb.Empty, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	contactIDs, err := parseUUIDSlice(req.ContactIds)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid contact_ids: %v", err)
	}

	if err := s.contactService.AddTags(ctx, listID, clientID, contactIDs, req.Tags); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) RemoveTags(ctx context.Context, req *contactv1.RemoveTagsRequest) (*emptypb.Empty, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	contactIDs, err := parseUUIDSlice(req.ContactIds)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "invalid contact_ids: %v", err)
	}

	if err := s.contactService.RemoveTags(ctx, listID, clientID, contactIDs, req.Tags); err != nil {
		return nil, s.mapError(err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListTags(ctx context.Context, req *contactv1.ListTagsRequest) (*contactv1.TagList, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	tags, err := s.contactService.ListTags(ctx, listID, clientID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &contactv1.TagList{Tags: tags}, nil
}

// === Import ===

func (s *Server) StartImport(ctx context.Context, req *contactv1.StartImportRequest) (*contactv1.ImportJob, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	importID := uuid.New()
	if req.ImportId != "" {
		importID, err = uuid.Parse(req.ImportId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid import_id")
		}
	}

	job := &domain.ImportJob{
		ID:            importID,
		ContactListID: listID,
		ClientID:      clientID,
		FileName:      req.FileName,
		FileSize:      req.FileSize,
	}

	// Parse column mapping from JSON string
	if req.ColumnMapping != "" {
		var mapping domain.ColumnMapping
		if err := json.Unmarshal([]byte(req.ColumnMapping), &mapping); err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid column_mapping JSON")
		}
		job.ColumnMapping = &mapping
	}

	result, err := s.contactService.StartImport(ctx, job)
	if err != nil {
		return nil, s.mapError(err)
	}
	return importJobToProto(result), nil
}

func (s *Server) GetImportStatus(ctx context.Context, req *contactv1.GetImportStatusRequest) (*contactv1.ImportJob, error) {
	id, err := parseUUID(req.Id, "id")
	if err != nil {
		return nil, err
	}
	listID, err := parseUUID(req.ContactListId, "contact_list_id")
	if err != nil {
		return nil, err
	}

	job, err := s.contactService.GetImportStatus(ctx, id, listID)
	if err != nil {
		return nil, s.mapError(err)
	}
	return importJobToProto(job), nil
}

func (s *Server) ListImports(ctx context.Context, req *contactv1.ListImportsRequest) (*contactv1.ImportJobPage, error) {
	listID, err := parseUUID(req.ContactListId, "contact_list_id")
	if err != nil {
		return nil, err
	}

	jobs, total, err := s.contactService.ListImports(ctx, listID, int(req.Limit), int(req.Offset))
	if err != nil {
		return nil, s.mapError(err)
	}

	protoJobs := make([]*contactv1.ImportJob, len(jobs))
	for i, j := range jobs {
		protoJobs[i] = importJobToProto(j)
	}
	return &contactv1.ImportJobPage{Imports: protoJobs, Total: int32(total)}, nil
}

// === Segmentation ===

func (s *Server) PreviewSegment(ctx context.Context, req *contactv1.PreviewSegmentRequest) (*contactv1.SegmentPreview, error) {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return nil, err
	}

	rules := protoRuleToDomain(req.Rules)
	count, err := s.contactService.PreviewSegment(ctx, listID, clientID, rules, req.Tags)
	if err != nil {
		return nil, s.mapError(err)
	}
	return &contactv1.SegmentPreview{Count: int32(count)}, nil
}

func (s *Server) StreamSegment(req *contactv1.StreamSegmentRequest, stream contactv1.ContactService_StreamSegmentServer) error {
	listID, clientID, err := parseTwoUUIDs(req.ContactListId, req.ClientId)
	if err != nil {
		return err
	}

	rules := protoRuleToDomain(req.Rules)
	return s.contactService.StreamSegment(stream.Context(), listID, clientID, rules, req.Tags, 5000, func(contacts []*domain.Contact) error {
		protoContacts := make([]*contactv1.Contact, len(contacts))
		for i, c := range contacts {
			protoContacts[i] = contactToProto(c)
		}
		return stream.Send(&contactv1.ContactBatch{Contacts: protoContacts})
	})
}

// === Helpers ===

func (s *Server) mapError(err error) error {
	switch {
	case errors.Is(err, domain.ErrContactListNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrContactNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrImportNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, domain.ErrDuplicatePhone):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrDuplicateListName):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, domain.ErrInvalidPhone):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrInvalidSegmentRules):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrSegmentDepthExceeded):
		return status.Error(codes.InvalidArgument, err.Error())
	default:
		s.logger.Error().Err(err).Msg("internal error")
		return status.Error(codes.Internal, err.Error())
	}
}
```

Note: The `StartImport` method needs `encoding/json` import — add it to the import block.

- [ ] **Step 2: Write mappers (proto ↔ domain converters)**

```go
package grpc

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/services/contact/domain"
)

func parseUUID(s, fieldName string) (uuid.UUID, error) {
	if s == "" {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "%s is required", fieldName)
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, status.Errorf(codes.InvalidArgument, "invalid %s format", fieldName)
	}
	return id, nil
}

func parseTwoUUIDs(idStr, clientIDStr string) (uuid.UUID, uuid.UUID, error) {
	id, err := parseUUID(idStr, "id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	clientID, err := parseUUID(clientIDStr, "client_id")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return id, clientID, nil
}

func parseUUIDSlice(ss []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, len(ss))
	for i, s := range ss {
		id, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid UUID at index %d: %s", i, s)
		}
		ids[i] = id
	}
	return ids, nil
}

func contactListToProto(cl *domain.ContactList) *contactv1.ContactList {
	return &contactv1.ContactList{
		Id:            cl.ID.String(),
		ClientId:      cl.ClientID.String(),
		Name:          cl.Name,
		Description:   cl.Description,
		ContactsCount: int32(cl.ContactsCount),
		CreatedAt:     timestamppb.New(cl.CreatedAt),
		UpdatedAt:     timestamppb.New(cl.UpdatedAt),
	}
}

func contactToProto(c *domain.Contact) *contactv1.Contact {
	attrs, _ := structpb.NewStruct(c.Attributes)
	return &contactv1.Contact{
		Id:            c.ID.String(),
		ContactListId: c.ContactListID.String(),
		Phone:         c.Phone,
		Attributes:    attrs,
		Tags:          c.Tags,
		CreatedAt:     timestamppb.New(c.CreatedAt),
		UpdatedAt:     timestamppb.New(c.UpdatedAt),
	}
}

func attributeListToProto(attrs []*domain.ContactAttribute) *contactv1.AttributeList {
	protoAttrs := make([]*contactv1.Attribute, len(attrs))
	for i, a := range attrs {
		protoAttrs[i] = &contactv1.Attribute{
			Id:          a.ID.String(),
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Type:        a.Type,
			Required:    a.Required,
			Position:    int32(a.Position),
		}
	}
	return &contactv1.AttributeList{Attributes: protoAttrs}
}

func importJobToProto(job *domain.ImportJob) *contactv1.ImportJob {
	proto := &contactv1.ImportJob{
		Id:            job.ID.String(),
		ContactListId: job.ContactListID.String(),
		ClientId:      job.ClientID.String(),
		FileName:      job.FileName,
		FileSize:      job.FileSize,
		Status:        job.Status,
		TotalRows:     int32(job.TotalRows),
		ImportedCount: int32(job.ImportedCount),
		UpdatedCount:  int32(job.UpdatedCount),
		ErrorCount:    int32(job.ErrorCount),
		CreatedAt:     timestamppb.New(job.CreatedAt),
	}
	if job.Errors != nil {
		errJSON, _ := json.Marshal(job.Errors)
		proto.Errors = string(errJSON)
	}
	if job.ColumnMapping != nil {
		mappingJSON, _ := json.Marshal(job.ColumnMapping)
		proto.ColumnMapping = string(mappingJSON)
	}
	if job.CompletedAt != nil {
		proto.CompletedAt = timestamppb.New(*job.CompletedAt)
	}
	return proto
}

func structToMap(s *structpb.Struct) map[string]interface{} {
	if s == nil {
		return make(map[string]interface{})
	}
	return s.AsMap()
}

func protoRuleToDomain(rule *contactv1.SegmentRule) *domain.SegmentRule {
	if rule == nil {
		return nil
	}
	r := &domain.SegmentRule{
		Operator: rule.Operator,
	}
	for _, c := range rule.Conditions {
		r.Conditions = append(r.Conditions, domain.SegmentCondition{
			Field:  c.Field,
			Op:     c.Op,
			Value:  c.Value,
			Value2: c.Value2,
		})
	}
	for _, n := range rule.Nested {
		nested := protoRuleToDomain(n)
		if nested != nil {
			r.Nested = append(r.Nested, *nested)
		}
	}
	return r
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/services/contact/grpc/
git commit -m "feat(contacts): add contact gRPC server and proto mappers"
```

---

## Task 10: contact-service — Main Entrypoint

**Files:**
- Create: `cmd/services/contact-service/main.go`

- [ ] **Step 1: Write service entrypoint**

Follow the exact pattern from `cmd/services/template-service/main.go`:

```go
package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
	"github.com/smpp-server/smpp-server/internal/config"
	"github.com/smpp-server/smpp-server/internal/monitoring"
	"github.com/smpp-server/smpp-server/internal/services/contact/application"
	contactgrpc "github.com/smpp-server/smpp-server/internal/services/contact/grpc"
	contactrepo "github.com/smpp-server/smpp-server/internal/services/contact/infrastructure/repository"
	"github.com/smpp-server/smpp-server/internal/shared"
	"github.com/smpp-server/smpp-server/internal/shared/database"
	"github.com/smpp-server/smpp-server/internal/storage"
)

func main() {
	shared.InitLogger("development")
	logger := shared.WithService("contact-service")

	cfg, err := config.Load("")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка загрузки конфигурации")
	}

	logger.Info().
		Str("version", cfg.Service.Version).
		Str("env", cfg.Service.Env).
		Msg("запуск Contact Service")

	// Wait for database
	logger.Info().Msg("ожидание готовности базы данных")
	if err := storage.WaitForDatabase(cfg.Database.GetDSN(), 30, 2*time.Second); err != nil {
		logger.Fatal().Err(err).Msg("база данных недоступна")
	}

	dbConn, err := database.NewDBWithConfig(database.Config{
		DSN:             cfg.Database.GetDSN(),
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка подключения к базе данных")
	}
	defer dbConn.Close()
	logger.Info().Msg("подключение к базе данных установлено")

	dbx := sqlx.NewDb(dbConn.DB, "pgx")

	// Repositories
	listRepo := contactrepo.NewContactListRepository(dbx)
	contactRepo := contactrepo.NewContactRepository(dbx)
	importRepo := contactrepo.NewImportRepository(dbx)

	// Application service
	contactService := application.NewContactService(listRepo, contactRepo, importRepo)

	// Health checker
	healthChecker := monitoring.NewHealthChecker("contact-service", cfg.Service.Version)
	healthChecker.SetDatabase(dbConn.DB)

	// gRPC server
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(cfg.API.GRPC.MaxRecv),
		grpc.MaxSendMsgSize(cfg.API.GRPC.MaxSend),
	)

	contactGrpcServer := contactgrpc.NewServer(contactService)
	contactv1.RegisterContactServiceServer(grpcServer, contactGrpcServer)

	if cfg.Service.Env == "development" {
		reflection.Register(grpcServer)
		logger.Info().Msg("gRPC reflection включен")
	}

	grpcListener, err := net.Listen("tcp", ":5012")
	if err != nil {
		logger.Fatal().Err(err).Msg("ошибка создания gRPC listener")
	}

	go func() {
		logger.Info().Str("addr", grpcListener.Addr().String()).Msg("gRPC сервер запущен")
		if err := grpcServer.Serve(grpcListener); err != nil {
			logger.Fatal().Err(err).Msg("ошибка запуска gRPC сервера")
		}
	}()

	// Metrics HTTP server
	metricsMux := http.NewServeMux()
	metricsMux.HandleFunc("/health", healthChecker.Handler())
	metricsMux.HandleFunc("/health/live", healthChecker.LivenessHandler())
	metricsMux.HandleFunc("/health/ready", healthChecker.ReadinessHandler())

	if cfg.Monitoring.Prometheus.Enabled {
		metricsMux.Handle(cfg.Monitoring.Prometheus.Path, promhttp.Handler())
		logger.Info().Str("path", cfg.Monitoring.Prometheus.Path).Msg("Prometheus metrics endpoint включен")
	}

	metricsServer := &http.Server{
		Addr:         ":2130",
		Handler:      metricsMux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	go func() {
		logger.Info().Str("addr", metricsServer.Addr).Msg("HTTP сервер для метрик запущен")
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("ошибка запуска HTTP сервера")
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info().Msg("получен сигнал завершения, остановка сервиса")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	logger.Info().Msg("gRPC сервер остановлен")

	if err := metricsServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("ошибка при остановке HTTP сервера")
	}
	logger.Info().Msg("HTTP сервер остановлен")

	logger.Info().Msg("Contact Service остановлен")
}
```

- [ ] **Step 2: Commit**

```bash
git add cmd/services/contact-service/
git commit -m "feat(contacts): add contact-service entrypoint"
```

---

## Task 11: campaign-service — Domain Models

**Files:**
- Create: `internal/services/campaign/domain/models.go`

- [ ] **Step 1: Write campaign domain models**

```go
package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrCampaignNotFound        = errors.New("campaign not found")
	ErrVariantNotFound         = errors.New("variant not found")
	ErrInvalidCampaignStatus   = errors.New("invalid campaign status for this operation")
	ErrCampaignNotDraft        = errors.New("campaign must be in draft status")
	ErrCampaignNotRunning      = errors.New("campaign must be in running status")
	ErrCampaignNotPaused       = errors.New("campaign must be in paused status")
	ErrVariantPercentageSum    = errors.New("variant percentages must sum to 100")
	ErrTooFewVariants          = errors.New("A/B test requires at least 2 variants")
	ErrTooManyVariants         = errors.New("maximum 5 variants allowed")
	ErrWinnerAlreadySelected   = errors.New("winner already selected")
	ErrNoFailedRecipients      = errors.New("no failed recipients to retry")
)

const (
	StatusDraft         = "draft"
	StatusScheduled     = "scheduled"
	StatusMaterializing = "materializing"
	StatusRunning       = "running"
	StatusPaused        = "paused"
	StatusCompleted     = "completed"
	StatusCancelled     = "cancelled"
)

const (
	RecipientPending   = "pending"
	RecipientSent      = "sent"
	RecipientDelivered = "delivered"
	RecipientFailed    = "failed"
	RecipientRetry     = "retry"
	RecipientCancelled = "cancelled"
)

type Campaign struct {
	ID              uuid.UUID
	ClientID        uuid.UUID
	Name            string
	Status          string
	ContactListID   uuid.UUID
	TemplateID      *uuid.UUID
	Source          string
	SegmentRules    string // JSON
	SegmentTags     []string
	SendRate        int
	ScheduledAt     *time.Time
	StartedAt       *time.Time
	CompletedAt     *time.Time
	RetryConfig     *RetryConfig
	TotalRecipients int
	SentCount       int
	DeliveredCount  int
	FailedCount     int
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Variants        []Variant
	ABConfig        *ABConfig
}

type Variant struct {
	ID             uuid.UUID
	CampaignID     uuid.UUID
	Name           string
	TemplateID     *uuid.UUID
	Percentage     int
	IsWinner       bool
	IsControl      bool
	SentCount      int
	DeliveredCount int
	FailedCount    int
}

type ABConfig struct {
	CampaignID        uuid.UUID
	Metric            string // delivery_rate
	TestDurationHours int
	AutoSelectWinner  bool
	WinnerVariantID   *uuid.UUID
	WinnerSelectedAt  *time.Time
}

type RetryConfig struct {
	Enabled               bool       `json:"enabled"`
	DelayHours            int        `json:"delay_hours"`
	MaxRetries            int        `json:"max_retries"`
	AlternativeTemplateID *uuid.UUID `json:"alternative_template_id,omitempty"`
}

type Recipient struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	ContactID   uuid.UUID
	Phone       string
	VariantID   *uuid.UUID
	Status      string
	MessageID   *uuid.UUID
	RetryCount  int
	LastRetryAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type RetryLogEntry struct {
	ID          uuid.UUID
	CampaignID  uuid.UUID
	RecipientID uuid.UUID
	RetryNumber int
	ProviderID  *uuid.UUID
	Status      string
	ErrorCode   string
	CreatedAt   time.Time
}

type StatsSnapshot struct {
	CampaignID       uuid.UUID
	VariantID        *uuid.UUID
	SnapshotAt       time.Time
	Sent             int
	Delivered        int
	Failed           int
	Pending          int
	AvgDeliveryTimeMs int
	Cost             float64
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/domain/models.go
git commit -m "feat(campaigns): add campaign domain models"
```

---

## Task 12: campaign-service — Repositories

**Files:**
- Create: `internal/services/campaign/infrastructure/repository/campaign_repository.go`
- Create: `internal/services/campaign/infrastructure/repository/recipient_repository.go`
- Create: `internal/services/campaign/infrastructure/repository/stats_repository.go`

- [ ] **Step 1: Write CampaignRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

type CampaignRepository struct {
	db *sqlx.DB
}

func NewCampaignRepository(db *sqlx.DB) *CampaignRepository {
	return &CampaignRepository{db: db}
}

type campaignRow struct {
	ID              uuid.UUID      `db:"id"`
	ClientID        uuid.UUID      `db:"client_id"`
	Name            string         `db:"name"`
	Status          string         `db:"status"`
	ContactListID   uuid.UUID      `db:"contact_list_id"`
	TemplateID      *uuid.UUID     `db:"template_id"`
	Source          string         `db:"source"`
	SegmentRules    sql.NullString `db:"segment_rules"`
	SegmentTags     pq.StringArray `db:"segment_tags"`
	SendRate        int            `db:"send_rate"`
	ScheduledAt     sql.NullTime   `db:"scheduled_at"`
	StartedAt       sql.NullTime   `db:"started_at"`
	CompletedAt     sql.NullTime   `db:"completed_at"`
	RetryConfig     sql.NullString `db:"retry_config"`
	TotalRecipients int            `db:"total_recipients"`
	SentCount       int            `db:"sent_count"`
	DeliveredCount  int            `db:"delivered_count"`
	FailedCount     int            `db:"failed_count"`
	CreatedAt       sql.NullTime   `db:"created_at"`
	UpdatedAt       sql.NullTime   `db:"updated_at"`
}

func (r *campaignRow) toDomain() *domain.Campaign {
	c := &domain.Campaign{
		ID:              r.ID,
		ClientID:        r.ClientID,
		Name:            r.Name,
		Status:          r.Status,
		ContactListID:   r.ContactListID,
		TemplateID:      r.TemplateID,
		Source:          r.Source,
		SegmentTags:     []string(r.SegmentTags),
		SendRate:        r.SendRate,
		TotalRecipients: r.TotalRecipients,
		SentCount:       r.SentCount,
		DeliveredCount:  r.DeliveredCount,
		FailedCount:     r.FailedCount,
	}
	if r.SegmentRules.Valid {
		c.SegmentRules = r.SegmentRules.String
	}
	if r.ScheduledAt.Valid {
		t := r.ScheduledAt.Time
		c.ScheduledAt = &t
	}
	if r.StartedAt.Valid {
		t := r.StartedAt.Time
		c.StartedAt = &t
	}
	if r.CompletedAt.Valid {
		t := r.CompletedAt.Time
		c.CompletedAt = &t
	}
	if r.RetryConfig.Valid {
		var rc domain.RetryConfig
		json.Unmarshal([]byte(r.RetryConfig.String), &rc)
		c.RetryConfig = &rc
	}
	if r.CreatedAt.Valid {
		c.CreatedAt = r.CreatedAt.Time
	}
	if r.UpdatedAt.Valid {
		c.UpdatedAt = r.UpdatedAt.Time
	}
	return c
}

const campaignCols = `id, client_id, name, status, contact_list_id, template_id, source, segment_rules, segment_tags, send_rate, scheduled_at, started_at, completed_at, retry_config, total_recipients, sent_count, delivered_count, failed_count, created_at, updated_at`

func (r *CampaignRepository) Create(ctx context.Context, c *domain.Campaign) (*domain.Campaign, error) {
	retryJSON := sql.NullString{}
	if c.RetryConfig != nil {
		data, _ := json.Marshal(c.RetryConfig)
		retryJSON = sql.NullString{String: string(data), Valid: true}
	}
	segmentRules := sql.NullString{}
	if c.SegmentRules != "" {
		segmentRules = sql.NullString{String: c.SegmentRules, Valid: true}
	}

	query := fmt.Sprintf(`INSERT INTO campaigns (id, client_id, name, status, contact_list_id, template_id, source, segment_rules, segment_tags, send_rate, scheduled_at, retry_config)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING %s`, campaignCols)

	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query,
		c.ID, c.ClientID, c.Name, c.Status, c.ContactListID, c.TemplateID, c.Source,
		segmentRules, pq.StringArray(c.SegmentTags), c.SendRate, c.ScheduledAt, retryJSON,
	).StructScan(&row)
	if err != nil {
		return nil, fmt.Errorf("failed to create campaign: %w", err)
	}
	return row.toDomain(), nil
}

func (r *CampaignRepository) GetByID(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	query := fmt.Sprintf(`SELECT %s FROM campaigns WHERE id = $1 AND client_id = $2`, campaignCols)
	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query, id, clientID).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCampaignNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get campaign: %w", err)
	}
	return row.toDomain(), nil
}

func (r *CampaignRepository) List(ctx context.Context, clientID uuid.UUID, statusFilter string, limit, offset int) ([]*domain.Campaign, int, error) {
	countQuery := `SELECT COUNT(*) FROM campaigns WHERE client_id = $1`
	listQuery := fmt.Sprintf(`SELECT %s FROM campaigns WHERE client_id = $1`, campaignCols)
	args := []interface{}{clientID}

	if statusFilter != "" {
		countQuery += ` AND status = $2`
		listQuery += ` AND status = $2`
		args = append(args, statusFilter)
	}

	var total int
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listQuery += fmt.Sprintf(` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.db.QueryxContext(ctx, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var row campaignRow
		if err := rows.StructScan(&row); err != nil {
			return nil, 0, err
		}
		campaigns = append(campaigns, row.toDomain())
	}
	return campaigns, total, nil
}

func (r *CampaignRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	query := `UPDATE campaigns SET status = $1, updated_at = now() WHERE id = $2`
	if status == domain.StatusRunning {
		query = `UPDATE campaigns SET status = $1, started_at = now(), updated_at = now() WHERE id = $2`
	} else if status == domain.StatusCompleted || status == domain.StatusCancelled {
		query = `UPDATE campaigns SET status = $1, completed_at = now(), updated_at = now() WHERE id = $2`
	}
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *CampaignRepository) UpdateCounters(ctx context.Context, id uuid.UUID, total, sent, delivered, failed int) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaigns SET total_recipients = $1, sent_count = $2, delivered_count = $3, failed_count = $4, updated_at = now() WHERE id = $5`,
		total, sent, delivered, failed, id)
	return err
}

func (r *CampaignRepository) Update(ctx context.Context, c *domain.Campaign) (*domain.Campaign, error) {
	retryJSON := sql.NullString{}
	if c.RetryConfig != nil {
		data, _ := json.Marshal(c.RetryConfig)
		retryJSON = sql.NullString{String: string(data), Valid: true}
	}
	segmentRules := sql.NullString{}
	if c.SegmentRules != "" {
		segmentRules = sql.NullString{String: c.SegmentRules, Valid: true}
	}

	query := fmt.Sprintf(`UPDATE campaigns SET name = $1, contact_list_id = $2, template_id = $3, source = $4,
		segment_rules = $5, segment_tags = $6, send_rate = $7, scheduled_at = $8, retry_config = $9, updated_at = now()
		WHERE id = $10 AND client_id = $11
		RETURNING %s`, campaignCols)

	var row campaignRow
	err := r.db.QueryRowxContext(ctx, query,
		c.Name, c.ContactListID, c.TemplateID, c.Source,
		segmentRules, pq.StringArray(c.SegmentTags), c.SendRate, c.ScheduledAt, retryJSON,
		c.ID, c.ClientID,
	).StructScan(&row)
	if err == sql.ErrNoRows {
		return nil, domain.ErrCampaignNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to update campaign: %w", err)
	}
	return row.toDomain(), nil
}

func (r *CampaignRepository) Delete(ctx context.Context, id, clientID uuid.UUID) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM campaigns WHERE id = $1 AND client_id = $2 AND status = 'draft'`, id, clientID)
	if err != nil {
		return fmt.Errorf("failed to delete campaign: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrCampaignNotFound
	}
	return nil
}

// === Variants ===

func (r *CampaignRepository) SetVariants(ctx context.Context, campaignID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM campaign_variants WHERE campaign_id = $1`, campaignID); err != nil {
		return nil, err
	}

	var result []domain.Variant
	for _, v := range variants {
		v.ID = uuid.New()
		v.CampaignID = campaignID
		query := `INSERT INTO campaign_variants (id, campaign_id, name, template_id, percentage, is_winner, is_control)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`
		if _, err := tx.ExecContext(ctx, query, v.ID, campaignID, v.Name, v.TemplateID, v.Percentage, v.IsWinner, v.IsControl); err != nil {
			return nil, err
		}
		result = append(result, v)
	}

	return result, tx.Commit()
}

func (r *CampaignRepository) GetVariants(ctx context.Context, campaignID uuid.UUID) ([]domain.Variant, error) {
	query := `SELECT id, campaign_id, name, template_id, percentage, is_winner, is_control, sent_count, delivered_count, failed_count
		FROM campaign_variants WHERE campaign_id = $1 ORDER BY percentage DESC`
	rows, err := r.db.QueryxContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var variants []domain.Variant
	for rows.Next() {
		var v domain.Variant
		if err := rows.Scan(&v.ID, &v.CampaignID, &v.Name, &v.TemplateID, &v.Percentage, &v.IsWinner, &v.IsControl, &v.SentCount, &v.DeliveredCount, &v.FailedCount); err != nil {
			return nil, err
		}
		variants = append(variants, v)
	}
	return variants, nil
}

// === A/B Config ===

func (r *CampaignRepository) SetABConfig(ctx context.Context, cfg *domain.ABConfig) (*domain.ABConfig, error) {
	query := `INSERT INTO campaign_ab_config (campaign_id, metric, test_duration_hours, auto_select_winner)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (campaign_id) DO UPDATE SET metric = $2, test_duration_hours = $3, auto_select_winner = $4`
	_, err := r.db.ExecContext(ctx, query, cfg.CampaignID, cfg.Metric, cfg.TestDurationHours, cfg.AutoSelectWinner)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func (r *CampaignRepository) GetABConfig(ctx context.Context, campaignID uuid.UUID) (*domain.ABConfig, error) {
	query := `SELECT campaign_id, metric, test_duration_hours, auto_select_winner, winner_variant_id, winner_selected_at
		FROM campaign_ab_config WHERE campaign_id = $1`
	var cfg domain.ABConfig
	var winnerID *uuid.UUID
	var winnerAt sql.NullTime
	err := r.db.QueryRowContext(ctx, query, campaignID).Scan(&cfg.CampaignID, &cfg.Metric, &cfg.TestDurationHours, &cfg.AutoSelectWinner, &winnerID, &winnerAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.WinnerVariantID = winnerID
	if winnerAt.Valid {
		cfg.WinnerSelectedAt = &winnerAt.Time
	}
	return &cfg, nil
}

func (r *CampaignRepository) SelectWinner(ctx context.Context, campaignID, variantID uuid.UUID) error {
	// Mark winner in variants
	if _, err := r.db.ExecContext(ctx, `UPDATE campaign_variants SET is_winner = (id = $1) WHERE campaign_id = $2`, variantID, campaignID); err != nil {
		return err
	}
	// Update AB config
	_, err := r.db.ExecContext(ctx, `UPDATE campaign_ab_config SET winner_variant_id = $1, winner_selected_at = now() WHERE campaign_id = $2`, variantID, campaignID)
	return err
}

// GetRunningCampaigns returns campaigns that are in 'running' status.
func (r *CampaignRepository) GetRunningCampaigns(ctx context.Context) ([]*domain.Campaign, error) {
	query := fmt.Sprintf(`SELECT %s FROM campaigns WHERE status = 'running'`, campaignCols)
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var campaigns []*domain.Campaign
	for rows.Next() {
		var row campaignRow
		if err := rows.StructScan(&row); err != nil {
			return nil, err
		}
		campaigns = append(campaigns, row.toDomain())
	}
	return campaigns, nil
}
```

- [ ] **Step 2: Write RecipientRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

type RecipientRepository struct {
	db *sqlx.DB
}

func NewRecipientRepository(db *sqlx.DB) *RecipientRepository {
	return &RecipientRepository{db: db}
}

func (r *RecipientRepository) BulkInsert(ctx context.Context, recipients []*domain.Recipient) error {
	if len(recipients) == 0 {
		return nil
	}

	query := `INSERT INTO campaign_recipients (id, campaign_id, contact_id, phone, variant_id, status) VALUES `
	var values []string
	var args []interface{}
	argIdx := 1

	for _, rec := range recipients {
		values = append(values, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
			argIdx, argIdx+1, argIdx+2, argIdx+3, argIdx+4, argIdx+5))
		args = append(args, rec.ID, rec.CampaignID, rec.ContactID, rec.Phone, rec.VariantID, rec.Status)
		argIdx += 6
	}

	query += strings.Join(values, ", ")
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *RecipientRepository) GetPendingBatch(ctx context.Context, campaignID uuid.UUID, limit int) ([]*domain.Recipient, error) {
	query := `SELECT id, campaign_id, contact_id, phone, variant_id, status, message_id, retry_count, last_retry_at, created_at, updated_at
		FROM campaign_recipients WHERE campaign_id = $1 AND status = 'pending' LIMIT $2`

	rows, err := r.db.QueryxContext(ctx, query, campaignID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []*domain.Recipient
	for rows.Next() {
		rec := &domain.Recipient{}
		var msgID *uuid.UUID
		var lastRetry sql.NullTime
		var createdAt, updatedAt sql.NullTime
		err := rows.Scan(&rec.ID, &rec.CampaignID, &rec.ContactID, &rec.Phone, &rec.VariantID,
			&rec.Status, &msgID, &rec.RetryCount, &lastRetry, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		rec.MessageID = msgID
		if lastRetry.Valid {
			rec.LastRetryAt = &lastRetry.Time
		}
		if createdAt.Valid {
			rec.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			rec.UpdatedAt = updatedAt.Time
		}
		recipients = append(recipients, rec)
	}
	return recipients, nil
}

func (r *RecipientRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string, messageID *uuid.UUID) error {
	query := `UPDATE campaign_recipients SET status = $1, message_id = $2, updated_at = now() WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, messageID, id)
	return err
}

func (r *RecipientRepository) UpdateStatusByMessageID(ctx context.Context, messageID uuid.UUID, status string) error {
	query := `UPDATE campaign_recipients SET status = $1, updated_at = now() WHERE message_id = $2`
	_, err := r.db.ExecContext(ctx, query, status, messageID)
	return err
}

func (r *RecipientRepository) CancelPending(ctx context.Context, campaignID uuid.UUID) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE campaign_recipients SET status = 'cancelled', updated_at = now() WHERE campaign_id = $1 AND status = 'pending'`,
		campaignID)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *RecipientRepository) GetFailedForRetry(ctx context.Context, campaignID uuid.UUID, maxRetries int) ([]*domain.Recipient, error) {
	query := `SELECT id, campaign_id, contact_id, phone, variant_id, status, message_id, retry_count, last_retry_at, created_at, updated_at
		FROM campaign_recipients WHERE campaign_id = $1 AND status = 'failed' AND retry_count < $2`

	rows, err := r.db.QueryxContext(ctx, query, campaignID, maxRetries)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var recipients []*domain.Recipient
	for rows.Next() {
		rec := &domain.Recipient{}
		var msgID *uuid.UUID
		var lastRetry sql.NullTime
		var createdAt, updatedAt sql.NullTime
		err := rows.Scan(&rec.ID, &rec.CampaignID, &rec.ContactID, &rec.Phone, &rec.VariantID,
			&rec.Status, &msgID, &rec.RetryCount, &lastRetry, &createdAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		rec.MessageID = msgID
		if lastRetry.Valid {
			rec.LastRetryAt = &lastRetry.Time
		}
		if createdAt.Valid {
			rec.CreatedAt = createdAt.Time
		}
		if updatedAt.Valid {
			rec.UpdatedAt = updatedAt.Time
		}
		recipients = append(recipients, rec)
	}
	return recipients, nil
}

func (r *RecipientRepository) IncrementRetry(ctx context.Context, id uuid.UUID) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE campaign_recipients SET retry_count = retry_count + 1, last_retry_at = now(), status = 'retry', updated_at = now() WHERE id = $1`, id)
	return err
}

func (r *RecipientRepository) CountByStatus(ctx context.Context, campaignID uuid.UUID) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM campaign_recipients WHERE campaign_id = $1 GROUP BY status`
	rows, err := r.db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[string]int)
	for rows.Next() {
		var status string
		var count int
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[status] = count
	}
	return counts, nil
}

func (r *RecipientRepository) CountTotal(ctx context.Context, campaignID uuid.UUID) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM campaign_recipients WHERE campaign_id = $1`, campaignID).Scan(&count)
	return count, err
}
```

- [ ] **Step 3: Write StatsRepository**

```go
package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
)

type StatsRepository struct {
	db *sqlx.DB
}

func NewStatsRepository(db *sqlx.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

func (r *StatsRepository) InsertSnapshot(ctx context.Context, snap *domain.StatsSnapshot) error {
	query := `INSERT INTO campaign_stats_snapshots (campaign_id, variant_id, snapshot_at, sent, delivered, failed, pending, avg_delivery_time_ms, cost)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`
	_, err := r.db.ExecContext(ctx, query,
		snap.CampaignID, snap.VariantID, snap.SnapshotAt,
		snap.Sent, snap.Delivered, snap.Failed, snap.Pending,
		snap.AvgDeliveryTimeMs, snap.Cost)
	return err
}

func (r *StatsRepository) GetLatestSnapshot(ctx context.Context, campaignID uuid.UUID) (*domain.StatsSnapshot, error) {
	query := `SELECT campaign_id, variant_id, snapshot_at, sent, delivered, failed, pending, avg_delivery_time_ms, cost
		FROM campaign_stats_snapshots WHERE campaign_id = $1 AND variant_id IS NULL ORDER BY snapshot_at DESC LIMIT 1`

	var snap domain.StatsSnapshot
	var variantID *uuid.UUID
	err := r.db.QueryRowContext(ctx, query, campaignID).Scan(
		&snap.CampaignID, &variantID, &snap.SnapshotAt,
		&snap.Sent, &snap.Delivered, &snap.Failed, &snap.Pending,
		&snap.AvgDeliveryTimeMs, &snap.Cost)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.VariantID = variantID
	return &snap, nil
}

// GetTimeline returns time-series data for a campaign.
func (r *StatsRepository) GetTimeline(ctx context.Context, campaignID uuid.UUID, interval string) ([]time.Time, []int, error) {
	truncInterval := "minute"
	switch interval {
	case "5min":
		truncInterval = "5 minutes"
	case "1hour":
		truncInterval = "hour"
	}

	// Use campaign_recipients updated_at for timeline
	query := `SELECT date_trunc($1, updated_at) AS ts, COUNT(*)
		FROM campaign_recipients
		WHERE campaign_id = $2 AND status IN ('delivered', 'sent', 'failed')
		GROUP BY ts ORDER BY ts`

	rows, err := r.db.QueryContext(ctx, query, truncInterval, campaignID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var timestamps []time.Time
	var values []int
	for rows.Next() {
		var ts time.Time
		var val int
		if err := rows.Scan(&ts, &val); err != nil {
			return nil, nil, err
		}
		timestamps = append(timestamps, ts)
		values = append(values, val)
	}
	return timestamps, values, nil
}

// GetHeatmap returns delivery heatmap data.
func (r *StatsRepository) GetHeatmap(ctx context.Context, campaignID uuid.UUID) ([]domain.HeatmapCell, error) {
	query := `SELECT EXTRACT(DOW FROM updated_at)::int AS day_of_week,
		EXTRACT(HOUR FROM updated_at)::int AS hour,
		COUNT(*) FILTER (WHERE status = 'delivered') AS delivered_count,
		CASE WHEN COUNT(*) > 0 THEN COUNT(*) FILTER (WHERE status = 'delivered')::float / COUNT(*)::float ELSE 0 END AS delivery_rate
		FROM campaign_recipients WHERE campaign_id = $1
		GROUP BY day_of_week, hour ORDER BY day_of_week, hour`

	rows, err := r.db.QueryContext(ctx, query, campaignID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cells []domain.HeatmapCell
	for rows.Next() {
		var cell domain.HeatmapCell
		if err := rows.Scan(&cell.DayOfWeek, &cell.Hour, &cell.DeliveredCount, &cell.DeliveryRate); err != nil {
			return nil, err
		}
		cells = append(cells, cell)
	}
	return cells, nil
}

type HeatmapCell = domain.HeatmapCell
```

Add the HeatmapCell to domain models (add to `internal/services/campaign/domain/models.go`):

```go
type HeatmapCell struct {
	DayOfWeek      int
	Hour           int
	DeliveredCount int
	DeliveryRate   float64
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/services/campaign/infrastructure/repository/
git commit -m "feat(campaigns): add campaign, recipient, and stats repositories"
```

---

## Task 13: campaign-service — Application Layer

**Files:**
- Create: `internal/services/campaign/application/campaign_service.go`

This is the core business logic. Due to size, this is split into the main service only; background workers are in separate tasks.

- [ ] **Step 1: Write CampaignService**

```go
package application

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/smpp-server/smpp-server/internal/services/campaign/domain"
	"github.com/smpp-server/smpp-server/internal/services/campaign/infrastructure/repository"
)

type CampaignService struct {
	campaignRepo  *repository.CampaignRepository
	recipientRepo *repository.RecipientRepository
	statsRepo     *repository.StatsRepository
	logger        zerolog.Logger
}

func NewCampaignService(
	campaignRepo *repository.CampaignRepository,
	recipientRepo *repository.RecipientRepository,
	statsRepo *repository.StatsRepository,
) *CampaignService {
	return &CampaignService{
		campaignRepo:  campaignRepo,
		recipientRepo: recipientRepo,
		statsRepo:     statsRepo,
		logger:        log.With().Str("component", "campaign-service").Logger(),
	}
}

func (s *CampaignService) CreateCampaign(ctx context.Context, clientID uuid.UUID, name string, contactListID uuid.UUID, templateID *uuid.UUID, source, segmentRules string, segmentTags []string, sendRate int, scheduledAt *interface{}) (*domain.Campaign, error) {
	c := &domain.Campaign{
		ID:            uuid.New(),
		ClientID:      clientID,
		Name:          name,
		Status:        domain.StatusDraft,
		ContactListID: contactListID,
		TemplateID:    templateID,
		Source:        source,
		SegmentRules:  segmentRules,
		SegmentTags:   segmentTags,
		SendRate:      sendRate,
	}
	return s.campaignRepo.Create(ctx, c)
}

func (s *CampaignService) GetCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	// Load variants and AB config
	variants, _ := s.campaignRepo.GetVariants(ctx, id)
	campaign.Variants = variants
	abConfig, _ := s.campaignRepo.GetABConfig(ctx, id)
	campaign.ABConfig = abConfig
	return campaign, nil
}

func (s *CampaignService) ListCampaigns(ctx context.Context, clientID uuid.UUID, status string, limit, offset int) ([]*domain.Campaign, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return s.campaignRepo.List(ctx, clientID, status, limit, offset)
}

func (s *CampaignService) UpdateCampaign(ctx context.Context, id, clientID uuid.UUID, name string, contactListID uuid.UUID, templateID *uuid.UUID, source, segmentRules string, segmentTags []string, sendRate int) (*domain.Campaign, error) {
	existing, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if existing.Status != domain.StatusDraft {
		return nil, domain.ErrCampaignNotDraft
	}
	existing.Name = name
	existing.ContactListID = contactListID
	existing.TemplateID = templateID
	existing.Source = source
	existing.SegmentRules = segmentRules
	existing.SegmentTags = segmentTags
	existing.SendRate = sendRate
	return s.campaignRepo.Update(ctx, existing)
}

func (s *CampaignService) DeleteCampaign(ctx context.Context, id, clientID uuid.UUID) error {
	return s.campaignRepo.Delete(ctx, id, clientID)
}

// Lifecycle

func (s *CampaignService) LaunchCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != domain.StatusDraft && campaign.Status != domain.StatusScheduled {
		return nil, domain.ErrCampaignNotDraft
	}
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusMaterializing); err != nil {
		return nil, err
	}
	campaign.Status = domain.StatusMaterializing
	return campaign, nil
}

func (s *CampaignService) PauseCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != domain.StatusRunning {
		return nil, domain.ErrCampaignNotRunning
	}
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusPaused); err != nil {
		return nil, err
	}
	campaign.Status = domain.StatusPaused
	return campaign, nil
}

func (s *CampaignService) ResumeCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != domain.StatusPaused {
		return nil, domain.ErrCampaignNotPaused
	}
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusRunning); err != nil {
		return nil, err
	}
	campaign.Status = domain.StatusRunning
	return campaign, nil
}

func (s *CampaignService) CancelCampaign(ctx context.Context, id, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, id, clientID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != domain.StatusRunning && campaign.Status != domain.StatusPaused && campaign.Status != domain.StatusMaterializing {
		return nil, fmt.Errorf("%w: can only cancel running, paused, or materializing campaigns", domain.ErrInvalidCampaignStatus)
	}
	s.recipientRepo.CancelPending(ctx, id)
	if err := s.campaignRepo.UpdateStatus(ctx, id, domain.StatusCancelled); err != nil {
		return nil, err
	}
	campaign.Status = domain.StatusCancelled
	return campaign, nil
}

// A/B Testing

func (s *CampaignService) SetVariants(ctx context.Context, campaignID, clientID uuid.UUID, variants []domain.Variant) ([]domain.Variant, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}
	if campaign.Status != domain.StatusDraft {
		return nil, domain.ErrCampaignNotDraft
	}
	if len(variants) < 2 {
		return nil, domain.ErrTooFewVariants
	}
	if len(variants) > 5 {
		return nil, domain.ErrTooManyVariants
	}
	sum := 0
	for _, v := range variants {
		sum += v.Percentage
	}
	if sum != 100 {
		return nil, domain.ErrVariantPercentageSum
	}
	return s.campaignRepo.SetVariants(ctx, campaignID, variants)
}

func (s *CampaignService) SetABConfig(ctx context.Context, campaignID, clientID uuid.UUID, metric string, durationHours int, autoSelect bool) (*domain.ABConfig, error) {
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}
	cfg := &domain.ABConfig{
		CampaignID:        campaignID,
		Metric:            metric,
		TestDurationHours: durationHours,
		AutoSelectWinner:  autoSelect,
	}
	return s.campaignRepo.SetABConfig(ctx, cfg)
}

func (s *CampaignService) SelectWinner(ctx context.Context, campaignID, clientID, variantID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}
	abConfig, _ := s.campaignRepo.GetABConfig(ctx, campaignID)
	if abConfig != nil && abConfig.WinnerVariantID != nil {
		return nil, domain.ErrWinnerAlreadySelected
	}
	if err := s.campaignRepo.SelectWinner(ctx, campaignID, variantID); err != nil {
		return nil, err
	}
	return s.GetCampaign(ctx, campaignID, campaign.ClientID)
}

// Retry

func (s *CampaignService) SetRetryConfig(ctx context.Context, campaignID, clientID uuid.UUID, cfg *domain.RetryConfig) (*domain.RetryConfig, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}
	campaign.RetryConfig = cfg
	retryJSON, _ := json.Marshal(cfg)
	campaign.SegmentRules = campaign.SegmentRules // keep
	_ = retryJSON // used in update
	if _, err := s.campaignRepo.Update(ctx, campaign); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (s *CampaignService) RetryFailed(ctx context.Context, campaignID, clientID uuid.UUID) (*domain.Campaign, error) {
	campaign, err := s.campaignRepo.GetByID(ctx, campaignID, clientID)
	if err != nil {
		return nil, err
	}
	failed, err := s.recipientRepo.GetFailedForRetry(ctx, campaignID, 999)
	if err != nil {
		return nil, err
	}
	if len(failed) == 0 {
		return nil, domain.ErrNoFailedRecipients
	}
	for _, rec := range failed {
		s.recipientRepo.IncrementRetry(ctx, rec.ID)
	}
	// Set campaign back to running to process retries
	if campaign.Status == domain.StatusCompleted {
		s.campaignRepo.UpdateStatus(ctx, campaignID, domain.StatusRunning)
	}
	return s.GetCampaign(ctx, campaignID, clientID)
}

// Analytics

func (s *CampaignService) GetCampaignStats(ctx context.Context, campaignID, clientID uuid.UUID) (*domain.StatsSnapshot, map[string]int, []domain.Variant, error) {
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, nil, nil, err
	}
	snap, _ := s.statsRepo.GetLatestSnapshot(ctx, campaignID)
	counts, _ := s.recipientRepo.CountByStatus(ctx, campaignID)
	variants, _ := s.campaignRepo.GetVariants(ctx, campaignID)
	return snap, counts, variants, nil
}

func (s *CampaignService) GetTimeline(ctx context.Context, campaignID, clientID uuid.UUID, interval string) ([]interface{}, error) {
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}
	timestamps, values, err := s.statsRepo.GetTimeline(ctx, campaignID, interval)
	if err != nil {
		return nil, err
	}
	var points []interface{}
	for i, ts := range timestamps {
		points = append(points, map[string]interface{}{"timestamp": ts, "value": values[i]})
	}
	return points, nil
}

func (s *CampaignService) GetHeatmap(ctx context.Context, campaignID, clientID uuid.UUID) ([]domain.HeatmapCell, error) {
	if _, err := s.campaignRepo.GetByID(ctx, campaignID, clientID); err != nil {
		return nil, err
	}
	return s.statsRepo.GetHeatmap(ctx, campaignID)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/application/campaign_service.go
git commit -m "feat(campaigns): add campaign application service with CRUD, lifecycle, A/B, retry, analytics"
```

---

## Task 14: campaign-service — gRPC Server & Mappers

**Files:**
- Create: `internal/services/campaign/grpc/server.go`
- Create: `internal/services/campaign/grpc/mappers.go`

Due to the very large number of RPCs, implement the core CRUD + lifecycle + A/B + retry + basic analytics endpoints. The server follows the same pattern as contact-service gRPC server (Task 9). Each RPC validates inputs, parses UUIDs, calls the service, and maps domain → proto.

- [ ] **Step 1: Write gRPC server and mappers**

The implementation follows the exact pattern from `internal/services/template/grpc/server.go` — embed `UnimplementedCampaignServiceServer`, wrap `CampaignService`, use `mapError` for domain errors → gRPC status codes.

Key mappings:
- `ErrCampaignNotFound` → `codes.NotFound`
- `ErrCampaignNotDraft`, `ErrCampaignNotRunning`, `ErrCampaignNotPaused`, `ErrInvalidCampaignStatus` → `codes.FailedPrecondition`
- `ErrVariantPercentageSum`, `ErrTooFewVariants`, `ErrTooManyVariants` → `codes.InvalidArgument`
- `ErrWinnerAlreadySelected` → `codes.AlreadyExists`

Proto mappers convert `domain.Campaign` → `campaignv1.Campaign`, `domain.Variant` → `campaignv1.Variant`, etc. using `timestamppb.New()` for timestamps and JSON strings for `segment_rules`/`retry_config`.

- [ ] **Step 2: Commit**

```bash
git add internal/services/campaign/grpc/
git commit -m "feat(campaigns): add campaign gRPC server and mappers"
```

---

## Task 15: campaign-service — Main Entrypoint

**Files:**
- Create: `cmd/services/campaign-service/main.go`

- [ ] **Step 1: Write service entrypoint**

Same pattern as contact-service (Task 10), but on port 5013 (gRPC) and 2131 (metrics). Additionally initializes Kafka producer (for `sms.outgoing`) and consumer (for `sms.status`), and the contact-service gRPC client (for `StreamSegment`).

```go
// Key differences from contact-service main.go:
// 1. gRPC port: ":5013"
// 2. Metrics port: ":2131"
// 3. Kafka producer for sms.outgoing
// 4. Kafka consumer for sms.status (consumer group: campaign-status-consumer)
// 5. gRPC client to contact-service for StreamSegment
// 6. Background goroutines: stats ticker (30s), retry ticker (60s)
```

- [ ] **Step 2: Commit**

```bash
git add cmd/services/campaign-service/
git commit -m "feat(campaigns): add campaign-service entrypoint with Kafka integration"
```

---

## Task 16: Gateway Integration — Portal Gateway

**Files:**
- Create: `internal/gateway/portal/handlers/contacts.go`
- Create: `internal/gateway/portal/handlers/campaigns.go`
- Modify: `internal/gateway/portal/clients.go`
- Modify: `internal/gateway/portal/router/router.go`
- Modify: `cmd/portal-gateway/main.go`

- [ ] **Step 1: Add ContactClient and CampaignClient to ServiceClients**

In `internal/gateway/portal/clients.go`, add:
```go
// Add imports:
contactv1 "github.com/smpp-server/smpp-server/api/proto/contactv1"
campaignv1 "github.com/smpp-server/smpp-server/api/proto/campaignv1"

// Add to ServiceClients struct:
ContactClient    contactv1.ContactServiceClient
CampaignClient   campaignv1.CampaignServiceClient

// Add to ServiceAddresses struct:
Contact   string
Campaign  string

// Add connection blocks in NewServiceClients (same pattern as existing):
// Contact Service
if addresses.Contact != "" { ... }
// Campaign Service
if addresses.Campaign != "" { ... }
```

- [ ] **Step 2: Write contact handlers**

Create `internal/gateway/portal/handlers/contacts.go` following the pattern from `internal/gateway/portal/handlers/webhooks.go`:
- `ContactHandlers` struct with `contactv1.ContactServiceClient`
- `NewContactHandlers(client contactv1.ContactServiceClient) *ContactHandlers`
- Methods: `CreateContactList`, `ListContactLists`, `GetContactList`, `UpdateContactList`, `DeleteContactList`
- Methods: `SetListAttributes`, `GetListAttributes`
- Methods: `CreateContact`, `ListContacts`, `UpdateContact`, `DeleteContact`, `BatchUpsertContacts`
- Methods: `AddTags`, `RemoveTags`, `ListTags`
- Methods: `UploadImport` (multipart file handling), `StartImport`, `GetImportStatus`, `ListImports`
- Methods: `PreviewSegment`
- Each method extracts client_id from request context, parses path variables via `mux.Vars(r)`, calls gRPC, uses `respondJSON`/`respondGRPCError`

- [ ] **Step 3: Write campaign handlers**

Create `internal/gateway/portal/handlers/campaigns.go`:
- `CampaignHandlers` struct with `campaignv1.CampaignServiceClient`
- Methods matching all campaign HTTP endpoints from spec section 7

- [ ] **Step 4: Add routes to router**

In `internal/gateway/portal/router/router.go`:
- Add `contactHandlers *handlers.ContactHandlers` and `campaignHandlers *handlers.CampaignHandlers` to `SetupRouter` parameters
- Add route groups under `protected`:

```go
// Contact Lists
contactLists := protected.PathPrefix("/contact-lists").Subrouter()
contactLists.HandleFunc("", contactHandlers.CreateContactList).Methods("POST")
contactLists.HandleFunc("", contactHandlers.ListContactLists).Methods("GET")
contactLists.HandleFunc("/{id}", contactHandlers.GetContactList).Methods("GET")
contactLists.HandleFunc("/{id}", contactHandlers.UpdateContactList).Methods("PUT")
contactLists.HandleFunc("/{id}", contactHandlers.DeleteContactList).Methods("DELETE")
contactLists.HandleFunc("/{id}/attributes", contactHandlers.SetListAttributes).Methods("PUT")
contactLists.HandleFunc("/{id}/attributes", contactHandlers.GetListAttributes).Methods("GET")
contactLists.HandleFunc("/{id}/contacts", contactHandlers.CreateContact).Methods("POST")
contactLists.HandleFunc("/{id}/contacts", contactHandlers.ListContacts).Methods("GET")
contactLists.HandleFunc("/{id}/contacts/batch", contactHandlers.BatchUpsertContacts).Methods("POST")
contactLists.HandleFunc("/{id}/contacts/tags", contactHandlers.AddTags).Methods("POST")
contactLists.HandleFunc("/{id}/contacts/tags", contactHandlers.RemoveTags).Methods("DELETE")
contactLists.HandleFunc("/{id}/contacts/{cid}", contactHandlers.UpdateContact).Methods("PUT")
contactLists.HandleFunc("/{id}/contacts/{cid}", contactHandlers.DeleteContact).Methods("DELETE")
contactLists.HandleFunc("/{id}/tags", contactHandlers.ListTags).Methods("GET")
contactLists.HandleFunc("/{id}/imports/upload", contactHandlers.UploadImport).Methods("POST")
contactLists.HandleFunc("/{id}/imports/{iid}/start", contactHandlers.StartImport).Methods("POST")
contactLists.HandleFunc("/{id}/imports/{iid}", contactHandlers.GetImportStatus).Methods("GET")
contactLists.HandleFunc("/{id}/imports", contactHandlers.ListImports).Methods("GET")

// Campaigns
campaigns := protected.PathPrefix("/campaigns").Subrouter()
campaigns.HandleFunc("", campaignHandlers.CreateCampaign).Methods("POST")
campaigns.HandleFunc("", campaignHandlers.ListCampaigns).Methods("GET")
campaigns.HandleFunc("/{id}", campaignHandlers.GetCampaign).Methods("GET")
campaigns.HandleFunc("/{id}", campaignHandlers.UpdateCampaign).Methods("PUT")
campaigns.HandleFunc("/{id}", campaignHandlers.DeleteCampaign).Methods("DELETE")
campaigns.HandleFunc("/{id}/launch", campaignHandlers.LaunchCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/pause", campaignHandlers.PauseCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/resume", campaignHandlers.ResumeCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/cancel", campaignHandlers.CancelCampaign).Methods("POST")
campaigns.HandleFunc("/{id}/variants", campaignHandlers.SetVariants).Methods("PUT")
campaigns.HandleFunc("/{id}/ab-config", campaignHandlers.SetABConfig).Methods("PUT")
campaigns.HandleFunc("/{id}/select-winner", campaignHandlers.SelectWinner).Methods("POST")
campaigns.HandleFunc("/{id}/retry-config", campaignHandlers.SetRetryConfig).Methods("PUT")
campaigns.HandleFunc("/{id}/retry", campaignHandlers.RetryFailed).Methods("POST")
campaigns.HandleFunc("/{id}/stats", campaignHandlers.GetCampaignStats).Methods("GET")
campaigns.HandleFunc("/{id}/timeline", campaignHandlers.GetTimeline).Methods("GET")
campaigns.HandleFunc("/{id}/variants/compare", campaignHandlers.GetVariantComparison).Methods("GET")
campaigns.HandleFunc("/{id}/heatmap", campaignHandlers.GetHeatmap).Methods("GET")
campaigns.HandleFunc("/{id}/optimal-time", campaignHandlers.GetOptimalSendTime).Methods("GET")
campaigns.HandleFunc("/{id}/report", campaignHandlers.ExportReport).Methods("GET")
```

- [ ] **Step 5: Wire in portal-gateway main.go**

In `cmd/portal-gateway/main.go`:
- Add `Contact` and `Campaign` to `serviceAddresses`
- Create `contactHandlers` and `campaignHandlers`
- Pass them to `SetupRouter`

- [ ] **Step 6: Commit**

```bash
git add internal/gateway/portal/handlers/contacts.go internal/gateway/portal/handlers/campaigns.go \
  internal/gateway/portal/clients.go internal/gateway/portal/router/router.go cmd/portal-gateway/main.go
git commit -m "feat(gateway): add contact and campaign handlers with routes to portal gateway"
```

---

## Task 17: Docker Compose — New Services

**Files:**
- Modify: `deployments/docker-compose.yml`

- [ ] **Step 1: Add contact-service and campaign-service**

Add after template-service block:

```yaml
  # Contact Service
  contact-service:
    build:
      context: ..
      dockerfile: deployments/docker/service-base.Dockerfile
      args:
        SERVICE_NAME: contact-service
        SERVICE_PATH: cmd/services/contact-service
    container_name: contact-service
    ports:
      - "5012:5012"
      - "2130:2130"
    environment:
      - SERVICE_NAME=contact-service
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=smpp
      - POSTGRES_PASSWORD=smpp_password
      - POSTGRES_DB=smpp_db
    networks:
      - smpp-network
    depends_on:
      postgres:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:2130/health"]
      interval: 10s
      timeout: 5s
      retries: 3

  # Campaign Service
  campaign-service:
    build:
      context: ..
      dockerfile: deployments/docker/service-base.Dockerfile
      args:
        SERVICE_NAME: campaign-service
        SERVICE_PATH: cmd/services/campaign-service
    container_name: campaign-service
    ports:
      - "5013:5013"
      - "2131:2131"
    environment:
      - SERVICE_NAME=campaign-service
      - POSTGRES_HOST=postgres
      - POSTGRES_PORT=5432
      - POSTGRES_USER=smpp
      - POSTGRES_PASSWORD=smpp_password
      - POSTGRES_DB=smpp_db
      - KAFKA_BROKERS=kafka:9092
      - CONTACT_SERVICE_ADDR=contact-service:5012
      - TEMPLATE_SERVICE_ADDR=template-service:9099
    networks:
      - smpp-network
    depends_on:
      postgres:
        condition: service_healthy
      kafka:
        condition: service_started
      contact-service:
        condition: service_healthy
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "--quiet", "--tries=1", "--spider", "http://localhost:2131/health"]
      interval: 10s
      timeout: 5s
      retries: 3
```

Also add to all gateway environment blocks:
```yaml
      - CONTACT_SERVICE_ADDR=contact-service:5012
      - CAMPAIGN_SERVICE_ADDR=campaign-service:5013
```

And add `contact-service` and `campaign-service` to gateway `depends_on`.

- [ ] **Step 2: Commit**

```bash
git add deployments/docker-compose.yml
git commit -m "feat(infra): add contact-service and campaign-service to docker-compose"
```

---

## Task 18: Frontend — API Client

**Files:**
- Create: `portal-frontend/src/api/contacts.ts`
- Create: `portal-frontend/src/api/campaigns.ts`

- [ ] **Step 1: Write contacts API client**

```typescript
// portal-frontend/src/api/contacts.ts
import { apiFetch } from './client';

export interface ContactList {
  id: string;
  client_id: string;
  name: string;
  description: string;
  contacts_count: number;
  created_at: string;
  updated_at: string;
}

export interface ContactAttribute {
  id: string;
  name: string;
  display_name: string;
  type: 'string' | 'number' | 'date' | 'boolean';
  required: boolean;
  position: number;
}

export interface Contact {
  id: string;
  contact_list_id: string;
  phone: string;
  attributes: Record<string, unknown>;
  tags: string[];
  created_at: string;
  updated_at: string;
}

export interface ImportJob {
  id: string;
  contact_list_id: string;
  file_name: string;
  status: 'pending' | 'processing' | 'completed' | 'failed';
  total_rows: number;
  imported_count: number;
  updated_count: number;
  error_count: number;
  errors: string;
  created_at: string;
  completed_at: string | null;
}

export const contactListsApi = {
  list: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return apiFetch<{ items: ContactList[]; total: number }>(`/contact-lists${qs}`);
  },
  create: (data: { name: string; description?: string }) =>
    apiFetch<ContactList>('/contact-lists', { method: 'POST', body: JSON.stringify(data) }),
  get: (id: string) => apiFetch<ContactList>(`/contact-lists/${id}`),
  update: (id: string, data: { name: string; description?: string }) =>
    apiFetch<ContactList>(`/contact-lists/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/contact-lists/${id}`, { method: 'DELETE' }),

  // Attributes
  getAttributes: (listId: string) =>
    apiFetch<{ attributes: ContactAttribute[] }>(`/contact-lists/${listId}/attributes`),
  setAttributes: (listId: string, attributes: Omit<ContactAttribute, 'id'>[]) =>
    apiFetch<{ attributes: ContactAttribute[] }>(`/contact-lists/${listId}/attributes`, {
      method: 'PUT',
      body: JSON.stringify({ attributes }),
    }),

  // Contacts
  listContacts: (listId: string, params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return apiFetch<{ contacts: Contact[]; total: number }>(`/contact-lists/${listId}/contacts${qs}`);
  },
  createContact: (listId: string, data: { phone: string; attributes?: Record<string, unknown>; tags?: string[] }) =>
    apiFetch<Contact>(`/contact-lists/${listId}/contacts`, { method: 'POST', body: JSON.stringify(data) }),
  updateContact: (listId: string, contactId: string, data: { phone?: string; attributes?: Record<string, unknown>; tags?: string[] }) =>
    apiFetch<Contact>(`/contact-lists/${listId}/contacts/${contactId}`, { method: 'PUT', body: JSON.stringify(data) }),
  deleteContact: (listId: string, contactId: string) =>
    apiFetch<void>(`/contact-lists/${listId}/contacts/${contactId}`, { method: 'DELETE' }),
  batchUpsert: (listId: string, contacts: { phone: string; attributes?: Record<string, unknown>; tags?: string[] }[]) =>
    apiFetch<{ created: number; updated: number; errors: number }>(`/contact-lists/${listId}/contacts/batch`, {
      method: 'POST',
      body: JSON.stringify({ contacts }),
    }),

  // Tags
  listTags: (listId: string) => apiFetch<{ tags: string[] }>(`/contact-lists/${listId}/tags`),
  addTags: (listId: string, contactIds: string[], tags: string[]) =>
    apiFetch<void>(`/contact-lists/${listId}/contacts/tags`, {
      method: 'POST',
      body: JSON.stringify({ contact_ids: contactIds, tags }),
    }),
  removeTags: (listId: string, contactIds: string[], tags: string[]) =>
    apiFetch<void>(`/contact-lists/${listId}/contacts/tags`, {
      method: 'DELETE',
      body: JSON.stringify({ contact_ids: contactIds, tags }),
    }),

  // Import
  uploadImport: (listId: string, file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    return apiFetch<{ import_id: string; preview: string[][] }>(`/contact-lists/${listId}/imports/upload`, {
      method: 'POST',
      body: formData,
      headers: {}, // Let browser set Content-Type with boundary
    });
  },
  startImport: (listId: string, importId: string, columnMapping: unknown) =>
    apiFetch<ImportJob>(`/contact-lists/${listId}/imports/${importId}/start`, {
      method: 'POST',
      body: JSON.stringify({ column_mapping: columnMapping }),
    }),
  getImportStatus: (listId: string, importId: string) =>
    apiFetch<ImportJob>(`/contact-lists/${listId}/imports/${importId}`),
  listImports: (listId: string) =>
    apiFetch<{ imports: ImportJob[]; total: number }>(`/contact-lists/${listId}/imports`),

  // Segmentation
  previewSegment: (listId: string, rules: unknown, tags?: string[]) =>
    apiFetch<{ count: number }>(`/contact-lists/${listId}/contacts/preview`, {
      method: 'POST',
      body: JSON.stringify({ rules, tags }),
    }),
};
```

- [ ] **Step 2: Write campaigns API client**

```typescript
// portal-frontend/src/api/campaigns.ts
import { apiFetch } from './client';

export interface Campaign {
  id: string;
  name: string;
  status: 'draft' | 'scheduled' | 'materializing' | 'running' | 'paused' | 'completed' | 'cancelled';
  contact_list_id: string;
  template_id: string;
  source: string;
  segment_rules: string;
  segment_tags: string[];
  send_rate: number;
  scheduled_at: string | null;
  started_at: string | null;
  completed_at: string | null;
  total_recipients: number;
  sent_count: number;
  delivered_count: number;
  failed_count: number;
  created_at: string;
  variants?: Variant[];
  ab_config?: ABConfig;
}

export interface Variant {
  id: string;
  name: string;
  template_id: string;
  percentage: number;
  is_winner: boolean;
  is_control: boolean;
  sent_count: number;
  delivered_count: number;
  failed_count: number;
}

export interface ABConfig {
  metric: string;
  test_duration_hours: number;
  auto_select_winner: boolean;
  winner_variant_id: string | null;
}

export interface CampaignStats {
  total_recipients: number;
  sent: number;
  delivered: number;
  failed: number;
  pending: number;
  delivery_rate: number;
  total_cost: number;
  per_variant: { variant_id: string; variant_name: string; sent: number; delivered: number; failed: number; delivery_rate: number }[];
}

export const campaignsApi = {
  list: (params?: Record<string, string>) => {
    const qs = params ? '?' + new URLSearchParams(params).toString() : '';
    return apiFetch<{ campaigns: Campaign[]; total: number }>(`/campaigns${qs}`);
  },
  create: (data: { name: string; contact_list_id: string; template_id?: string; source?: string; segment_rules?: string; segment_tags?: string[]; send_rate?: number }) =>
    apiFetch<Campaign>('/campaigns', { method: 'POST', body: JSON.stringify(data) }),
  get: (id: string) => apiFetch<Campaign>(`/campaigns/${id}`),
  update: (id: string, data: Record<string, unknown>) =>
    apiFetch<Campaign>(`/campaigns/${id}`, { method: 'PUT', body: JSON.stringify(data) }),
  remove: (id: string) => apiFetch<void>(`/campaigns/${id}`, { method: 'DELETE' }),

  // Lifecycle
  launch: (id: string) => apiFetch<Campaign>(`/campaigns/${id}/launch`, { method: 'POST' }),
  pause: (id: string) => apiFetch<Campaign>(`/campaigns/${id}/pause`, { method: 'POST' }),
  resume: (id: string) => apiFetch<Campaign>(`/campaigns/${id}/resume`, { method: 'POST' }),
  cancel: (id: string) => apiFetch<Campaign>(`/campaigns/${id}/cancel`, { method: 'POST' }),

  // A/B
  setVariants: (id: string, variants: { name: string; template_id: string; percentage: number; is_control?: boolean }[]) =>
    apiFetch<{ variants: Variant[] }>(`/campaigns/${id}/variants`, { method: 'PUT', body: JSON.stringify({ variants }) }),
  setABConfig: (id: string, config: { metric: string; test_duration_hours: number; auto_select_winner: boolean }) =>
    apiFetch<ABConfig>(`/campaigns/${id}/ab-config`, { method: 'PUT', body: JSON.stringify(config) }),
  selectWinner: (id: string, variantId: string) =>
    apiFetch<Campaign>(`/campaigns/${id}/select-winner`, { method: 'POST', body: JSON.stringify({ variant_id: variantId }) }),

  // Retry
  setRetryConfig: (id: string, config: { enabled: boolean; delay_hours: number; max_retries: number; alternative_template_id?: string }) =>
    apiFetch<unknown>(`/campaigns/${id}/retry-config`, { method: 'PUT', body: JSON.stringify(config) }),
  retryFailed: (id: string) => apiFetch<Campaign>(`/campaigns/${id}/retry`, { method: 'POST' }),

  // Analytics
  getStats: (id: string) => apiFetch<CampaignStats>(`/campaigns/${id}/stats`),
  getTimeline: (id: string, params: Record<string, string>) => {
    const qs = new URLSearchParams(params).toString();
    return apiFetch<{ points: { timestamp: string; value: number }[] }>(`/campaigns/${id}/timeline?${qs}`);
  },
  getVariantComparison: (id: string) => apiFetch<unknown>(`/campaigns/${id}/variants/compare`),
  getHeatmap: (id: string) => apiFetch<unknown>(`/campaigns/${id}/heatmap`),
  getOptimalTime: () => apiFetch<unknown>(`/campaigns/optimal-time`),
};
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/api/contacts.ts portal-frontend/src/api/campaigns.ts
git commit -m "feat(frontend): add contacts and campaigns API clients"
```

---

## Task 19: Frontend — Contact Lists Pages

**Files:**
- Create: `portal-frontend/src/pages/contacts/ContactListsPage.tsx`
- Create: `portal-frontend/src/pages/contacts/ContactListDetailPage.tsx`

- [ ] **Step 1: Write ContactListsPage**

Table listing contact lists with columns: Name, Description, Contacts Count, Created, Actions (view/edit/delete). Create dialog for new list. Follow the pattern from existing pages (e.g., `ProvidersPage.tsx`) using `useState`, `useEffect`, API calls, Tailwind styling.

- [ ] **Step 2: Write ContactListDetailPage**

Dynamic table of contacts for a specific list. Features:
- Contacts table with phone, dynamic attribute columns (from `getAttributes`), tags
- Search by phone
- Filter by tags
- Add/edit/delete individual contacts (modal)
- Import button → navigates to `/contact-lists/{id}/import`
- Manage attributes button → modal for attribute configuration

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/pages/contacts/
git commit -m "feat(frontend): add contact lists and detail pages"
```

---

## Task 20: Frontend — Import Wizard

**Files:**
- Create: `portal-frontend/src/pages/contacts/ImportWizardPage.tsx`

- [ ] **Step 1: Write 4-step import wizard**

Step 1 (Upload): Drag-and-drop area for CSV/XLSX file upload. Calls `uploadImport`.
Step 2 (Mapping): Shows preview of first 5 rows. Dropdown per column to map to attributes or "phone". Create new attribute inline.
Step 3 (Progress): Calls `startImport`, then polls `getImportStatus` every 2 seconds. Shows progress bar.
Step 4 (Result): Shows imported/updated/errors counts. Link to download error log.

Use `useState` for wizard step tracking. Each step is a component rendered conditionally.

- [ ] **Step 2: Commit**

```bash
git add portal-frontend/src/pages/contacts/ImportWizardPage.tsx
git commit -m "feat(frontend): add CSV import wizard for contacts"
```

---

## Task 21: Frontend — Campaigns Pages

**Files:**
- Create: `portal-frontend/src/pages/campaigns/CampaignsPage.tsx`
- Create: `portal-frontend/src/pages/campaigns/CampaignWizardPage.tsx`
- Create: `portal-frontend/src/pages/campaigns/CampaignDetailPage.tsx`
- Create: `portal-frontend/src/components/SegmentBuilder.tsx`

- [ ] **Step 1: Write CampaignsPage (list)**

Table: Name, Status (badge), Contact List, Progress bar (sent/total), Created, Actions (view/edit/launch/pause/cancel/delete). Status badge colors: draft=gray, running=blue, paused=yellow, completed=green, cancelled=red.

- [ ] **Step 2: Write SegmentBuilder component**

AND/OR tree builder:
- Top-level operator toggle (AND/OR)
- Add Condition button → row with: field dropdown (from attributes), operator dropdown (filtered by type), value input
- Add Group button → nested group with its own operator and conditions
- Max 3 levels deep
- Debounced `previewSegment` call → shows count badge
- `onChange` callback with the segment rules JSON

- [ ] **Step 3: Write CampaignWizardPage (5 steps)**

Step 1 (Basics): Name, contact list dropdown, sender ID
Step 2 (Audience): SegmentBuilder + tag filter + audience count preview
Step 3 (Message): Template selector, variable preview, optional A/B variant setup
Step 4 (Schedule & Retry): Now/scheduled toggle, send rate, auto-retry config
Step 5 (Confirm): Summary of all settings, estimated cost, Launch button

- [ ] **Step 4: Write CampaignDetailPage**

Header: Campaign name, status badge, action buttons (pause/resume/cancel)
Tabs:
- Overview: Stats cards (total, sent, delivered, failed, delivery_rate)
- Timeline: Placeholder for chart (data from `getTimeline`)
- A/B Results: Variant comparison table (from `getVariantComparison`)
- Recipients: Paginated table (phone, status, variant)

- [ ] **Step 5: Commit**

```bash
git add portal-frontend/src/pages/campaigns/ portal-frontend/src/components/SegmentBuilder.tsx
git commit -m "feat(frontend): add campaigns pages, wizard, segment builder"
```

---

## Task 22: Frontend — Navigation & Routing

**Files:**
- Modify: `portal-frontend/src/components/layout/UserLayout.tsx`
- Modify: `portal-frontend/src/App.tsx`

- [ ] **Step 1: Add nav items to UserLayout**

In `portal-frontend/src/components/layout/UserLayout.tsx`, add after the "Сообщения" entry in `USER_NAV`:

```typescript
{ path: '/contact-lists', label: 'Контакты' },
{ path: '/campaigns', label: 'Рассылки' },
```

- [ ] **Step 2: Add routes to App.tsx**

In `portal-frontend/src/App.tsx`:
- Import new pages
- Add routes inside `<Route element={<RequireAuth />}>`:

```tsx
<Route path="/contact-lists" element={<ContactListsPage />} />
<Route path="/contact-lists/:id" element={<ContactListDetailPage />} />
<Route path="/contact-lists/:id/import" element={<ImportWizardPage />} />
<Route path="/campaigns" element={<CampaignsPage />} />
<Route path="/campaigns/new" element={<CampaignWizardPage />} />
<Route path="/campaigns/:id" element={<CampaignDetailPage />} />
```

- [ ] **Step 3: Commit**

```bash
git add portal-frontend/src/components/layout/UserLayout.tsx portal-frontend/src/App.tsx
git commit -m "feat(frontend): add contact and campaign routes and navigation"
```

---

## Task 23: Final Verification

- [ ] **Step 1: Verify Go compilation**

```bash
cd /home/magomed/projects/sms
go build ./...
```

Fix any compilation errors.

- [ ] **Step 2: Verify frontend compilation**

```bash
cd /home/magomed/projects/sms/portal-frontend
npm run build
```

Fix any TypeScript errors.

- [ ] **Step 3: Run existing tests**

```bash
cd /home/magomed/projects/sms
go test ./...
```

- [ ] **Step 4: Apply migrations (if deploying)**

```bash
# On server or locally:
# migrate -path migrations -database "postgres://..." up
```

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve compilation issues from broadcast campaigns implementation"
```

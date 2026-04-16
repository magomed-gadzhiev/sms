-- Pre-aggregated hourly statistics for network partners
CREATE TABLE IF NOT EXISTS network_stats_hourly (
  partner_id    BIGINT      NOT NULL,
  hour          TIMESTAMPTZ NOT NULL,
  provider_id   BIGINT      NOT NULL DEFAULT 0,
  operator      VARCHAR(50) NOT NULL DEFAULT '',
  country       VARCHAR(3)  NOT NULL DEFAULT '',
  channel       VARCHAR(50) NOT NULL DEFAULT '',
  login         VARCHAR(100) NOT NULL DEFAULT '',
  sender_name   VARCHAR(50) NOT NULL DEFAULT '',
  traffic_type  VARCHAR(20) NOT NULL DEFAULT '',
  method        VARCHAR(20) NOT NULL DEFAULT '',
  total         INT         NOT NULL DEFAULT 0,
  sent          INT         NOT NULL DEFAULT 0,
  delivered     INT         NOT NULL DEFAULT 0,
  failed        INT         NOT NULL DEFAULT 0,
  pending       INT         NOT NULL DEFAULT 0,
  timeout       INT         NOT NULL DEFAULT 0,
  error         INT         NOT NULL DEFAULT 0,
  revenue       NUMERIC(12,4) NOT NULL DEFAULT 0,
  cost          NUMERIC(12,4) NOT NULL DEFAULT 0,
  dlr_latency_sum  BIGINT   NOT NULL DEFAULT 0,
  dlr_latency_cnt  INT      NOT NULL DEFAULT 0,
  dlr_latency_p50  INT      NOT NULL DEFAULT 0,
  dlr_latency_p95  INT      NOT NULL DEFAULT 0,
  throughput_max   DOUBLE PRECISION NOT NULL DEFAULT 0,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (partner_id, hour, provider_id, operator, country, channel, login, sender_name, traffic_type, method)
) PARTITION BY RANGE (hour);

CREATE INDEX IF NOT EXISTS idx_nsh_partner_hour ON network_stats_hourly (partner_id, hour);

-- Default partition for data that doesn't match monthly partitions
CREATE TABLE IF NOT EXISTS network_stats_hourly_default PARTITION OF network_stats_hourly DEFAULT;

-- Real-time monitoring snapshots (5-minute buckets from Redis)
CREATE TABLE IF NOT EXISTS network_monitoring_snapshot (
  partner_id      BIGINT      NOT NULL,
  provider_id     BIGINT      NOT NULL,
  ts              TIMESTAMPTZ NOT NULL,
  throughput      DOUBLE PRECISION NOT NULL DEFAULT 0,
  queue_depth     INT         NOT NULL DEFAULT 0,
  active_conns    INT         NOT NULL DEFAULT 0,
  error_count     INT         NOT NULL DEFAULT 0,
  timeout_count   INT         NOT NULL DEFAULT 0,
  pending_count   INT         NOT NULL DEFAULT 0,
  dlr_latency_p50 INT         NOT NULL DEFAULT 0,
  dlr_latency_p95 INT         NOT NULL DEFAULT 0,
  health_status   VARCHAR(10) NOT NULL DEFAULT 'ok',
  PRIMARY KEY (partner_id, provider_id, ts)
);

CREATE INDEX IF NOT EXISTS idx_nms_partner_ts ON network_monitoring_snapshot (partner_id, ts);

-- Saved views for network statistics UI
CREATE TABLE IF NOT EXISTS saved_views (
  id          BIGSERIAL   PRIMARY KEY,
  partner_id  BIGINT      NOT NULL,
  user_id     BIGINT,
  name        VARCHAR(100) NOT NULL,
  is_default  BOOLEAN     NOT NULL DEFAULT false,
  mode        VARCHAR(20) NOT NULL,
  filters     JSONB       NOT NULL DEFAULT '{}',
  group_by    VARCHAR(30),
  sort_by     VARCHAR(30),
  sort_dir    VARCHAR(4)  DEFAULT 'desc',
  columns     TEXT[]      NOT NULL DEFAULT '{}',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (partner_id, user_id, name)
);

-- Export jobs for async CSV/XLSX generation
CREATE TABLE IF NOT EXISTS export_jobs (
  id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  partner_id   BIGINT      NOT NULL,
  user_id      BIGINT      NOT NULL,
  mode         VARCHAR(20) NOT NULL,
  filters      JSONB       NOT NULL,
  format       VARCHAR(10) NOT NULL,
  status       VARCHAR(20) NOT NULL DEFAULT 'pending',
  file_path    TEXT,
  row_count    INT,
  error        TEXT,
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_ej_partner_user ON export_jobs (partner_id, user_id, created_at DESC);

-- Seed role-based view templates (partner_id=0, user_id=NULL = global templates)
INSERT INTO saved_views (partner_id, user_id, name, mode, filters, columns) VALUES
  (0, NULL, 'Владелец — полный обзор', 'analytics', '{}', ARRAY['slice','total','delivered','dlr_rate','revenue','cost','profit','margin','health']),
  (0, NULL, 'Техподдержка — мониторинг', 'monitoring', '{}', ARRAY['slice','throughput','sent','delivered','pending','timeout','error','dlr_latency_p50','dlr_latency_p95','dlr_rate','top_error','health']),
  (0, NULL, 'Менеджер — клиенты', 'stats', '{}', ARRAY['slice','total','delivered','dlr_rate','revenue','profit','margin'])
ON CONFLICT DO NOTHING;

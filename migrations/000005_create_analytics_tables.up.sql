-- Таблица для хранения статистики сообщений (сырые метрики)
CREATE TABLE message_stats (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    metric_type VARCHAR(50) NOT NULL, -- message.created, message.sent, message.delivered, message.failed
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    message_id UUID,
    status VARCHAR(50) NOT NULL,
    value BIGINT NOT NULL DEFAULT 1,
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_message_stats_metric_type ON message_stats(metric_type);
CREATE INDEX idx_message_stats_timestamp ON message_stats(timestamp);
CREATE INDEX idx_message_stats_client_id ON message_stats(client_id);
CREATE INDEX idx_message_stats_provider_id ON message_stats(provider_id);
CREATE INDEX idx_message_stats_status ON message_stats(status);
CREATE INDEX idx_message_stats_created_at ON message_stats(created_at);

-- Таблица для агрегированных метрик
CREATE TABLE aggregated_metrics (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    period VARCHAR(20) NOT NULL, -- day, hour, minute
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    status VARCHAR(50) NOT NULL,
    count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    UNIQUE(period, period_start, client_id, provider_id, status)
);

CREATE INDEX idx_aggregated_metrics_period ON aggregated_metrics(period, period_start);
CREATE INDEX idx_aggregated_metrics_client_id ON aggregated_metrics(client_id);
CREATE INDEX idx_aggregated_metrics_provider_id ON aggregated_metrics(provider_id);
CREATE INDEX idx_aggregated_metrics_status ON aggregated_metrics(status);
CREATE INDEX idx_aggregated_metrics_period_range ON aggregated_metrics(period_start, period_end);

-- Таблица для хранения отчетов
CREATE TABLE reports (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    report_type VARCHAR(50) NOT NULL, -- daily, monthly, provider, client
    format VARCHAR(20) NOT NULL, -- json, csv, pdf
    client_id UUID REFERENCES clients(id) ON DELETE SET NULL,
    provider_id UUID REFERENCES providers(id) ON DELETE SET NULL,
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    data BYTEA NOT NULL,
    generated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL
);

CREATE INDEX idx_reports_report_type ON reports(report_type);
CREATE INDEX idx_reports_client_id ON reports(client_id);
CREATE INDEX idx_reports_provider_id ON reports(provider_id);
CREATE INDEX idx_reports_period ON reports(period_start, period_end);
CREATE INDEX idx_reports_created_at ON reports(created_at);

-- Функция для автоматического обновления updated_at в aggregated_metrics
CREATE TRIGGER update_aggregated_metrics_updated_at BEFORE UPDATE ON aggregated_metrics
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

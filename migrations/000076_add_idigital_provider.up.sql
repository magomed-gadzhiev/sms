-- Добавляем провайдер I-Digital (общий, максимальный приоритет)
INSERT INTO providers (
    id, name, host, port, system_id, password,
    system_type, bind_type,
    bind_ton, bind_npi, addr_ton, addr_npi,
    address_range, max_connections,
    active, priority, throughput_per_second
) VALUES (
    uuid_generate_v4(),
    'Provider-I-Digital',
    'smpp.i-dgtl.ru', 2775, 'ip_sharip_smpp', 'en9Fahwi',
    'CMT', 'transceiver',
    0, 0, 0, 0,
    '', 5,
    true, 200, 5000
);

-- Создаём маршрут: все РФ номера через I-Digital (максимальный приоритет 200)
INSERT INTO routes (
    id, name, pattern, pattern_type,
    provider_id, priority, active,
    failover_provider_id
) VALUES (
    uuid_generate_v4(),
    'I-Digital-Russia',
    '7', 'prefix',
    (SELECT id FROM providers WHERE name = 'Provider-I-Digital'),
    200,
    true,
    (SELECT id FROM providers WHERE name = 'Provider-Backup')
);

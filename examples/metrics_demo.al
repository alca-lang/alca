// ==============================================================================
// Alca Lang: Демонстрация Prometheus/OpenMetrics метрик для мониторинга
// ==============================================================================

use std.metrics;

println("=== Alca Lang: Prometheus Metrics Demo ===");

// 1. Создаем Counter (счетчик событий)
requests := metrics.counter("api_requests_total", "Общее количество входящих API запросов");
requests.inc(10);
requests.inc(5);

// 2. Создаем Gauge (значение с плавающим уровнем)
sessions := metrics.gauge("active_user_sessions", "Количество активных пользовательских сессий");
sessions.set(128);
sessions.inc(4);
sessions.dec(2);

// 3. Создаем Histogram (распределение latency)
latency := metrics.histogram("http_request_duration_seconds", "Время обработки запроса в секундах", [0.01, 0.05, 0.1, 0.5, 1.0]);
latency.observe(0.042);
latency.observe(0.088);
latency.observe(0.320);

println("Метрики успешно собраны в приложении.");
println("\n--- Экспортированный Prometheus OpenMetrics формат ---");
println(string(metrics.export()));

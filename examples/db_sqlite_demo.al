// ==============================================================================
// Alca Lang: Демонстрация работы со встроенной базой SQLite
// ==============================================================================

use std.db;

println("=== Alca Lang: SQLite Database Demo ===");

conn := db.sqlite(":memory:");
println("Соединение с базой открыто. Ping: " + string(conn.ping()));

// 1. Создание таблицы
conn.exec("CREATE TABLE products (id INTEGER PRIMARY KEY, title TEXT, price REAL, qty INTEGER);");

// 2. Вставка данных с параметризованными запросами (защита от SQL-инъекций)
conn.exec("INSERT INTO products (title, price, qty) VALUES (?, ?, ?);", "Сервер Supermicro", 4500.00, 2);
conn.exec("INSERT INTO products (title, price, qty) VALUES (?, ?, ?);", "Коммутатор Cisco", 1200.50, 5);
conn.exec("INSERT INTO products (title, price, qty) VALUES (?, ?, ?);", "Кабель оптический", 45.00, 50);

// 3. Выборка данных
println("\nСписок оборудования из базы SQLite:");
items := conn.query("SELECT id, title, price, qty FROM products ORDER BY id ASC;");
for p in items:
    println("  [ID " + string(p["id"]) + "] " + string(p["title"]) + " | Цена: $" + string(p["price"]) + " | В наличии: " + string(p["qty"]) + " шт.");

// 4. Транзакции (атомарное списание)
println("\nВыполняем транзакционное списание склада...");
trx := conn.begin();
trx.exec("UPDATE products SET qty = qty - 1 WHERE id = ?;", 1);
trx.commit();

row := conn.query_row("SELECT qty FROM products WHERE id = 1;");
println("Остаток серверов после транзакции: " + string(row["qty"]) + " шт.");

conn.close();
println("\nСоединение с базой успешно закрыто.");

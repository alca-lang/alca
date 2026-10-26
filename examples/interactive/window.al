// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// ==========================================
// Interactive Native Desktop Window in Alca
// Interactive example: opens a native window and blocks until it is closed.
// ==========================================

use std.gui;

println("Создаю нативное окно...");

// 1. Создаем окно
win := gui.window("Alca Control Panel", 550, 420);

// 2. Добавляем элементы
win.add_label("🚀 Alca Language Desktop GUI (Windows & Linux)");
win.add_input("Поиск узлов и процессов...");
win.add_checkbox("Включить автоматический бэкап", true);
win.add_progress(75);
win.add_button("Запустить диагностику");

println("Окно отображено на экране. Закройте его для завершения скрипта.");

// 3. Отображаем окно на экране
win.show();

println("Окно закрыто. Завершение работы.");

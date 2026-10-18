// ==========================================
// Interactive Native Desktop Window in Alca
// ==========================================

use std.gui

print("Создаю нативное окно...")

// 1. Создаем окно
win = gui.window("Alca Control Panel", 550, 420)

// 2. Добавляем элементы
win.add_label("🚀 Alca Language Desktop GUI (Windows & Linux)")
win.add_input("Поиск узлов и процессов...")
win.add_checkbox("Включить автоматический бэкап", true)
win.add_progress(75)
win.add_button("Запустить диагностику")

print("Окно отображено на экране. Закройте его для завершения скрипта.")

// 3. Отображаем окно на экране
win.show()

print("Окно закрыто. Завершение работы.")

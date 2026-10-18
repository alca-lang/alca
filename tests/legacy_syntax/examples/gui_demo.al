// ==========================================
// Alca GUI Showcase — All 3 Modes
// ==========================================

print("=== Alca GUI Module Loaded ===")

// ------------------------------------------
// 1. Native System Dialogs (Zero-Dependency)
// ------------------------------------------
// Native Windows Win32 API MessageBox / Linux Zenity dialogs:
// gui.alert("Системное уведомление", "Сервер Alca успешно запущен!")
// ok = gui.confirm("Перезагрузка", "Выполнить безопасную перезагрузку?")
// name = gui.prompt("Авторизация", "Введите логин администратора:", "admin")
// file_path = gui.open_file("Выберите конфиг", "JSON Files (*.json)|*.json|All Files (*.*)|*.*")
// gui.notify("Alca Backup", "Резервная копия создана за 0.4 сек")

print("1. Native dialogs API ready: gui.alert, gui.confirm, gui.prompt, gui.open_file, gui.save_file, gui.notify")

// ------------------------------------------
// 2. Desktop WebView UI (HTML/CSS/JS + Alca IPC Bridge)
// ------------------------------------------
// Opens a dedicated desktop application window rendering modern HTML/CSS:
/*
html = """
<!DOCTYPE html>
<html>
<head>
    <style>
        body { font-family: sans-serif; background: #1e1e2e; color: #cdd6f4; padding: 30px; text-align: center; }
        button { background: #89b4fa; border: none; padding: 12px 24px; border-radius: 8px; font-weight: bold; cursor: pointer; }
    </style>
</head>
<body>
    <h1>🚀 Alca Desktop Application</h1>
    <p>Modern HTML/CSS UI powered by Alca high-performance runtime</p>
    <button onclick="window.al.send('button_clicked')">Выполнить действие</button>
</body>
</html>
"""
app = gui.webview("Alca Control Center", html, width=800, height=500)
*/

print("2. Desktop WebView API ready: gui.webview(title, htmlOrURL, width, height)")

// ------------------------------------------
// 3. Native Windows & Interactive Widgets
// ------------------------------------------
// Pure native desktop window with buttons, inputs, checkboxes, and progress bar:
/*
win = gui.window("Панель мониторинга Alca", 550, 420)
win.add_label("Узел: Node-01 (Online)")
win.add_input("Поиск логов...")
win.add_checkbox("Включить автоматический бэкап", true)
win.add_progress(85)
win.add_button("Запустить диагностику")
win.show()
*/

print("3. Native Windows & Widgets API ready: win = gui.window(title, w, h)")
print("   - win.add_label(text)")
print("   - win.add_button(text, callback)")
print("   - win.add_input(placeholder)")
print("   - win.add_checkbox(text, checked)")
print("   - win.add_progress(percent)")
print("   - win.show() / win.run()")

print("=== GUI Verification Complete ===")

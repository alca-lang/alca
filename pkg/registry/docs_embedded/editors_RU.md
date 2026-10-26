# Поддержка редакторов и инструментарий разработки (IDE)

[English](editors_EN.md) | [Русский](editors_RU.md)

Alca предоставляет интеграцию с современными средами разработки через официальный демон **Language Server Protocol (LSP)** и расширения для **Visual Studio Code**, **Cursor** и **Zed**.

---

## 1. Расширение для Visual Studio Code и Cursor

Исходный код расширения находится в каталоге [`editors/vscode/`](../editors/vscode/).

### Возможности:
- **Подсветка синтаксиса:** Полный набор ключевых слов, классов, интерфейсов, структур, операторов конвейеров (`|>`), потокового перенаправления (`>>`, `>!`, `>>>`, `?>>>`), асинхронности и тест-блоков.
- **Интеграция с Language Server (`alca lsp`):** Автодополнение кода, проверка типов в реальном времени, всплывающие подсказки (hover) с описанием сигнатур функций и методов.
- **Встроенная fallback-диагностика:** При временной недоступности LSP расширение автоматически запускает `alca check` на текущем буфере редактора.
- **Автоматическое форматирование:** Интеграция с `alca fmt` по комбинации клавиш (`Shift+Alt+F`) или автоматически при сохранении файла (`editor.formatOnSave`).
- **Сниппеты кода:** Готовые шаблоны для функций, классов, методов, сопоставления с образцом (`match`), обработки ошибок (`?>>>`), метрик Prometheus и SQL-запросов к SQLite.

### Установка

#### Windows (PowerShell):
```powershell
Remove-Item -Recurse -Force "$env:USERPROFILE\.vscode\extensions\alca-lang" -ErrorAction SilentlyContinue
Copy-Item -Recurse -Force "editors\vscode" "$env:USERPROFILE\.vscode\extensions\alca-lang"
```

#### Linux / macOS:
```bash
cp -r editors/vscode ~/.vscode/extensions/alca-lang
```

После копирования перезагрузите окно VS Code (`Ctrl+Shift+P` → `Developer: Reload Window`).

### Настройки конфигурации

| Параметр | По умолчанию | Описание |
| --- | --- | --- |
| `alca.executablePath` | `"alca"` | Путь к бинарнику Alca CLI. |
| `alca.useLanguageServer` | `true` | Использовать живой демон LSP для диагностики и автодополнения. |
| `alca.diagnostics.onOpen` | `true` | Запускать проверку типов при открытии файла. |
| `alca.diagnostics.onSave` | `true` | Проверять файл при сохранении. |
| `alca.diagnostics.onType` | `true` | Проверять код прямо во время набора текста (с задержкой). |
| `alca.diagnostics.delay` | `600` | Задержка в миллисекундах перед запуском проверки при наборе. |

---

## 2. Расширение для редактора Zed

Находится в каталоге [`editors/zed/`](../editors/zed/).

Обеспечивает подсветку синтаксиса Alca, фирменные иконки файлов `.al`, стили каталогов и тему интерфейса для быстрого редактора Zed.

### Установка
Скопируйте каталог `editors/zed` в папку установленных расширений Zed:
- Linux: `~/.local/share/zed/extensions/installed/alca`
- macOS: `~/Library/Application Support/Zed/extensions/installed/alca`
- Windows: `%LOCALAPPDATA%\Zed\extensions\installed\alca`

---

## 3. Сервер языковых протоколов (`alca lsp`)

CLI Alca включает встроенный сервер протокола LSP:

```bash
alca lsp
```

Сервер взаимодействует через стандартный ввод/вывод (stdio) с любым LSP-совместимым редактором (Neovim, Helix, Emacs, Sublime Text, Kate и др.).

### Реализованные возможности:
- **`textDocument/didOpen` и `textDocument/didChange`:** Инкрементальный синтаксический анализ и обновление дерева AST.
- **`textDocument/publishDiagnostics`:** Немедленный вывод синтаксических ошибок и несоответствий типов со строгой точностью до строки и колонки.
- **`textDocument/completion`:** Контекстное автодополнение для встроенных типов, модулей `std.*`, импортированных идентификаторов, полей структур и методов классов.
- **`textDocument/hover`:** Вывод сигнатур типов и описаний при наведении курсора на символ.

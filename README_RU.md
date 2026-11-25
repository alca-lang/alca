# Язык программирования Pawl

[English](README_EN.md) | [Русский](README_RU.md)

**Pawl** — это высокопроизводительный статически типизированный язык системного программирования, спроектированный для разработки операционных систем на голом железе (Bare-Metal OS dev), низкоуровневых системных утилит и автономных нативных приложений.

### Ключевые возможности:
- 🚀 **Нативный AOT-компилятор**: Прямая трансляция в машинный код x86-64 без внешних линковщиков и виртуальных машин. Автономная сборка форматов **PE** (`.exe` для Windows), **ELF** (для Linux) и **Raw Binary**.
- 💻 **Разработка операционных систем (OS & Bare-Metal)**:
  - Процессорные интринсики Ring 0: `cpu.cli()`, `cpu.sti()`, `cpu.hlt()`, `cpu.pause()`, порты ввода/вывода `cpu.in8()`, `cpu.out8()`, регистры управления памятью и MMU `cpu.read_cr3()`, `cpu.write_cr3()`, `cpu.invlpg()`.
  - Прямой доступ к физической памяти (MMIO): `mem.read_u8/16/32/64` и `mem.write_u8/16/32/64` (прямой вывод в видеобуфер VGA `0xB8000`, регистры APIC, контроллер прерываний PIC).
- 💿 **Поддержка загрузчиков Multiboot 1 / GRUB / QEMU**:
  - Таргет `--target freestanding-elf` внедряет заголовок Multiboot 1 с адресом загрузки `0x100000` (1 МБ). Скомпилированное ядро запускается прямо в QEMU (`qemu-system-x86_64 -kernel kernel.elf`).
- ⚡ **Продвинутое управление памятью**:
  - Собственный Mark-Sweep Garbage Collector (точный сборщик мусора) и сверхбыстрый арена-аллокатор (Bump Allocator).
- 🧵 **Асинхронность и зелёные потоки**:
  - Планировщик легких волокон (Fibers / Green Threads) с M:N конкурентностью.

Файлы программ используют расширение `.al`.

```pawl
use std.cpu
use std.mem

// Пример минимального ядра ОС: вывод в видеобуфер VGA и сон
func kernel_main() {
    cpu.cli()
    vga := 753664 // 0xB8000
    attr := 3840  // White on black

    // Пишем "PAWL" в видеобуфер
    mem.write_u16(vga + 0, attr + 80) // 'P'
    mem.write_u16(vga + 2, attr + 65) // 'A'
    mem.write_u16(vga + 4, attr + 87) // 'W'
    mem.write_u16(vga + 6, attr + 76) // 'L'

    loop {
        cpu.hlt()
    }
}
```

## Быстрый старт и сборка

```powershell
# Сборка нативного компилятора:
.\compiler\build.ps1

# Сборка приложения под Windows/Linux:
.\compiler\build_bin\pawl.exe build -o my_app.exe my_app.al

# Сборка ядра ОС под QEMU/GRUB:
.\compiler\build_bin\pawl.exe build --target freestanding-elf -o kernel.elf kernel.al
```

---

## Документация и руководства

- 📘 [**Полное методическое руководство (Handbook)**](docs/handbook_RU.md) ([English](docs/handbook_EN.md)) — исчерпывающая методичка по синтаксису, типам, логике языка, песочнице, встроенным функциям и всем библиотекам `std.*`.
- 📖 [**Спецификация языка (Language Reference)**](docs/language_RU.md) ([English](docs/language_EN.md)) — полный формальный справочник грамматики, типов, классов и операторов.
- 📚 [**Каталог стандартной библиотеки (Stdlib)**](docs/stdlib_RU.md) ([English](docs/stdlib_EN.md)) — детальный справочник функций и сигнатур более 40 модулей `stdlib`.
- 📦 [**Пакетный менеджер (APM)**](https://github.com/pawl-lang/apm) — отдельный репозиторий пакетного менеджера Pawl (`pawl.toml`, `pawl.lock`, реестр, CLI).
- 💻 [**Поддержка редакторов и LSP**](docs/editors_RU.md) ([English](docs/editors_EN.md)) — расширения для VS Code, Cursor и Zed, Language Server Protocol (`pawl lsp`).
- 🛠️ [**Руководство по CLI и песочнице**](docs/cli_RU.md) ([English](docs/cli_EN.md)) — команды `pawl run`, `build`, `dll`, `fmt`, `lsp`, флаги безопасности и песочницы.
- 🏗️ [**Архитектура и уровни исполнения**](docs/architecture_RU.md) ([English](docs/architecture_EN.md)) — устройство Bytecode VM, JIT компилятора, PE/DLL генератора и модели безопасности.
- 🌐 [**Кросс-компиляция**](docs/cross-compilation_RU.md) ([English](docs/cross-compilation_EN.md)) — сборка автономных бинарных файлов под целевые операционные системы.
- 🧪 [**Набор прикладных тестов**](tests/) — 10 тестов реального использования языка (JSON, CSV, HTTP, Crypto, IO, Time, OOP).

---

## Ключевые правила языка

- **Синтаксис**: Инструкции заканчиваются `;`. Блоки могут быть заключены в канонические фигурные скобки `{ ... }` либо оформлены через `:` и отступы.
- **Функции**: Канонический синтаксис — `fn name(params) -> ReturnType { ... }` или `pub fn`.
- **Типы**: Строгая статическая типизация с двунаправленным выводом (`name: Type = val;` или `name := val;`). Неявные приведения типов запрещены.
- **Равенство**: `==` и `!=` выполняют структурное сравнение значений по содержимому. `===` и `!==` выполняют строгую проверку идентичности указателей.
- **Диапазоны**: Полуоткрытый диапазон `a..b` (`[a, b)`) проходит от `a` до `b - 1`. Включительный диапазон `a..=b` (`[a, b]`) проходит от `a` до `b`.
- **Абстракции**:
  - `enum`: Алгебраические типы данных с сопоставлением через `match`.
  - `interface` / `trait`: Полиморфные контракты, проверяемые статически во время компиляции.
  - Конкурентность: Фоновые задачи (`@ba`) и пулы потоков (`@threads`). Полный паритет `async`/`await` между обоими движками.
- **Классы и структуры**: Поля объявляются как `name: Type [= default];` или `pub name: Type [= default];`. Одиночное наследование `class Child extends Parent`.
- **Условия**: Условия обязаны строго возвращать `bool` (концепции truthy/falsy отсутствуют).
- **Конвейеры и потоки**:
  - Функциональный конвейер: `x |> f(...)` передает `x` в качестве первого аргумента в `f`.
  - Потоковые конвейеры: `>>` (дозапись), `>!` (перезапись), `>>>` (поток в переменную), `<<` (поток из файла/консоли), `&>>` (асинхронный стрим), `?>>` / `?>>>` (логирование и перехват ошибок).
- **Динамический шлюз**: `json` — единственный динамический тип, требующий явного приведения перед использованием в типизированном коде.

---

## Песочница на основе прав (Capability-Based Sandbox)

По умолчанию Pawl запрещает доступ к файловой системе, сети, процессам, переменным окружения и нативным библиотекам:

- `--allow-read[=<path>]`: Разрешает чтение файлов или каталогов.
- `--allow-write[=<path>]`: Разрешает запись файлов или каталогов.
- `--allow-net[=<host[:port]>]`: Разрешает сетевой доступ к сокетам и HTTP.
- `--allow-run[=<cmd>]`: Разрешает запуск подпроцессов (`exec.Command` / `std.process`).
- `--allow-env[=<var>]`: Разрешает доступ к переменным окружения (`std.env`).
- `--allow-ffi[=<lib>]`: Разрешает загрузку нативных библиотек (.dll, .so).
- `--allow-all`: Безусловно предоставляет все права.

---

## Использование CLI

```text
pawl run <file.al> [args...]       # проверка типов и запуск (по умолчанию VM)
pawl run --jit <file.al> [args...] # включение многоуровневого AMD64 JIT
pawl run --vm ...                  # требовать VM; отклонять неподдерживаемые конструкции
pawl run --interp ...              # запуск через древовидный интерпретатор
pawl check <file.al>               # проверка синтаксиса и строгая проверка типов (для CI)
pawl test <file.al>                # запуск тестовых блоков и сводка PASS/FAIL
pawl fmt [-w] <file.al>            # форматирование кода (-w перезаписывает файл)
pawl repl                          # интерактивная REPL-сессия с сохранением состояния
pawl lsp                           # запуск Language Server Protocol через stdio
pawl build <file.al> [-o out]      # сборка автономного EXE для хоста (PE/COFF)
pawl build <file.al> --shared [-o lib.dll] # компиляция в нативную библиотеку (.dll / .so / .dylib)
pawl targets                       # список поддерживаемых платформ и установленных рантаймов
```

---

## Пакетный менеджер (APM)

APM вынесен в отдельный репозиторий: [**github.com/pawl-lang/apm**](https://github.com/pawl-lang/apm).

`apm` устанавливает библиотеки для использования через `use` без ручной настройки путей:

```bash
# Сборка и запуск APM
git clone https://github.com/pawl-lang/apm.git
cd apm && go build -o apm ./cmd/apm

apm install "json@^1.2.0"       # из реестра пакетов
apm show                        # список установленных библиотек
```

---

## Разработка и тестирование

```bash
gofmt -l .
go vet ./...
go test ./...
go test -race ./...
```

Запуск всех 10 тестов:
```bash
pawl run --allow-all tests/test_01_strings_text.al
pawl run --allow-all tests/test_02_collections_algorithms.al
pawl run --allow-all tests/test_03_json_data.al
pawl run --allow-all tests/test_04_csv_ini_formats.al
pawl run --allow-all tests/test_05_math_financial.al
pawl run --allow-all tests/test_06_oop_ecommerce.al
pawl run --allow-all tests/test_07_error_handling_resilience.al
pawl run --allow-all tests/test_08_file_io_logging.al
pawl run --allow-all tests/test_09_crypto_security.al
pawl run --allow-all tests/test_10_datetime_bench.al
```

---

## Лицензия

Pawl распространяется под лицензией [Apache License, Version 2.0](LICENSE).  
См. [NOTICE](NOTICE) для информации об авторских правах.

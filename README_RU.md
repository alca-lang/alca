# Язык программирования Alca

[English](README_EN.md) | [Русский](README_RU.md)

**Alca** — это современный, статически типизированный язык программирования, спроектированный для системной автоматизации, кроссплатформенных утилит и нативной разработки. Он сочетает эргономику скриптовых языков со статической безопасностью типов, выполнением на виртуальной машине (Bytecode VM) с JIT-ускорением и автономной сборкой без внешних тулчейнов:
- **Windows**: Встроенный генератор исполняемых файлов PE/EXE и нативных библиотек DLL (не требуются MSVC, MinGW или Clang).
- **Windows и Linux**: Быстрая Bytecode VM с многоуровневым JIT-компилятором и гранулярной песочницей на основе прав (Capabilities).

Файлы программ используют расширение `.al`.

```alca
use std.math;

struct Server {
    ip: string;
    port: int = 80;
}

pub fn area(r: float) -> float {
    return math.PI * r * r;
}

srv := Server("127.0.0.1", 8080);
println("listening on " + srv.ip + ":" + string(srv.port));
println(area(2.0));
```

## Быстрый старт

```bash
go build -o alca ./cmd/alca
./alca run examples/hello.al
```

Основным движком выполнения является Bytecode VM. Программы можно напрямую компилировать в автономные исполняемые файлы Windows (`alca build app.al`) и нативные DLL (`alca dll lib.al`).

---

## Документация и руководства

- 📘 [**Полное методическое руководство (Handbook)**](docs/handbook_RU.md) ([English](docs/handbook_EN.md)) — исчерпывающая методичка по синтаксису, типам, логике языка, песочнице, встроенным функциям и всем библиотекам `std.*`.
- 📖 [**Спецификация языка (Language Reference)**](docs/language_RU.md) ([English](docs/language_EN.md)) — полный формальный справочник грамматики, типов, классов и операторов.
- 📚 [**Каталог стандартной библиотеки (Stdlib)**](docs/stdlib_RU.md) ([English](docs/stdlib_EN.md)) — детальный справочник функций и сигнатур более 40 модулей `stdlib`.
- 📦 [**Пакетный менеджер (APM)**](https://github.com/alca-lang/apm) — отдельный репозиторий пакетного менеджера Alca (`alca.toml`, `alca.lock`, реестр, CLI).
- 💻 [**Поддержка редакторов и LSP**](docs/editors_RU.md) ([English](docs/editors_EN.md)) — расширения для VS Code, Cursor и Zed, Language Server Protocol (`alca lsp`).
- 🛠️ [**Руководство по CLI и песочнице**](docs/cli_RU.md) ([English](docs/cli_EN.md)) — команды `alca run`, `build`, `dll`, `fmt`, `lsp`, флаги безопасности и песочницы.
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

По умолчанию Alca запрещает доступ к файловой системе, сети, процессам, переменным окружения и нативным библиотекам:

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
alca run <file.al> [args...]       # проверка типов и запуск (по умолчанию VM)
alca run --jit <file.al> [args...] # включение многоуровневого AMD64 JIT
alca run --vm ...                  # требовать VM; отклонять неподдерживаемые конструкции
alca run --interp ...              # запуск через древовидный интерпретатор
alca check <file.al>               # проверка синтаксиса и строгая проверка типов (для CI)
alca test <file.al>                # запуск тестовых блоков и сводка PASS/FAIL
alca fmt [-w] <file.al>            # форматирование кода (-w перезаписывает файл)
alca repl                          # интерактивная REPL-сессия с сохранением состояния
alca lsp                           # запуск Language Server Protocol через stdio
alca build <file.al> [-o out]      # сборка автономного EXE для хоста (PE/COFF)
alca dll <file.al> [-o out.dll]    # компиляция в нативную DLL / Linux SO
alca targets                       # список поддерживаемых платформ и установленных рантаймов
```

---

## Пакетный менеджер (APM)

APM вынесен в отдельный репозиторий: [**github.com/alca-lang/apm**](https://github.com/alca-lang/apm).

`apm` устанавливает библиотеки для использования через `use` без ручной настройки путей:

```bash
# Сборка и запуск APM
git clone https://github.com/alca-lang/apm.git
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
alca run --allow-all tests/test_01_strings_text.al
alca run --allow-all tests/test_02_collections_algorithms.al
alca run --allow-all tests/test_03_json_data.al
alca run --allow-all tests/test_04_csv_ini_formats.al
alca run --allow-all tests/test_05_math_financial.al
alca run --allow-all tests/test_06_oop_ecommerce.al
alca run --allow-all tests/test_07_error_handling_resilience.al
alca run --allow-all tests/test_08_file_io_logging.al
alca run --allow-all tests/test_09_crypto_security.al
alca run --allow-all tests/test_10_datetime_bench.al
```

---

## Лицензия

Alca распространяется под лицензией [Apache License, Version 2.0](LICENSE).  
См. [NOTICE](NOTICE) для информации об авторских правах.

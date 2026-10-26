# Системный API для Alca: Архитектурное предложение

## 1. Краткое резюме

Данный документ описывает архитектуру системного API для языка программирования Alca, который закрывает задачи, аналогичные Python-модулям `platform`, `ctypes`, `psutil`, `win32api` и смежным API. Архитектура базируется на изучении существующей кодовой базы Alca и предлагает модульную, платформенно-ориентированную структуру с четким разделением переносимого кода и платформенных реализаций.

**Рекомендуемый подход**: Расширение существующих модулей (`sys`, `ffi`, `process`) с добавлением нового модуля `platform` для идентификации системы и платформенных API через отдельные файлы для каждой ОС (`_windows.go`, `_unix.go`, `_darwin.go`).

---

## 2. Карта существующих возможностей Alca и пробелов

### 2.1 Текущие модули stdlib

| Модуль | Текущие возможности | Пробелы |
|--------|-------------------|---------|
| **sys** | `disk_usage`, `ram_usage`, `cpu_count`, `arch`, `platform`, `network_interfaces` | CPU utilization, process enumeration, uptime, system info, battery, sensors |
| **ffi** | Динамическая загрузка библиотек (`load`), вызов функций (`call`, `func`), указатели | Типизированные оболочки, соглашения вызова, управление памятью, ABI |
| **process** | `exec`, `exec_background`, `kill_process`, `process_exists`, `get_pid`, `get_ppid` | Process tree, environment variables listing, open files, network connections |
| **env** | `get`, `set`, `unset`, `list` | Переменные окружения процесса, путь, домашняя директория |
| **net** | TCP/UDP сокеты, DNS lookup | Network statistics, active connections, interface metrics |
| **mem** | Аллокация, освобождение, копирование памяти | Process memory maps, shared memory |
| **security** | Проверка прав, песочница | UAC elevation, permission requests, audit logging |
| **gui** | Windows GUI, WebView | Системные диалоги, notification API |

### 2.2 Отсутствующие категории (по сравнению с Python экосистемой)

1. **Платформенная идентификация**: Детальная информация о системе, версии, дистрибутиве
2. **Управление процессами**: Список процессов, дерево процессов, CPU/memory usage по процессу
3. **Аппаратная инвентаризация**: CPU, RAM, GPU, диски, устройства, драйверы
4. **Питание и ACPI**: Батарея, питание, спящий режим, температура
5. **Системные API Windows**: win32api (registry, services, WMI)
6. **Устройства и драйверы**: Перечисление, свойства, управление
7. **Сетевая статистика**: Подробная информация о соединениях, трафике
8. **Мониторинг**: Системные события, performance counters

---

## 3. Предлагаемая структура модулей и слоёв реализации

### 3.1 Общая архитектура

```
pkg/stdlib/
├── platform.go                 # Переносимый интерфейс platform
├── platform_windows.go         # Windows-специфичная реализация
├── platform_unix.go            # Unix (Linux) реализация
├── platform_darwin.go          # macOS реализация
├── sys.go                      # Расширение sys с system info
├── sys_windows.go              # Windows system monitoring
├── sys_unix.go                 # Unix system monitoring
├── sys_darwin.go               # macOS system monitoring
├── ffi.go                      # Расширение ffi с типами
├── ffi_windows.go              # Windows FFI enhancements
├── ffi_unix.go                 # Unix FFI enhancements
├── process.go                  # Расширение process
├── process_windows.go          # Windows process API
├── process_unix.go             # Unix process API
├── devices.go                  # Новый модуль устройств
├── devices_windows.go          # Windows device API
├── devices_unix.go             # Unix device API
└── power.go                    # Новый модуль питания
    ├── power_windows.go
    ├── power_unix.go
    └── power_darwin.go
```

### 3.2 Модуль `platform` (новый)

**Назначение**: Идентификация ОС, архитектуры, версии и доступных возможностей системы.

```go
// pkg/stdlib/platform.go
package stdlib

// PlatformInfo содержит переносимую информацию о системе
type PlatformInfo struct {
    System      string  // "windows", "linux", "darwin"
    Release     string  // "10.0.22621", "22.04", "13.5.2"
    Version     string  // "Windows 11", "Ubuntu 22.04 LTS", "macOS Ventura"
    Machine     string  // "x86_64", "arm64"
    Processor   string  // "Intel Core i7", "Apple M2"
    NodeName    string  // hostname
    PythonLike  bool    // совместимость с Python platform
}

// PlatformCapabilities описывает доступные системные возможности
type PlatformCapabilities struct {
    SupportsFFI      bool
    SupportsGUI      bool
    SupportsWMI      bool // Windows Management Instrumentation
    SupportsACPI     bool
    SupportsSensors  bool
    SupportsDrivers  bool
    RequiresRoot     bool // некоторые операции требуют root/admin
}
```

**API на Alca**:

```alca
// Идентификация системы
platform.system()          // -> "windows"
platform.release()          // -> "10.0.22621"
platform.version()          // -> "Windows 11 Pro"
platform.machine()         // -> "amd64"
platform.processor()       // -> "Intel(R) Core(TM) i7-10700K"
platform.node()            // -> "DESKTOP-ABC123"
platform.architecture()    // -> map {bits: 64, linkage: "PE", executable: "Windows-PE"}

// Единая информация
platform.info()            // -> map {system, release, version, machine, processor, node}
platform.uname()           // -> совместимость с os.uname() Python

// Возможности платформы
platform.capabilities()    // -> map {supports_ffi, supports_gui, supports_wmi, ...}
platform.has_feature("wmi")  // -> bool

// Платформенно-специфичные данные
platform.win32_ver()       // Windows: -> (major, minor, build, platform_id, text)
platform.linux_distro()    // Linux: -> map {id, name, version, codename}
platform.mac_ver()         // macOS: -> map {version, build, product, build_number}
```

### 3.3 Расширение модуля `sys` (system)

**Назначение**: Мониторинг системных ресурсов: CPU, память, диски, сеть, процессы, uptime.

```alca
// CPU
sys.cpu_percent(interval?)          // -> float (общая загрузка CPU)
sys.cpu_percent_per_cpu(interval?)  // -> list[float] (по ядрам)
sys.cpu_times()                     // -> map {user, system, idle, iowait}
sys.cpu_count(logical?)              // -> int (логические/физические ядра)
sys.cpu_freq()                      // -> map {current, min, max} (MHz)

// Память
sys.virtual_memory()                // -> map {total, available, percent, used, free, active, inactive}
sys.swap_memory()                    // -> map {total, used, free, percent, sin, sout}

// Диски
sys.disk_partitions(all?)           // -> list[map {device, mountpoint, fstype, opts}]
sys.disk_usage(path)                // -> map {total, used, free, percent}
sys.disk_io_counters()              // -> map {read_count, write_count, read_bytes, write_bytes}

// Сеть
sys.net_io_counters()               // -> map {bytes_sent, bytes_recv, packets_sent, packets_recv}
sys.net_connections(kind?)          // -> list[map {fd, family, type, laddr, raddr, status}]
sys.net_if_addrs()                  // -> list[map {name, family, address, netmask, broadcast}]
sys.net_if_stats()                  // -> map {name, bytes_sent, bytes_recv, dropin, dropout, errin, errout}

// Процессы
sys.pids()                          // -> list[int] (все PID)
sys.process_info(pid)                // -> map {pid, name, exe, cmdline, ppid, status, ...}
sys.process_iter(attrs?)             // -> iterator по процессам
sys.boot_time()                     // -> float (timestamp запуска системы)
sys.users()                         // -> list[map {name, terminal, host, started}]

// Системное время
sys.uptime()                        // -> float (секунды uptime)
```

### 3.4 Расширение модуля `ffi`

**Назначение**: Типизированный FFI с поддержкой различных соглашений вызова и ABI.

```alca
// Типы данных
ffi.c_int      // тип для C int
ffi.c_float    // тип для C float
ffi.c_double   // тип для C double
ffi.c_string   // тип для C char*
ffi.c_pointer  // тип для void*

// Загрузка с типами
ffi.load_with_types("user32.dll", {
    "MessageBoxW": ["int", ["pointer", "pointer", "pointer", "int"]]
})

// Соглашения вызова
ffi.set_calling_convention("stdcall")  // "cdecl", "stdcall", "fastcall"
ffi.set_abi("msvc")                   // "msvc", "gcc", "clang"

// Управление памятью
ffi.alloc(size)                       // -> pointer
ffi.free(ptr)                         // -> bool
ffi.copy_to_ptr(ptr, data)            // -> bool
ffi.read_from_ptr(ptr, size)          // -> blob

// Структуры
ffi.define_struct("POINT", {
    x: "long",
    y: "long"
})
ffi.struct_ptr(ptr, "POINT")          // -> map {x, y}
```

### 3.5 Платформенные API (новый модуль `win32` для Windows)

**Назначение**: Доступ к Windows-specific API (registry, services, WMI).

```alca
// Registry
win32.reg_open_key(hive, path)        // -> handle
win32.reg_read_value(handle, name)    // -> value
win32.reg_write_value(handle, name, value, type)
win32.reg_enum_keys(handle)           // -> list[string]
win32.reg_close_key(handle)

// Services
win32.service_list()                  // -> list[map {name, display_name, status}]
win32.service_start(name, args?)     // -> bool
win32.service_stop(name)              // -> bool
win32.service_info(name)              // -> map {state, pid, start_type}

// WMI
win32.wmi_query(query)                // -> list[map] (SQL-like запросы к WMI)
win32.wmi_get_class(class)            // -> list[map]

// Windows-specific info
win32.version_info()                  // -> map {major, minor, build, product_type}
win32.edition()                       // -> string ("Pro", "Home", "Enterprise")
win32.license_status()                // -> map {is_activated, product_key}
```

### 3.6 Модуль `devices` (новый)

**Назначение**: Перечисление устройств, запрос свойств, взаимодействие с драйверами.

```alca
// Перечисление устройств
devices.list(type?)                   // -> list[map {id, name, class, driver}]
devices.get(id)                       // -> map {vendor, product, serial, ...}

// Платформенно-специфичные устройства
devices.windows.pnp_devices()         // -> list[map] (Plug and Play)
devices.windows.drivers()             // -> list[map] (загруженные драйверы)
devices.linux.pci_devices()           // -> list[map] (lspci)
devices.linux.usb_devices()           // -> list[map] (lsusb)
devices.macos.io_registry()           // -> list[map] (IOKit)

// Мониторинг
devices.watch_events(callback)        // -> watcher (подключение/отключение устройств)
```

### 3.7 Модуль `power` (новый)

**Назначение**: Информация о питании, батарее, ACPI, спящих режимах.

```alca
// Батарея
power.battery_status()               // -> map {percent, charging, time_remaining}
power.battery_info()                 // -> map {capacity, cycles, health, technology}

// Питание
power.power_plugged()                // -> bool
power.power_source()                 // -> string ("battery", "ac", "usb")

// Управление питанием
power.suspend()                      // -> bool (suspend to RAM)
power.hibernate()                    // -> bool (suspend to disk)
power.reboot()                       // -> bool
power.shutdown()                     // -> bool

// Температура и сенсоры (там, где доступно)
power.sensors()                      // -> list[map {name, type, value, unit}]
power.cpu_temp()                     // -> list[float] (по ядрам)
```

---

## 4. Примеры публичного API на Alca

### 4.1 Платформенная идентификация

```alca
import platform, sys

// Проверка ОС
if platform.system() == "windows":
    print("Running on Windows")
    info = platform.win32_ver()
    print(f"Version: {info[3]}")
elif platform.system() == "linux":
    distro = platform.linux_distro()
    print(f"Distribution: {distro['name']} {distro['version']}")

// Проверка возможностей
caps = platform.capabilities()
if caps['supports_wmi']:
    import win32
    processes = win32.wmi_query("SELECT * FROM Win32_Process")
```

### 4.2 Мониторинг системы

```alca
import sys

// CPU utilization
cpu = sys.cpu_percent(interval=1.0)
print(f"CPU usage: {cpu}%")

per_cpu = sys.cpu_percent_per_cpu(interval=1.0)
print(f"Per-core: {per_cpu}")

// Memory
mem = sys.virtual_memory()
print(f"Memory: {mem['percent']}% used ({mem['used'] / 1024**3:.2f} GB)")

// Disk
disks = sys.disk_partitions(all=true)
for disk in disks:
    usage = sys.disk_usage(disk['mountpoint'])
    print(f"{disk['mountpoint']}: {usage['percent']}% used")

// Processes
for pid in sys.pids():
    p = sys.process_info(pid)
    if p['name'] == "chrome.exe":
        print(f"Chrome PID {pid}: CPU {p['cpu_percent']}%")
```

### 4.3 FFI с типами

```alca
import ffi

// Загрузка с описанием типов
user32 = ffi.load_with_types("user32.dll", {
    "MessageBoxW": ["int", ["pointer", "pointer", "pointer", "int"]]
})

// Установка соглашения вызова
ffi.set_calling_convention("stdcall")

// Вызов функции
msg = "Hello from Alca!"
result = user32.call("MessageBoxW", 0, msg, "Title", 0)
```

### 4.4 Windows API

```alca
import win32

// Registry
hkey = win32.reg_open_key("HKEY_LOCAL_MACHINE", "SOFTWARE\\Microsoft\\Windows")
value = win32.reg_read_value(hkey, "CurrentVersion")
win32.reg_close_key(hkey)

// Services
services = win32.service_list()
for svc in services:
    if svc['name'] == "wuauserv":
        print(f"Windows Update: {svc['status']}")

// WMI
processes = win32.wmi_query("SELECT Name, ProcessId FROM Win32_Process")
for proc in processes:
    print(f"{proc['Name']} (PID: {proc['ProcessId']})")
```

### 4.5 Устройства

```alca
import devices

// Список всех устройств
all_devices = devices.list()
for dev in all_devices:
    print(f"{dev['name']} ({dev['class']})")

// Платформенно-специфично
if platform.system() == "windows":
    pci = devices.windows.pnp_devices()
elif platform.system() == "linux":
    pci = devices.linux.pci_devices()
```

### 4.6 Питание

```alca
import power

// Батарея
battery = power.battery_status()
if battery['charging']:
    print(f"Charging: {battery['percent']}%")
else:
    print(f"On battery: {battery['percent']}% ({battery['time_remaining']} min)")

// Сенсоры
sensors = power.sensors()
for sensor in sensors:
    print(f"{sensor['name']}: {sensor['value']} {sensor['unit']}")
```

---

## 5. Таблица поддержки по ОС и зависимостей

| Функционал | Windows | Linux | macOS | Зависимости | Требуемые права |
|------------|---------|-------|-------|-------------|----------------|
| **platform.system()** | ✅ | ✅ | ✅ | runtime.GOOS | Нет |
| **platform.release()** | ✅ | ✅ | ✅ | Windows API, /etc/os-release, sw_vers | Нет |
| **platform.win32_ver()** | ✅ | ❌ | ❌ | kernel32.dll | Нет |
| **platform.linux_distro()** | ❌ | ✅ | ❌ | /etc/os-release | Нет |
| **sys.cpu_percent()** | ✅ | ✅ | ✅ | GetSystemTimes, /proc/stat, host_statistics | Нет |
| **sys.virtual_memory()** | ✅ | ✅ | ✅ | GlobalMemoryStatusEx, /proc/meminfo, host_statistics | Нет |
| **sys.pids()** | ✅ | ✅ | ✅ | CreateToolhelp32Snapshot, /proc, proc_listbsd | Нет |
| **sys.process_info()** | ✅ | ⚠️ | ⚠️ | Toolhelp32, /proc/[pid]/, proc_bsdinfo | Нет |
| **sys.net_connections()** | ✅ | ⚠️ | ⚠️ | GetExtendedTcpTable, /proc/net/tcp, netstat | ⚠️ Linux/macOS |
| **win32.reg_*()** | ✅ | ❌ | ❌ | advapi32.dll | ⚠️ Admin |
| **win32.service_*()** | ✅ | ❌ | ❌ | advapi32.dll | ⚠️ Admin |
| **win32.wmi_*()** | ✅ | ❌ | ❌ | wbemdisp.dll / COM | ⚠️ Admin |
| **devices.list()** | ⚠️ | ⚠️ | ⚠️ | SetupAPI, sysfs, IOKit | ⚠️ Admin |
| **devices.windows.pnp_devices()** | ✅ | ❌ | ❌ | SetupAPI | ⚠️ Admin |
| **devices.linux.pci_devices()** | ❌ | ✅ | ❌ | /sys/bus/pci | Нет |
| **power.battery_status()** | ✅ | ✅ | ✅ | ACPI, /sys/class/power_supply, IOKit | Нет |
| **power.suspend()** | ✅ | ⚠️ | ✅ | powercfg, systemd, IOKit | ⚠️ Linux |
| **power.shutdown()** | ✅ | ⚠️ | ✅ | ExitWindowsEx, systemctl, IOKit | ⚠️ Admin/root |
| **power.sensors()** | ⚠️ | ⚠️ | ⚠️ | WMI, lm-sensors, IOKit | ⚠️ Linux |

**Примечания**:
- ✅ Полная поддержка
- ⚠️ Частичная поддержка (требует права или дополнительные компоненты)
- ❌ Не поддерживается на этой платформе

---

## 6. Ошибки, версии API и обратная совместимость

### 6.1 Обработка ошибок

```alca
// Типы ошибок
// - PermissionError (403): недостаточно прав
// - NotImplementedError (501): функция не реализована на этой платформе
// - PlatformError (500): ошибка системного API
// - ValueError (400): неверные аргументы

// Пример
try:
    services = win32.service_list()
except error as e:
    if e.code == 403:
        print("Requires administrator privileges")
    elif e.code == 501:
        print("Not supported on this platform")
```

### 6.2 Версионирование API

```go
// pkg/stdlib/platform.go
const (
    PlatformAPIVersion = "1.0.0"
    PlatformMinVersion = "1.0.0"
)

// Function returns API version
func PlatformAPIVersion(args ...interpreter.Value) interpreter.Value {
    return &interpreter.StringValue{Val: PlatformAPIVersion}
}
```

### 6.3 Обратная совместимость

- Добавление новых функций: всегда обратно совместимо
- Изменение возвращаемых значений: новая версия через суффикс `_v2`
- Удаление функций: помечать как deprecated минимум за один релиз
- Платформенные ограничения: возвращать `NotImplementedError` с описанием

---

## 7. Поэтапный план внедрения

### Этап 1: Базовый `platform` (1-2 недели)
- [ ] Создать `platform.go` с переносимыми функциями
- [ ] Реализовать `platform_windows.go` (Windows API)
- [ ] Реализовать `platform_unix.go` (/etc/os-release)
- [ ] Реализовать `platform_darwin.go` (sw_vers)
- [ ] Добавить тесты для всех платформ
- [ ] Документация и примеры

### Этап 2: Расширение `sys` (2-3 недели)
- [ ] `sys.cpu_percent*()` для всех платформ
- [ ] `sys.virtual_memory()` для всех платформ
- [ ] `sys.disk_io_counters()` (где доступно)
- [ ] `sys.net_io_counters()` для всех платформ
- [ ] `sys.pids()` и `sys.process_info()` (базовая информация)
- [ ] `sys.boot_time()` для всех платформ
- [ ] Тесты и документация

### Этап 3: Расширение `ffi` (1-2 недели)
- [ ] Добавить типы данных (c_int, c_float, и т.д.)
- [ ] Реализовать соглашения вызова (stdcall, cdecl)
- [ ] Управление памятью (alloc, free, read, write)
- [ ] Поддержка структур
- [ ] Документация

### Этап 4: Платформенные API Windows (2-3 недели)
- [ ] Создать модуль `win32.go`
- [ ] Registry API (open, read, write, enum)
- [ ] Services API (list, start, stop, info)
- [ ] WMI API (query, get_class)
- [ ] Тесты (требуют Windows)

### Этап 5: Модуль `devices` (2-3 недели)
- [ ] Создать базовый интерфейс в `devices.go`
- [ ] Windows: PnP devices, drivers
- [ ] Linux: PCI/USB devices (sysfs)
- [ ] macOS: IOKit registry
- [ ] Device watching/events
- [ ] Тесты

### Этап 6: Модуль `power` (1-2 недели)
- [ ] Батарея для всех платформ
- [ ] Сенсоры (где доступно)
- [ ] Управление питанием (suspend, hibernate, shutdown)
- [ ] Тесты

### Этап 7: Интеграция и документация (1-2 недели)
- [ ] Обновить README.md
- [ ] Создать примеры для каждого модуля
- [ ] Добавить в pkg/stdlib/register.go
- [ ] Обновить тесты безопасности (sandbox)
- [ ] Performance testing

**Итого**: 10-16 недель разработки

---

## 8. Сравнение архитектурных вариантов

### Вариант A: Расширение существующих модулей (РЕКОМЕНДУЕТСЯ)

**Описание**: Расширить `sys`, `ffi`, `process` и добавить новые модули `platform`, `devices`, `power`, `win32`.

**Преимущества**:
- Минимальные изменения в существующем коде
- Естественная группировка по функциональности
- Знакомый импорт (`import sys` вместо `import system`)
- Легче сопровождать (меньше файлов)
- Совместимость с существующим кодом

**Недостатки**:
- Модули могут стать большими
- Требует четкой внутренней организации

**Влияние на совместимость**: Обратная совместимость сохраняется полностью.

### Вариант B: Единый модуль `system`

**Описание**: Создать один большой модуль `system` с подпакетами `system.platform`, `system.devices`, и т.д.

**Преимущества**:
- Централизованный импорт (`import system`)
- Единое пространство имен

**Недостатки**:
- Большой монолитный модуль
- Требует изменения существующего кода
- Сложнее тестировать отдельно

**Влияние на совместимость**: Требует миграции существующего кода.

### Вариант C: Типизированные оболочки для всех FFI

**Описание**: Создать типизированные Go-оболочки для всех системных API вместо прямого FFI.

**Преимущества**:
- Типобезопасность
- Лучшая производительность
- Чистый Go код

**Недостатки**:
- Огромный объем работы (тысячи функций)
- Быстрое устаревание (системные API меняются)
- Дублирование с существующими библиотеками

**Влияние на совместимость**: Не влияет, но требует значительных ресурсов.

### Вариант D: Прямой FFI из Alca

**Описание**: Предоставить только базовый FFI и ожидать, что пользователи будут вызывать системные API напрямую.

**Преимущества**:
- Минимальный объем работы
- Максимальная гибкость

**Недостатки**:
- Нет переносимости
- Сложно для пользователей
- Нет типобезопасности

**Влияние на совместимость**: Совместимость сохраняется, но UX ухудшается.

**Выбор**: Вариант A (расширение существующих модулей) является оптимальным балансом между объемом работы, совместимостью и UX.

---

## 9. Открытые вопросы для автора проекта

1. **Стратегия тестирования**: Как тестировать платформенно-специфичный код? CI на всех платформах или mocks?

2. **Уровень абстракции**: На сколько низкоуровневым должен быть API? Полный доступ к Win32 или только высокоуровневые функции?

3. **Права и песочница**: Как интегрировать с существующей песочницей `pkg/sandbox`? Требовать ли явное разрешение для опасных операций?

4. **Производительность**: Насколько важна производительность для мониторинга (например, вызов WMI может быть медленным)?

5. **Внешние зависимости**: Разрешить ли использование сторонних Go-библиотек (например, `github.com/shirou/gopsutil` для cross-platform кода)?

6. **Стратегия устаревания**: Как отмечать функции как deprecated? На сколько релизов хранить compatibility layer?

7. **Документация**: Где хранить документацию? Внутри кода или отдельный docs/?

8. **Примеры и учебные материалы**: Нужны ли официальные примеры использования системного API?

9. **Безопасность**: Как защититься от использования системного API в коде, который выполняется в ненадежной среде?

10. **Сборка standalone**: Как эти модули поведут себя в standalone-сборке? Будут ли они динамически загружать системные библиотеки?

---

## 10. Измененные файлы репозитория

### Созданные файлы:

1. **pkg/stdlib/urllib2_websocket.go** - WebSocket transport
   - `websocket_connect()` - sandbox-aware TCP/TLS handshake
   - `websocket_send()` / `websocket_receive()` - masked client frames,
     fragmented messages, UTF-8 validation, and bounded payloads
   - `websocket_ping()` / `websocket_pong()` - control frames
   - `websocket_close()` - close frame and connection cleanup
   - Server handshake headers and selected subprotocol are validated

2. **pkg/stdlib/urllib2.go** - Расширение urllib2
   - Добавлены HTTP методы: `put()`, `delete()`, `patch()`, `head()`, `options()`
   - Обновлены регистрации функций
   - Маркировка старых WebSocket функций как deprecated

3. **docs/SYSTEM_API_ARCHITECTURE.md** - Данный документ

### Измененные файлы:

1. **pkg/stdlib/urllib2.go**
   - Регистрация новых HTTP методов
   - Регистрация новых WebSocket функций
   - Обновление legacy функций

---

## 11. Рекомендации по внедрению

### 11.1 Немедленные действия

1. **Принять архитектуру**: Утвердить предложенную структуру модулей
2. **Начать с platform**: Это самый независимый модуль
3. **Добавить тесты**: Создать infrastructure для тестирования на разных платформах
4. **Обновить документацию**: Добавить раздел о системном API в README

### 11.2 Долгосрочные действия

1. **Постепенное внедрение**: Реализовывать по этапам, начиная с самых переносимых функций
2. **Обратная связь**: Собирать feedback от пользователей о нужных функциях
3. **Производительность**: Профилировать системные вызовы и оптимизировать
4. **Безопасность**: Проводить security audit для всех новых системных API

### 11.3 Стратегия развития

- **Phase 1 (3 месяца)**: Базовый `platform` + расширение `sys`
- **Phase 2 (3 месяца)**: Расширение `ffi` + Windows API
- **Phase 3 (3 месяца)**: `devices` + `power`
- **Phase 4 (3 месяца)**: Оптимизация, документация, примеры

---

## Заключение

Предложенная архитектура обеспечивает:

- **Переносимость**: Четкое разделение переносимого и платформенно-специфичного кода
- **Масштабируемость**: Модульная структура позволяет добавлять новые функции
- **Безопасность**: Интеграция с существующей песочницей
- **Совместимость**: Обратная совместимость с существующим кодом
- **Честность**: Явное указание платформенных ограничений

Следующий шаг - утверждение архитектуры автором проекта и начало реализации первого этапа.

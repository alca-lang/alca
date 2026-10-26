# Спецификация и справочник языка Alca

[English](language_EN.md) | [Русский](language_RU.md)

Alca — это статически типизированный язык программирования для системной автоматизации и нативной разработки. Он сочетает лаконичный синтаксис со статическим анализом типов, быстрым исполнением на виртуальной машине Bytecode VM, JIT-ускорением и гранулярной песочницей.

---

## 1. Структура программы и синтаксис

Программа представляет собой последовательность инструкций.
- **Окончание инструкций**: Простые инструкции заканчиваются точкой с запятой (`;`).
- **Блоки кода**: Блоки могут быть заключены либо в канонические фигурные скобки (`{ ... }`), либо оформлены двоеточием (`:`) с 4-пробельным отступом.
- **Комментарии**: Однострочные начинаются с `//`, многострочные обрамляются `/* ... */`.

```alca
use std.math;

// Канонический синтаксис со скобками
fn calculate(r: float) -> float {
    if r <= 0.0 {
        return 0.0;
    }
    return math.PI * r * r;
}

// Синтаксис с двоеточием и отступами
fn greeting(name: string):
    if len(name) == 0:
        println("Привет, Гость!");
    else:
        println("Привет, " + name + "!");
```

### Импорт модулей
- `use std.math;` — импорт модуля стандартной библиотеки в пространство имен `math`.
- `use "./utils.al";` — импорт локального файла скрипта.

---

## 2. Система типов

Alca реализует **строгую статическую типизацию** с двунаправленным выводом. Неявные расширения, сужения или тихие приведения типов строго запрещены.

### Встроенные типы
- **Примитивные**: `int`, `float`, `string`, `bool`.
- **Тип отсутствия значения**: `void`.
- **Динамический шлюз**: `json` (единственный динамический тип; требует приведения перед типизированным использованием).
- **Null-безопасность**: `T | null` (значения типа `T` не могут содержать `null`, если тип не объявлен как nullable явно).
- **Коллекции**: `list[T]`, `map[K, V]`.
- **Функции**: `func`.
- **Ошибки**: `error`.
- **Пользовательские типы**: `struct`, `class`, `enum`, `interface`.

### Объявления переменных и неизменяемость
```alca
// Явное объявление
count: int = 10;
price: float = 99.95;
name: string = "Alca";
active: bool = true;

// Вывод типа (фиксируется один раз на этапе компиляции)
ratio := 1.75;              // float
items := ["apple", "pear"]; // list[string]

// Повторное объявление переменной в той же или вложенной области запрещено:
// count: int = 20;         // ОШИБКА: variable 'count' already declared
count = 20;                 // Корректно: присваивание нового значения
```

### Строгая арифметика
Числовые типы не смешиваются неявно:
```alca
a: int = 5;
b: float = 2.5;

// c := a + b;              // ОШИБКА: operator '+' cannot be applied to int and float
c := float(a) + b;          // OK: 7.5
quotient := float(a) / 2.0; // Деление с плавающей точкой: 2.5
int_div := a / 2;           // Целочисленное деление усекается к нулю: 2
```

### Null-безопасность и сужение типов
```alca
maybe_user: string | null = null;

// Прямой доступ к полям nullable-значений блокируется тайпчекером
if maybe_user != null {
    // Внутри блока тип maybe_user безопасно сужается до string
    println("Пользователь: " + maybe_user);
}
```

---

## 3. Операторы

### Арифметика и строки
- `+`, `-`, `*`, `/`, `%`, `**` (возведение в степень).
- `+` для строк выполняет конкатенацию.

### Сравнения и идентичность
- Равенство значений: `==` и `!=` (структурное сравнение содержимого). Несовместимые типы (например, `"1" == 1`) вызывают ошибку компиляции.
- Идентичность: `===` и `!==` (строгая проверка равенства указателей в памяти).
- Операторы порядка: `<`, `<=`, `>`, `>=`.

### Логические операторы
- Логическое И: `and` или `&&`
- Логическое ИЛИ: `or` или `||`
- Логическое НЕ: `not` или `!`

### Диапазоны (Ranges)
- Полуоткрытый диапазон: `a..b` (`[a, b)`): от `a` до `b - 1` (например, `0..3` дает `0, 1, 2`).
- Включительный диапазон: `a..=b` (`[a, b]`): от `a` до `b` включительно (например, `0..=3` дает `0, 1, 2, 3`).

### Функциональные конвейеры (`|>`)
Оператор `x |> f(...)` передает `x` в качестве первого аргумента в `f(...)`:
```alca
val := 10 |> double |> add(5); // add(double(10), 5) -> 25
```

### Потоковые конвейеры ввода-вывода (`>>`, `>!`, `>>>`, `<<`, `&>>`, `?>>`, `?>>>`)
- `data >> "file.log"`: Дозапись в конец файла или HTTP POST по сети (`STREAM_APPEND`).
- `data >! "file.txt"`: Перезапись содержимого файла (`STREAM_OVERWRITE`).
- `expr >>> var_name`: Сохранение результата выражения в переменную среды (`STREAM_VAR`).
- `var_name << "file.txt"` или `var_name << console`: Чтение из файла или консоли (`STREAM_IN`).
- `data &>> "file.log"`: Неблокирующая асинхронная запись в фоновом потоке (`STREAM_ASYNC`).
- `data &&>> "https://..."`: Асинхронный сетевой стрим (`STREAM_THREAD`).
- `expr ?>> "errors.log"`: Перенаправление ошибки в лог-файл.
- `expr ?>>> err_var`: Перехват ошибки в переменную `error | null` без прерывания программы.
- Сброс потока: отправка в `null`, `"/dev/null"` или `"NUL"` отбрасывает вывод.

### Принадлежность
- `item in collection` проверяет наличие элемента в списке, ключа в словаре или подстроки в строке.

---

## 4. Управляющие конструкции

### If / Elif / Else
Условия обязаны строго возвращать тип `bool`:
```alca
if score >= 90 {
    println("Отлично");
} elif score >= 75 {
    println("Хорошо");
} else {
    println("Удовлетворительно");
}
```

### Циклы For и While
```alca
for i in 0..5 {
    if i == 2 {
        continue;
    }
    println(i);
}

names := ["Алиса", "Борис", "Виктор"];
for name in names {
    println("Привет, " + name);
}

counter := 3;
while counter > 0 {
    counter = counter - 1;
}
```

---

## 5. Функции, замыкания и асинхронность

### Сигнатуры функций
```alca
pub fn multiply(x: int, y: int = 1) -> int {
    return x * y;
}

void log_info(msg: string) {
    println("[INFO] " + msg);
}
```

### Замыкания (Closures)
```alca
fn make_adder(base: int) -> func {
    fn add(n: int) -> int {
        return base + n;
    }
    return add;
}

add10 := make_adder(10);
assert add10(5) == 15;
```

### Асинхронность (`async` / `await`)
Полный паритет между Bytecode VM и интерпретатором:
```alca
async fn fetch_status(endpoint: string) -> string {
    return "200 OK from " + endpoint;
}

status := await fetch_status("https://api.alca.org");
assert status == "200 OK from https://api.alca.org";
```

### Фоновые вычисления (`@ba`, `@threads`)
```alca
ba(timeout=5s):
    println("Фоновая задача исполняется...");

threads 4:
    println("Параллельная обработка в пуле потоков");
```

---

## 6. Структуры, классы и интерфейсы

### Структуры (`struct`)
```alca
struct Point {
    x: int;
    y: int = 0;
}

p := Point(10, 20);
p.x = 15;
assert p.x == 15;
```

### Классы и одиночное наследование
```alca
class Account {
    pub owner: string;
    private balance: float = 0.0;

    pub fn deposit(amount: float) -> void {
        self.balance = self.balance + amount;
    }

    pub fn get_balance() -> float {
        return self.balance;
    }
}

class PremiumAccount extends Account {
    pub bonus_rate: float = 0.05;

    pub fn deposit_with_bonus(amount: float) -> void {
        bonus := amount * self.bonus_rate;
        super.deposit(amount + bonus);
    }
}

acc := PremiumAccount("Alice", 100.0, 0.1);
acc.deposit_with_bonus(100.0);
assert acc.get_balance() == 210.0;
```

### Перечисления и сопоставление (`enum`, `match`)
```alca
enum OrderStatus {
    Pending,
    Processing = 1,
    Shipped,
    Delivered,
}

status := OrderStatus.Shipped;
match status {
    case OrderStatus.Pending:
        println("Ожидание оплаты");
    case OrderStatus.Shipped:
        println("Заказ отправлен");
    else:
        println("Статус обновлен");
}
```

### Интерфейсы
```alca
interface Notifier {
    fn notify(msg: string) -> bool;
}

class EmailSender {
    pub fn notify(msg: string) -> bool {
        println("Email sent: " + msg);
        return true;
    }
}

sender: Notifier = EmailSender();
sender.notify("Внимание!");
```

---

## 7. Обработка ошибок

- Создание ошибок: `err(message, code=...)`.
- Перехват ошибок оператором `?>>>`:
```alca
use std.io;

failure: error | null = null;
content := io.read_file("missing.log") ?>>> failure;

if failure != null {
    println("Код ошибки: " + string(failure.code));
    println("Сообщение: " + failure.msg);
}
```

---

## 8. Глобальные встроенные функции и переменные

Доступны во всей программе без импорта модулей:
- **Ввод/вывод:** `print(...args)`, `println(...args)`, `input(prompt?)`.
- **Инспекция:** `len(x)`, `type(x)`, `typeof(x)`.
- **Конструкторы:** `int(x)`, `float(x)`, `string(x)`, `bool(x)`, `list(x)`.
- **Коллекции:** `append(list, elem)`, `push(list, elem)`, `pop(list)` (также доступна как `list.pop(list)` в `std.list`), `keys(map)`, `values(map)`, `range(start, end, step?)`.
- **Математика:** `abs(x)`, `min(a, b)`, `max(a, b)`.
- **Строки:** `str_trim`, `str_upper`, `str_lower`, `str_contains`, `str_starts_with`, `str_ends_with`, `str_split`, `str_replace`.
- **JSON (zero-import):** `json_encode(val)`, `json_decode(str)`, `json_prettify(val)`, `json_query(json, query)`. (Модульный доступ доступен через `use std.json;`: `json.parse`, `json.encode` / `json.stringify`, `json.prettify`, `json.query`, `json.validate`).
- **Управление процессом:** `sleep(ms)`, `exit(code?)`, `time_now()`, `exec(cmd_str)` (требует `--allow-run`).
- **Dunder-переменные:** `__file__`, `__dir__`, `__dirname__`, `__line__`, `__name__`, `__version__`.

> [!NOTE]
> Переменные окружения доступны через модуль `use std.env;`: `env.get_env(name)` и `env.set_env(name, val)`.
> Прежние глобальные шлюзовые псевдонимы (`read_file`, `write_file`, `file_exists`, `uuid`, `cpu_count` и др.) удалены из глобальной области для соблюдения строгих пространств имен и аудита безопасности; их необходимо импортировать из соответствующих модулей (`std.io`, `std.crypto`, `std.process`, `std.env`). Их вызов глобально генерирует ошибку компиляции.

---

## 9. Песочница на основе прав (Capabilities)

По умолчанию права доступа к ресурсам операционной системы заблокированы:
- Файлы: `--allow-read[=<путь>]`, `--allow-write[=<путь>]`
- Сеть: `--allow-net[=<хост[:порт]>]`
- Подпроцессы: `--allow-run[=<команда>]`
- Переменные окружения: `--allow-env[=<переменная>]`
- Нативные библиотеки: `--allow-ffi[=<библиотека>]`
- Полный доступ: `--allow-all`

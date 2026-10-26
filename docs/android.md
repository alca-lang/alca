# Разработка под Android на Alca (Zero-Toolchain APK Builder)

Alca предоставляет встроенный генератор Android-проектов и сборщик APK-файлов **без необходимости скачивать Android Studio, Gradle, Java JDK или гигабайтные Android SDK**.

Весь процесс создания, компиляции бинарного манифеста (AXML), упаковки Dalvik-байткода (`classes.dex`), сборки ресурсов и цифровой подписи (APK v1 JAR/PKCS#7) выполняется полностью встроенным движком Alca.

---

## 1. Быстрый старт

### Создание нового Android-проекта
```bash
alca android init MyApp
# или:
alca init --android MyApp
```

### Сборка подписанного APK
```bash
cd MyApp
alca android build
# или:
alca build --apk
```

Результат: готовый, автоматически подписанный файл `build/myapp.apk`, который можно сразу установить на телефон или эмулятор:
```bash
adb install build/myapp.apk
```

---

## 2. Структура Android-проекта Alca

```
MyApp/
├── alca.toml                # Конфигурация проекта и параметры Android
├── AndroidManifest.xml      # Стандартный манифест Android (Activity, права, темы)
├── src/
│   └── main.al              # Главная точка входа логики на языке Alca
├── res/
│   ├── values/
│   │   └── strings.xml      # Название приложения и строковые ресурсы
│   └── mipmap/
│       └── ic_launcher.png  # Иконка приложения (чистый PNG)
└── assets/
    ├── index.html           # Мобильный UI (HTML5 / WebView)
    ├── style.css            # Мобильные стили и темы
    └── app.js               # Мост (Bridge) между JavaScript и рантаймом Alca
```

---

## 3. Файлы проекта

### `alca.toml`
Определяет метаданные приложения:
```toml
[package]
name = "myapp"
version = "1.0.0"
description = "My First Alca Mobile App"
entry = "src/main.al"

[android]
package = "com.alca.myapp"
label = "My App"
version_code = 1
min_sdk = 21
target_sdk = 34
activity = "alca.runtime.MainActivity"
```

### `AndroidManifest.xml`
Стандартный XML манифест Android, который можно редактировать напрямую:
```xml
<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.alca.myapp"
    android:versionCode="1"
    android:versionName="1.0.0">

    <uses-sdk
        android:minSdkVersion="21"
        android:targetSdkVersion="34" />

    <uses-permission android:name="android.permission.INTERNET" />
    <uses-permission android:name="android.permission.ACCESS_NETWORK_STATE" />

    <application
        android:label="My App"
        android:icon="@mipmap/ic_launcher"
        android:allowBackup="true"
        android:supportsRtl="true">
        <activity
            android:name="alca.runtime.MainActivity"
            android:label="My App"
            android:exported="true"
            android:theme="@android:style/Theme.NoTitleBar.Fullscreen">
            <intent-filter>
                <action android:name="android.intent.action.MAIN" />
                <category android:name="android.intent.category.LAUNCHER" />
            </intent-filter>
        </activity>
    </application>
</manifest>
```

### `src/main.al`
Исходный код приложения на Alca:
```alca
println("=== Alca Mobile Engine Initialized ===");

pub fn init_app() {
    println("App services started.");
}

pub fn handle_event(event_name: string, payload: string) -> string {
    println("Event: " + event_name + " -> " + payload);
    if event_name == "ping":
        return "pong from Alca runtime!";
    return "OK: " + event_name;
}

init_app();
```

---

## 4. Команды CLI

| Команда | Описание |
|---|---|
| `alca android init [name]` | Создать структуру Android-проекта |
| `alca android build [-o out.apk]` | Скомпилировать и подписать `.apk` |
| `alca android check [dir]` | Проверить валидность манифеста и ресурсов |
| `alca build --apk [-o out.apk]` | Собрать APK (также распознается автоматически в папке с `AndroidManifest.xml`) |

---

## 5. Архитектура рантайма и поддерживаемое подмножество Alca

### Архитектурная модель
При сборке `alca android build` исходный код `src/main.al` проходит полный статический анализ и тайпчек (`pkg/typecheck`). После успешной проверки исходники транспилируются строгим компилятором в проверенный JavaScript-бандл (`assets/alca/runtime.js`), исполняемый в защищённом WebView мобильного приложения с нулевыми внешними зависимостями (без Gradle, NDK и Android Studio).

### Поддерживаемое подмножество Alca для Android
- **Переменные и константы:** `let`, `const`, типизированные объявления вида `name: Type = val;` или `name := val;`.
- **Функции:** `fn`, `pub fn`, параметры с типами и значениями по умолчанию. Функции с модификатором `pub` автоматически экспортируются в мобильный рантайм `window.Alca.modules[name]` и доступны для вызова из пользовательского интерфейса через `window.Alca.invoke(name, ...args)`.
- **Управляющие конструкции:** `if / elif / else`, `while`, `loop`, `break`, `continue`.
- **Циклы и диапазоны:**
  - `for i in 0..10:` — полуоткрытый интервал `[0, 10)` (0..9).
  - `for i in 0..=10:` — включительный интервал `[0, 10]` (0..10).
  - `for item in list:` — итерация по элементам списков и массивов.
- **Структуры данных:**
  - Списки (`[1, 2, 3]`), мапы (`{"key": "value"}`), индексация (`items[i]`), доступ к полям (`obj.property`).
  - `struct` и `class` (с полями, методами, конструкторами и наследованием).
  - `enum` с именованными вариантами, числовыми тегами и сопоставлением через `match`.
- **Операторы:**
  - Логические операторы `and`, `or`, `not` (транспилируются в `&&`, `||`, `!`).
  - Равенство: структурное `==` и `!=` (в JS `===` и `!==`).
  - Целочисленное деление `/` (транспилируется с `Math.trunc(a / b)` для строгого сохранения семантики Alca).
- **Встроенные функции:** `println`, `print`, `len`, `string`, `int`, `float`.

### Ограничения и строгая диагностика
Низкоуровневые API десктопных платформ (прямой Win32 FFI, системные процессы, потоки `@threads`, фоновые блоки `@ba`) не поддерживаются в WebView-таргете. Если в коде мобильного приложения встречается неподдерживаемая конструкция, транспилер **не проглатывает её молча**, а возвращает строгую ошибку с указанием файла и номера строки:
```
error: transpiling src/main.al: transpiler error at line 14: unsupported statement *ast.BaStatement ("ba")
```
Сборка APK в этом случае немедленно завершается с ненулевым кодом возврата.

### Мобильный мост `window.Alca`
Интерфейс связи UI и логики Alca включает:
- `window.Alca.invoke(name, ...args)` — безопасный вызов функций Alca из UI.
- `window.Alca.vibrate(ms)` — тактильный отклик (требует `<uses-permission android:name="android.permission.VIBRATE" />` в `AndroidManifest.xml`).
- `window.Alca.toast(message)` — визуальные уведомления.
- `window.Alca.getDeviceInfo()` — метаданные экрана и устройства.

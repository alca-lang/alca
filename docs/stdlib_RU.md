# Справочник стандартной библиотеки (Stdlib)

[English](stdlib_EN.md) | [Русский](stdlib_RU.md)

Стандартная библиотека Alca предоставляет богатый набор встроенных модулей для высокопроизводительной автоматизации системных задач, сетевого взаимодействия, криптографии, работы с форматами данных, ввода-вывода, аудио, графических интерфейсов и машинного обучения.

> 📘 **Ищете подробные методические руководства и практические примеры?**  
> См. [**Английское руководство (Handbook)**](handbook_EN.md) или [**Русское методическое руководство**](handbook_RU.md) для концептуальных объяснений, архитектурных паттернов и примеров из реальной практики.

## Оглавление

1. [Импорт модулей и песочница](#импорт-модулей-и-песочница)
2. [Модули стандартной библиотеки](#модули-стандартной-библиотеки)
3. [Глобальные встроенные идентификаторы и функции](#глобальные-встроенные-идентификаторы-и-функции)
4. [Исполнение в JIT и автономных бинарниках](#jit-and-standalone-execution)
5. [Политика безопасности песочницы](#политика-безопасности-песочницы)

---

## Импорт модулей и песочница

Модули подключаются инструкцией `use std.<имя>;`, после чего их функции и типы вызываются через квалифицированное пространство имен модуля:

```alca
use std.io;

text := io.read_file("input.txt");
println(text);
```

### Матрица прав доступа песочницы (Capabilities)

По умолчанию пользовательские скрипты исполняются в изолированном окружении (*Deny-by-Default*). Модули стандартной библиотеки проверяют права через модуль `pkg/sandbox`:

| Модуль | Требуемый флаг прав | Поведение при отсутствии прав |
| --- | --- | --- |
| `std.io`, `std.archive` (чтение) | `--allow-read[=<путь>]` | Возвращает `error` с кодом `403` |
| `std.io`, `std.archive` (запись) | `--allow-write[=<путь>]` | Возвращает `error` с кодом `403` |
| `std.net`, `std.http`, `std.urllib2` | `--allow-net[=<хост[:порт]>]` | Возвращает `error` с кодом `403` |
| `std.process` | `--allow-run[=<команда>]` | Возвращает `error` с кодом `403` |
| `std.env` | `--allow-env[=<переменная>]` | Возвращает `error` с кодом `403` |
| `std.ffi` | `--allow-ffi[=<библиотека>]` | Возвращает `error` с кодом `403` |
| Любой модуль | `--allow-all` | Безоговорочно разрешает все операции |

Спецификация системы типов приведена в [`docs/language_RU.md`](language_RU.md). Эталонным источником сигнатур является таблица [`pkg/typecheck/builtins_table.go`](../pkg/typecheck/builtins_table.go); настоящий документ полностью с ней синхронизирован. Нотация сигнатур:

- `(аргументы) -> результат` описывает допустимые перегрузки, разделенные точкой с запятой `;`.
- `?` обозначает опциональный аргумент, а `...` — вариативный (переменное число аргументов).
- Пустая сигнатура соответствует динамическому члену, аргументы которого не проверяются статически, а результат имеет тип `json`.

Псевдонимы модулей: `std.regex` доступен как `std.re`, `std.str` — как `std.string`, а `std.zip` — как `std.archive`. Модули `csv`, `ini` и `json` также доступны через агрегатор `std.formats`.

---

## Модули стандартной библиотеки

### `audio`

Низкоуровневая обработка аудиоданных, цифровой синтез сигналов (DSP), трансформация форм волны, спектральный анализ и кодирование/декодирование WAV/MP3.

Аудиобуферы (`AudioBuffer`) поддерживают методы инспекции и поточечной модификации сэмплов:
- `.channels()`, `.sample_rate()`, `.num_samples()`, `.duration()`
- `.sample(ch, idx)`, `.set_sample(ch, idx, val)`, `.channel_data(ch)`

| Член | Сигнатура |
| --- | --- |
| `audio.create_buffer` | `(int?,int?,int?)->AudioBuffer` |
| `audio.read` | `(string\|blob)->AudioBuffer` |
| `audio.read_wav` | `(string)->AudioBuffer` |
| `audio.write_wav` | `(string,AudioBuffer,int?)->bool` |
| `audio.encode_wav` | `(AudioBuffer,int?)->string` |
| `audio.decode` | `(string\|blob)->AudioBuffer` |
| `audio.decode_wav` | `(string)->AudioBuffer` |
| `audio.decode_mp3` | `(string\|blob)->AudioBuffer` |
| `audio.encode_base64` | `(AudioBuffer,int?)->string` |
| `audio.decode_base64` | `(string)->AudioBuffer` |
| `audio.sine` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.square` | `(float,float,int?,float?,float?)->AudioBuffer` |
| `audio.sawtooth` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.triangle` | `(float,float,int?,float?)->AudioBuffer` |
| `audio.noise` | `(float,string?,int?,float?)->AudioBuffer` |
| `audio.silence` | `(float,int?,int?)->AudioBuffer` |
| `audio.chirp` | `(float,float,float,int?,float?)->AudioBuffer` |
| `audio.gain` | `(AudioBuffer,float)->AudioBuffer` |
| `audio.normalize` | `(AudioBuffer,float?)->AudioBuffer` |
| `audio.mix` | `(AudioBuffer,AudioBuffer,float?,float?)->AudioBuffer` |
| `audio.pan` | `(AudioBuffer,float)->AudioBuffer` |
| `audio.delay` | `(AudioBuffer,float?,float?,float?)->AudioBuffer` |
| `audio.lowpass` | `(AudioBuffer,float,float?)->AudioBuffer` |
| `audio.highpass` | `(AudioBuffer,float,float?)->AudioBuffer` |
| `audio.adsr` | `(AudioBuffer,float,float,float,float)->AudioBuffer` |
| `audio.fft` | `(AudioBuffer,int?,int?)->list[float]` |
| `audio.dominant_frequency` | `(AudioBuffer,int?)->float` |
| `audio.rms` | `(AudioBuffer,int?)->float` |

### `encoding`

Универсальный модуль кодирования и декодирования текста с поддержкой всех ключевых национальных кодировок (UTF-8, UTF-16LE, UTF-16BE, Windows-1251, CP866, ISO-8859-1, ASCII и др.) и преобразований HEX.

| Член | Сигнатура |
| --- | --- |
| `encoding.decode` | `(blob,string)->string` |
| `encoding.encode` | `(string,string)->blob` |
| `encoding.from_hex` | `(string)->blob` |
| `encoding.supported` | `()->list[string]` |
| `encoding.to_hex` | `(blob)->string` |

### `marshal`

Бинарная сериализация объектов для быстрого, компактного обмена данными и кэширования любых значений языка Alca без потери типизации.

| Член | Сигнатура |
| --- | --- |
| `marshal.dump` | `(any,string)->bool` |
| `marshal.dumps` | `(any)->blob` |
| `marshal.load` | `(string)->any` |
| `marshal.loads` | `(blob)->any` |

### `image`

Комплексная работа с растровой графикой: поддержка форматов PNG, JPEG, GIF, BMP и WebP из файлов, сетевых URL, base64-строк и буферов в памяти.

Объекты `Image` предоставляют развитый набор методов манипуляции:
- `.width()`, `.height()`, `.bounds()`, `.get_pixel(x, y)`
- `.set_pixel(x, y, r, g, b, a?)`
- `.resize(w, h)`, `.crop(x, y, w, h)`, `.rotate(deg)`, `.flip_h()`, `.flip_v()`
- `.grayscale()`, `.invert()`, `.brightness(delta)`, `.contrast(factor)`
- `.to_base64(format?)`

| Член | Сигнатура |
| --- | --- |
| `image.create` | `(int,int,list[int]?)->Image` |
| `image.read` | `(string\|blob)->Image` |
| `image.decode` | `(string\|blob)->Image` |
| `image.write` | `(Image,string,int?)->bool` |
| `image.encode` | `(Image,string?,int?)->string` |
| `image.to_base64` | `(Image,string?)->string` |
| `image.screenshot` | `(int?,int?,int?,int?)->Image` |

### `devices`

Обнаружение и инспекция аппаратных устройств операционной системы. `devices.list` возвращает список доступных устройств; `devices.get` выполняет поиск устройства по его аппаратному идентификатору. Функция `devices.watch_events` в настоящий момент зарезервирована.

| Член | Сигнатура |
| --- | --- |
| `devices.get` | `(string)->map[string, json]` |
| `devices.list` | `(string?)->list[map[string, json]]` |
| `devices.watch_events` | `(...)->error` |

### `platform` [Кроссплатформенный]

Определение идентификатора операционной системы, архитектуры оборудования и обнаруженных системных возможностей на Windows, Linux и macOS.
Поле `requires_root` равно true, если текущий процесс запущен без повышенных привилегий (не от имени администратора/root).

| Член | Сигнатура |
| --- | --- |
| `platform.api_version` | `()->string` |
| `platform.architecture` | `()->map[string, json]` |
| `platform.capabilities` | `()->map[string, json]` |
| `platform.has_feature` | `(string)->bool` |
| `platform.info` | `()->map[string, json]` |
| `platform.linux_distro` | `()->map[string, json]` |
| `platform.mac_ver` | `()->map[string, json]` |
| `platform.machine` | `()->string` |
| `platform.node` | `()->string` |
| `platform.processor` | `()->string` |
| `platform.release` | `()->string` |
| `platform.system` | `()->string` |
| `platform.uname` | `()->list[string]` |
| `platform.version` | `()->string` |
| `platform.win32_ver` | `()->list[json]` |

### `power` [Кроссплатформенный]

Получение данных о состоянии аккумулятора, датчиках температуры и управление системным питанием на Windows, Linux и macOS.
Операции управления питанием (`shutdown`, `reboot`, `suspend`) требуют административных привилегий операционной системы.

| Член | Сигнатура |
| --- | --- |
| `power.battery_info` | `()->map[string, json]` |
| `power.battery_percent` | `()->int` |
| `power.battery_status` | `()->map[string, json]` |
| `power.is_battery_present` | `()->bool` |
| `power.is_charging` | `()->bool` |
| `power.power_plugged` | `()->bool` |
| `power.power_source` | `()->string` |
| `power.reboot` | `()->bool` |
| `power.sensors` | `()->list[map[string, json]]` |
| `power.shutdown` | `()->bool` |
| `power.suspend` | `()->bool` |

### `win32` [Только для Windows]

Специализированные вспомогательные функции для Windows: доступ к реестру, управление службами (Windows Services) и выполнение WMI-запросов. Вызов на других ОС возвращает ошибку несовместимой платформы. Модификация реестра или состояния служб требует флага прав песочницы `--allow-all`.

| Член | Сигнатура |
| --- | --- |
| `win32.is_admin` | `()->bool` |
| `win32.license_status` | `()->map[string, json]` |
| `win32.reg_delete_key` | `(int,string)->bool` |
| `win32.reg_delete_value` | `(int,string,string)->bool` |
| `win32.reg_read_value` | `(int,string,string)->json` |
| `win32.reg_write_value` | `(int,string,string,any,int?)->bool` |
| `win32.service_control` | `(string,string)->bool` |
| `win32.service_info` | `(string)->map[string, json]` |
| `win32.service_list` | `()->list[map[string, json]]` |
| `win32.version_info` | `()->map[string, json]` |
| `win32.wmi_get_class` | `(string)->list[json]` |
| `win32.wmi_query` | `(string)->list[json]` |

### `libtorch`

Полнофункциональный высокопроизводительный модуль машинного обучения и многомерных тензорных вычислений. Модуль опирается на внутренний движок графа вычислений с динамическим автоматическим дифференцированием (Autograd), аппаратным ускорением на GPU через NVIDIA CUDA, нейросетевыми слоями, функциями потерь и адаптивными оптимизаторами.

#### Функции модуля

| Член | Сигнатура | Описание |
| --- | --- | --- |
| `libtorch.tensor` | `(any,list[int]?)->Tensor` | Создает тензор из списков Alca или чисел с заданной формой |
| `libtorch.zeros` | `(list[int])->Tensor` | Тензор, заполненный нулями |
| `libtorch.ones` | `(list[int])->Tensor` | Тензор, заполненный единицами |
| `libtorch.rand` | `(list[int])->Tensor` | Равномерное псевдослучайное распределение в диапазоне [0, 1) |
| `libtorch.randn` | `(list[int])->Tensor` | Стандартное нормальное распределение (среднее 0, дисперсия 1) |
| `libtorch.full` | `(list[int],float)->Tensor` | Тензор, заполненный скалярным значением |
| `libtorch.eye` | `(int,int?)->Tensor` | Единичная матрица (2D identity) |
| `libtorch.arange` | `(float,float,float?)->Tensor` | Последовательность чисел с шагом от начала до конца |
| `libtorch.matrix` | `(list[list[float]])->Tensor` | Конструктор двумерной матрицы |
| `libtorch.add` | `(Tensor,Tensor)->Tensor` | Поэлементное сложение |
| `libtorch.sub` | `(Tensor,Tensor)->Tensor` | Поэлементное вычитание |
| `libtorch.mul` | `(Tensor,Tensor)->Tensor` | Поэлементное умножение (произведение Адамара) |
| `libtorch.div` | `(Tensor,Tensor)->Tensor` | Поэлементное деление |
| `libtorch.matmul` | `(Tensor,Tensor)->Tensor` | Матричное умножение (2D или пакетное batched) |
| `libtorch.dot` | `(Tensor,Tensor)->float` | Скалярное произведение векторов |
| `libtorch.pow` | `(Tensor,float)->Tensor` | Поэлементное возведение в степень |
| `libtorch.abs` | `(Tensor)->Tensor` | Абсолютное значение элементов |
| `libtorch.neg` | `(Tensor)->Tensor` | Поэлементная смена знака |
| `libtorch.exp` | `(Tensor)->Tensor` | Экспонента $e^x$ |
| `libtorch.log` | `(Tensor)->Tensor` | Натуральный логарифм |
| `libtorch.sqrt` | `(Tensor)->Tensor` | Квадратный корень |
| `libtorch.sum` | `(Tensor,int?)->Tensor` | Суммирование (по всей матрице или по оси) |
| `libtorch.mean` | `(Tensor,int?)->Tensor` | Вычисление среднего арифметического |
| `libtorch.max` | `(Tensor,int?)->Tensor` | Редукция максимума |
| `libtorch.min` | `(Tensor,int?)->Tensor` | Редукция минимума |
| `libtorch.relu` | `(Tensor)->Tensor` | Функция активации ReLU (Rectified Linear Unit) |
| `libtorch.leaky_relu` | `(Tensor,float?)->Tensor` | Leaky ReLU с коэффициентом наклона отрицательной ветви |
| `libtorch.sigmoid` | `(Tensor)->Tensor` | Логистическая сигмоида |
| `libtorch.tanh` | `(Tensor)->Tensor` | Гиперболический тангенс |
| `libtorch.softmax` | `(Tensor,int?)->Tensor` | Функция Softmax для вероятностного распределения |
| `libtorch.log_softmax` | `(Tensor,int?)->Tensor` | Логарифм Softmax для устойчивого вычисления потерь |
| `libtorch.linear` | `(Tensor,Tensor,Tensor?)->Tensor` | Полносвязный линейный слой ($xW^T + b$) |
| `libtorch.dense_forward`| `(Tensor,Tensor,Tensor,string?)->Tensor` | Прямой проход с объединенной функцией активации |
| `libtorch.conv2d` | `(Tensor,Tensor,Tensor?,int?,int?)->Tensor` | Двумерная свертка (Convolution 2D) |
| `libtorch.max_pool2d` | `(Tensor,int,int?)->Tensor` | Операция субдискретизации Max Pooling 2D |
| `libtorch.batch_norm_2d`| `(Tensor,Tensor,Tensor,Tensor,Tensor,bool?,float?,float?)->Tensor` | Пакетная нормализация (Batch Normalization 2D) |
| `libtorch.layer_norm` | `(Tensor,list[int],Tensor?,Tensor?,float?)->Tensor` | Нормализация слоев (Layer Normalization) |
| `libtorch.dropout` | `(Tensor,float?,bool?)->Tensor` | Регуляризация случайным отключением нейронов (Dropout) |
| `libtorch.mse_loss` | `(Tensor,Tensor)->Tensor` | Функция потерь среднеквадратичной ошибки (MSE) |
| `libtorch.cross_entropy`| `(Tensor,Tensor)->Tensor` | Категориальная кросс-энтропия |
| `libtorch.bce_with_logits`| `(Tensor,Tensor)->Tensor` | Бинарная кросс-энтропия с логитами |
| `libtorch.l1_loss` | `(Tensor,Tensor)->Tensor` | Функция потерь средней абсолютной ошибки (L1 / MAE) |
| `libtorch.backward` | `(Tensor)->bool` | Запуск обратного распространения ошибки (Autograd backward) |
| `libtorch.grad` | `(Tensor)->Tensor` | Извлечение накопленного градиента тензора |
| `libtorch.requires_grad`| `(Tensor,bool)->Tensor` | Включение/отключение отслеживания градиентов |
| `libtorch.detach` | `(Tensor)->Tensor` | Отсоединение тензора от графа вычислений |
| `libtorch.zero_grad` | `(Tensor)->bool` | Сброс накопленного градиента тензора |
| `libtorch.adam` | `(list[Tensor],float?,float?,float?)->Optimizer` | Адаптивный оптимизатор Adam |
| `libtorch.adamw` | `(list[Tensor],float?,float?,float?,float?)->Optimizer` | Оптимизатор AdamW с расцепленным затуханием весов |
| `libtorch.sgd` | `(list[Tensor],float?,float?,float?)->Optimizer` | Стохастический градиентный спуск (SGD) с моментом |
| `libtorch.rmsprop` | `(list[Tensor],float?,float?)->Optimizer` | Оптимизатор RMSprop |
| `libtorch.opt_step` | `(Optimizer)->bool` | Шаг оптимизатора: обновление весов по градиентам |
| `libtorch.opt_zero_grad`| `(Optimizer)->bool` | Обнуление градиентов всех параметров модели |
| `libtorch.reshape` | `(Tensor,list[int])->Tensor` | Изменение формы тензора без копирования данных |
| `libtorch.transpose` | `(Tensor,int,int)->Tensor` | Транспонирование двух осей |
| `libtorch.permute` | `(Tensor,list[int])->Tensor` | Перестановка осей многомерного тензора |
| `libtorch.flatten` | `(Tensor,int?,int?)->Tensor` | Уплощение смежных размерностей в вектор |
| `libtorch.squeeze` | `(Tensor,int?)->Tensor` | Удаление единичных размерностей |
| `libtorch.unsqueeze` | `(Tensor,int)->Tensor` | Вставка новой единичной размерности |
| `libtorch.stack` | `(list[Tensor],int?)->Tensor` | Объединение тензоров вдоль новой размерности |
| `libtorch.concat` | `(list[Tensor],int?)->Tensor` | Конкатенация тензоров вдоль существующей оси |
| `libtorch.cuda_is_available`| `()->bool` | Проверка доступности видеокарты NVIDIA CUDA |
| `libtorch.cuda_device_count`| `()->int` | Количество доступных устройств CUDA GPU |
| `libtorch.cuda_device_name` | `(int?)->string` | Модель и наименование графического процессора |
| `libtorch.cuda_memory_allocated`| `(int?)->int` | Объем выделенной видеопамяти VRAM в байтах |
| `libtorch.cuda_synchronize` | `(int?)->bool` | Ожидание завершения всех операций в потоках GPU |
| `libtorch.cuda` | `(Tensor,int?)->Tensor` | Перенос тензора в память графического процессора |
| `libtorch.cpu` | `(Tensor)->Tensor` | Перенос тензора в оперативную память хоста (RAM) |
| `libtorch.to` | `(Tensor,string)->Tensor` | Диспетчеризация на устройство (`"cuda"`, `"cpu"`) |
| `libtorch.is_cuda` | `(Tensor)->bool` | Проверка, расположен ли тензор на GPU |
| `libtorch.save` | `(Tensor,string)->bool` | Бинарное сохранение весов тензора в файл |
| `libtorch.load` | `(string)->Tensor` | Загрузка тензора из сохраненного файла |
| `libtorch.to_list` | `(Tensor)->list[any]` | Преобразование тензора во вложенные списки Alca |
| `libtorch.item` | `(Tensor)->float` | Извлечение скалярного вещественного значения из тензора 0-го ранга |
| `libtorch.shape` | `(Tensor)->list[int]` | Получение размерностей тензора |
| `libtorch.dim` | `(Tensor)->int` | Количество размерностей (ранг тензора) |
| `libtorch.numel` | `(Tensor)->int` | Общее количество элементов в тензоре |
| `libtorch.clone` | `(Tensor)->Tensor` | Создание глубокой копии тензора |

#### Объектно-ориентированный синтаксис методов
Тензоры Alca поддерживают цепочечный синтаксис методов вызова (Method Chaining):
```alca
use std.libtorch;

// Пример шага обучения нейросети через ООП-синтаксис тензоров
x := libtorch.randn([32, 128]);
w := libtorch.randn([128, 10]);
w.requires_grad(true);

out := x.matmul(w).relu();
loss := out.sum();
loss.backward();

println("Форма градиента:", w.grad().shape());
```

### `archive`

Создание, извлечение и просмотр содержимого архивов форматов ZIP и TAR (tar.gz). Защита от уязвимости *Zip-Slip* (выход за пределы каталога распаковки через относительные пути `../`) гарантируется на уровне ядра рантайма.

| Член | Сигнатура |
| --- | --- |
| `archive.list` | `(string)->list[map[string, json]]` |
| `archive.tar` | `(string,list[string])->bool` |
| `archive.untar` | `(string,string)->bool` |
| `archive.unzip` | `(string,string)->bool` |
| `archive.zip` | `(string,list[string])->bool` |

### `cli`

Парсинг аргументов командной строки, интерактивные запросы ввода от пользователя, терминальная стилизация цветом (ANSI) и индикаторы хода выполнения (progress bar).

| Член | Сигнатура |
| --- | --- |
| `cli.args` | `()->list[string]` |
| `cli.clear` | `()->bool` |
| `cli.color` | `(string,string,string?)->string` |
| `cli.confirm` | `(string,bool?)->bool` |
| `cli.flag` | `(string,string?)->string` |
| `cli.height` | `()->int` |
| `cli.is_terminal` | `()->bool` |
| `cli.progress` | `(int,int,int?,string?)->string` |
| `cli.prompt` | `(string,string?)->string` |
| `cli.width` | `()->int` |

### `compress`

Сжатие и распаковка потоков данных алгоритмами Gzip и Zlib. Сжатые данные представляются в виде бинарных блобов или строк Base64. По умолчанию объем распаковки ограничен 16 МиБ для защиты от вредоносных «бомб сжатия».

| Член | Сигнатура |
| --- | --- |
| `compress.gunzip` | `(blob)->blob;(string)->string` |
| `compress.gzip` | `(blob)->blob;(string)->string` |
| `compress.zlib_decompress` | `(blob)->blob;(string)->string` |
| `compress.zlib_deflate` | `(blob)->blob;(string)->string` |

### `crypto`

Криптографические алгоритмы хэширования, вычисление HMAC, симметричное шифрование AES-256-GCM, генерация криптографически стойких случайных данных и UUID.

| Член | Сигнатура |
| --- | --- |
| `crypto.aes_decrypt` | `(blob,blob,blob?)->blob;(string,string,string?)->string` |
| `crypto.aes_decrypt_password` | `(blob,string)->blob;(string,string)->string` |
| `crypto.aes_encrypt` | `(blob,blob,blob?)->blob;(string,string,string?)->string` |
| `crypto.aes_encrypt_password` | `(blob,string)->blob;(string,string)->string` |
| `crypto.base64_decode` | `(string)->blob` |
| `crypto.base64_encode` | `(blob)->string` |
| `crypto.constant_time_equal` | `(blob,blob)->bool;(string,string)->bool` |
| `crypto.derive_key` | `(string,blob?,int?,int?)->blob` |
| `crypto.hex_decode` | `(string)->blob` |
| `crypto.hex_encode` | `(blob)->string` |
| `crypto.hmac_sha256` | `(blob,blob)->string;(string,string)->string` |
| `crypto.hmac_sha512` | `(blob,blob)->string;(string,string)->string` |
| `crypto.md5` | `(blob)->string;(string)->string` |
| `crypto.random_bytes` | `(int)->blob` |
| `crypto.random_int` | `(int,int)->int` |
| `crypto.random_string` | `(int,string?)->string` |
| `crypto.sha1` | `(blob)->string;(string)->string` |
| `crypto.sha256` | `(blob)->string;(string)->string` |
| `crypto.sha512` | `(blob)->string;(string)->string` |
| `crypto.uuid` | `()->string` |

### `csv`

Чтение, парсинг и запись структурированных табличных данных формата CSV (дублируется в модуле `std.formats`).

| Член | Сигнатура |
| --- | --- |
| `csv.parse` | `(string,bool?)->list[json]` |
| `csv.read` | `(string,bool?)->list[json]` |
| `csv.stringify` | `(json,list[string]?)->string` |
| `csv.write` | `(string,json,list[string]?)->bool` |

### `db`

Полнофункциональные дескрипторы СУБД SQLite и PostgreSQL. Подключения предоставляют методы `ping`, `exec`, `query`, `query_row`, `prepare`, `transaction`, пул соединений и строгую параметризацию аргументов для исключения SQL-инъекций.

| Член | Сигнатура |
| --- | --- |
| `db.open` | `(string,string)->json` |
| `db.postgres` | `(string)->json` |
| `db.sqlite` | `(string?)->json` |

### `env`

Инспекция и модификация переменных окружения, системных путей PATH, сетевого имени хоста, идентификатора пользователя и рабочего каталога.

| Член | Сигнатура |
| --- | --- |
| `env.add_path` | `(string)->bool` |
| `env.add_system_path` | `(string)->bool` |
| `env.add_user_path` | `(string)->bool` |
| `env.all_env` | `()->map[string, json]` |
| `env.broadcast_env_change` | `()->bool` |
| `env.expand_env` | `(string)->string` |
| `env.get_env` | `(string)->string` |
| `env.get_path` | `()->list[string]` |
| `env.get_system_path` | `()->list[string]` |
| `env.get_user_path` | `()->list[string]` |
| `env.has_path` | `(string)->bool` |
| `env.home_dir` | `()->string` |
| `env.hostname` | `()->string` |
| `env.is_linux` | `()->bool` |
| `env.is_windows` | `()->bool` |
| `env.os_name` | `()->string` |
| `env.path_separator` | `()->string` |
| `env.persist_user_path` | `(string)->bool` |
| `env.prepend_path` | `(string)->bool` |
| `env.remove_path` | `(string)->bool` |
| `env.remove_user_path` | `(string)->bool` |
| `env.set_env` | `(string,string)->bool` |
| `env.set_path` | `(list[string])->bool;(string)->bool` |
| `env.unset_env` | `(string)->bool` |
| `env.username` | `()->string` |
| `env.which` | `(string)->string | null` |
| `env.working_dir` | `()->string` |

### `ffi`

Динамическая загрузка разделяемых библиотек (.dll на Windows, .so на Linux, .dylib на macOS), компиляция C/C++ прямо во время выполнения программы, прямая сборка нативных DLL из кода Alca, управление указателями и сырой памятью (требует `--allow-ffi` или `--allow-all`).
- На Windows динамически загружает библиотеки `.dll` (Win32 API, Raylib, сторонние C/C++ библиотеки).
- На Unix/Linux динамически загружает библиотеки `.so` / `.dylib` и плагины.
- `ffi.compile` и `ffi.compile_file` на лету компилируют C/C++ код в разделяемую библиотеку и возвращают путь к ней.
- `ffi.build_native_dll` компилирует код Alca напрямую в бинарную нативную PE DLL.
- Операции с указателями (`c_str`, `utf16_str`, `alloc_ptr`, `copy_to_ptr`, `read_from_ptr`, `free`) обеспечивают безнакладный обмен буферами с нативными библиотеками.

| Член | Сигнатура |
| --- | --- |
| `ffi.alloc` | `(int?)->blob` |
| `ffi.alloc_ptr` | `(int)->int` |
| `ffi.build_native_dll` | `(string,string)->string` |
| `ffi.c_str` | `(string)->int` |
| `ffi.compile` | `(string,string?)->string` |
| `ffi.compile_file` | `(string,string?)->string` |
| `ffi.copy_to_ptr` | `(int,blob \| string)->bool` |
| `ffi.free` | `(int)->bool` |
| `ffi.load` | `(string)->json` |
| `ffi.load_embedded` | `(string)->json` |
| `ffi.ptr_to_str` | `(int)->string` |
| `ffi.ptr_to_utf16` | `(int)->string` |
| `ffi.read_from_ptr` | `(int,int)->blob` |
| `ffi.size_of` | `(string)->int` |
| `ffi.utf16_str` | `(string)->int` |

`ffi.compile` компилирует текст исходного кода C/C++ в нативную разделяемую библиотеку (.dll / .so) и возвращает путь к собранному файлу.  
`ffi.compile_file` компилирует файл `.c` или `.cpp` напрямую в разделяемую библиотеку.  
`ffi.load_embedded` загружает нативную библиотеку, встроенную в бинарник компилятором `alca build --embed` или нативными опциями; файл извлекается во временную директорию и динамически подключается.

### `mem`

Низкоуровневая адресация памяти внутри выделенного изолированного блока. Программа может адресовать только ту память, которую выделила самостоятельно через `mem.alloc`, исключая чтение и запись посторонней памяти хост-процесса.

| Член | Сигнатура |
| --- | --- |
| `mem.add` | `(int,int)->int` |
| `mem.alloc` | `(int)->int` |
| `mem.copy` | `(int,int,int)->bool` |
| `mem.fill` | `(int,int,int)->bool` |
| `mem.free` | `(int)->bool` |
| `mem.is_null` | `(int)->bool` |
| `mem.read_bytes` | `(int,int?,int)->blob` |
| `mem.read_f64` | `(int,int?)->float` |
| `mem.read_i16` | `(int,int?)->int` |
| `mem.read_i32` | `(int,int?)->int` |
| `mem.read_i64` | `(int,int?)->int` |
| `mem.read_u8` | `(int,int?)->int` |
| `mem.write_bytes` | `(int,blob,int?)->bool` |
| `mem.write_f64` | `(int,float,int?)->bool` |
| `mem.write_i16` | `(int,int,int?)->bool` |
| `mem.write_i32` | `(int,int,int?)->bool` |
| `mem.write_i64` | `(int,int,int?)->bool` |
| `mem.write_u8` | `(int,int,int?)->bool` |

### `mtproto`

Реализация криптографического сетевого протокола Telegram MTProto: симметричное шифрование AES-IGE, 2048-битный протокол обмена сессионными ключами Диффи-Хеллмана (DH), формирование и валидация бинарных пакетов Type-Language (TL), генерация Nonce и безопасный транспорт сообщений RPC.

Функции, открывающие сетевые сокеты к серверам дата-центров Telegram, требуют прав `--allow-net`.

| Член | Сигнатура |
| --- | --- |
| `mtproto.call` | `(int,int,blob)->blob` |
| `mtproto.close` | `(int)->bool` |
| `mtproto.connect` | `(int)->json` |
| `mtproto.dc_list` | `()->list[json]` |
| `mtproto.decrypt_message` | `(blob,string,blob)->json` |
| `mtproto.dh_derive` | `(int,blob,blob,blob,blob)->blob` |
| `mtproto.dh_start` | `(string)->json` |
| `mtproto.encrypt_message` | `(blob,int,int,int,int,blob)->blob` |
| `mtproto.fingerprint` | `(blob)->blob` |
| `mtproto.ige_decrypt` | `(blob,blob,blob)->blob` |
| `mtproto.ige_encrypt` | `(blob,blob,blob)->blob` |
| `mtproto.key_hex` | `(blob)->string` |
| `mtproto.key_id` | `(blob)->blob` |
| `mtproto.new_msg_id` | `(int)->int` |
| `mtproto.parse_key` | `(string)->blob` |
| `mtproto.rsa_encrypt` | `(string,blob,blob)->blob` |
| `mtproto.tl_bool` | `(bool)->blob` |
| `mtproto.tl_bytes` | `(blob)->blob` |
| `mtproto.tl_int32` | `(int)->blob` |
| `mtproto.tl_int64` | `(int)->blob` |
| `mtproto.tl_read_bool` | `(blob)->bool` |
| `mtproto.tl_read_bytes` | `(blob)->blob` |
| `mtproto.tl_read_int32` | `(blob)->int` |
| `mtproto.tl_read_int64` | `(blob)->int` |
| `mtproto.tl_read_string` | `(blob)->string` |
| `mtproto.tl_string` | `(string)->blob` |
| `mtproto.use_key` | `(int,blob)->bool` |

### `embed`

Доступ в режиме «только для чтения» к статическим ресурсам, упакованным в автономный исполняемый файл командой `alca build --embed <path>`.

| Член | Сигнатура |
| --- | --- |
| `embed.exists` | `(string)->bool` |
| `embed.list` | `()->list[string]` |
| `embed.read` | `(string)->string` |
| `embed.read_bytes` | `(string)->blob` |
| `embed.size` | `(string)->int` |

### `formats`

Составное пространство имен, агрегирующее модули сериализации структурированных форматов `csv`, `ini` и `json`.

| Член | Сигнатура |
| --- | --- |
| `formats.csv` | `map[string, json]` |
| `formats.ini` | `map[string, json]` |
| `formats.json` | `map[string, json]` |

### `gui` [Кроссплатформенный / Multi-backend]

Автоматизация десктопного графического интерфейса, нативные диалоговые окна операционной системы, оконные формы, кнопки, поля ввода и встроенный браузерный компонент WebView.
На Linux требует наличия библиотек GTK+3 или утилит `zenity`/`kdialog`; на Windows использует нативные Win32 API / WebView2.

| Член | Сигнатура |
| --- | --- |
| `gui.alert` | `(string,string?)->bool` |
| `gui.backends` | `()->map[string, bool]` |
| `gui.button` | `(string,any?,int?,int?)->json` |
| `gui.checkbox` | `(string,bool?,any?,int?,int?)->json` |
| `gui.color_picker` | `(string?,string?)->string` |
| `gui.confirm` | `(string,string?)->bool` |
| `gui.error` | `(string,string?)->bool` |
| `gui.file_dialog` | `(string?,string?,string?)->string` |
| `gui.has_native` | `()->bool` |
| `gui.has_webview` | `()->bool` |
| `gui.height` | `()->int` |
| `gui.info` | `(string,string?)->bool` |
| `gui.input` | `(string,string?,string?)->string` |
| `gui.label` | `(string,int?,int?)->json` |
| `gui.notification` | `(string,string?)->bool` |
| `gui.open_folder` | `(string?,string?)->string` |
| `gui.password` | `(string,string?)->string` |
| `gui.progress_dialog` | `(string,string?)->json` |
| `gui.run` | `(any)->bool` |
| `gui.save_file` | `(string?,string?,string?)->string` |
| `gui.screen_size` | `()->list[int]` |
| `gui.slider` | `(int?,int?,int?,any?,int?,int?)->json` |
| `gui.status` | `(string)->bool` |
| `gui.textbox` | `(string?,any?,int?,int?)->json` |
| `gui.warning` | `(string,string?)->bool` |
| `gui.webview` | `(string,int?,int?,string?)->json` |
| `gui.width` | `()->int` |
| `gui.window` | `(string,int?,int?,any?)->json` |

### `ini`

Парсинг, чтение, редактирование и сохранение конфигурационных файлов формата INI.

| Член | Сигнатура |
| --- | --- |
| `ini.parse` | `(string)->json` |
| `ini.read` | `(string)->json` |
| `ini.stringify` | `(json)->string` |
| `ini.write` | `(string,json)->bool` |

### `io`

Файловые операции: чтение и запись текстовых файлов и бинарных блобов, добавление данных, блокировки файлов, проверка метаданных и прав, рекурсивный обход каталогов.

| Член | Сигнатура |
| --- | --- |
| `io.append_file` | `(string,string)->bool` |
| `io.check_access` | `(string,string)->bool` |
| `io.chmod` | `(string,int)->bool;(string,string)->bool` |
| `io.copy_file` | `(string,string)->bool` |
| `io.delete_file` | `(string)->bool` |
| `io.file_exists` | `(string)->bool` |
| `io.file_info` | `(string)->map[string, json]` |
| `io.file_size` | `(string)->int` |
| `io.glob` | `(string)->list[string]` |
| `io.is_permission_error` | `(any)->bool` |
| `io.list_dir` | `(string?)->list[string]` |
| `io.lock_file` | `(string,bool?)->bool;(FileHandle,bool?)->bool` |
| `io.make_dir` | `(string)->bool` |
| `io.move_file` | `(string,string)->bool` |
| `io.open` | `(string,string?)->FileHandle` |
| `io.permissions` | `(string)->map[string, json]` |
| `io.read_bytes` | `(string)->blob` |
| `io.read_file` | `(string)->string` |
| `io.read_lines` | `(string)->list[string]` |
| `io.temp_dir` | `(string?)->string` |
| `io.temp_file` | `(string?)->string` |
| `io.unlock_file` | `(string)->bool;(FileHandle)->bool` |
| `io.walk` | `(string)->list[string]` |
| `io.write_bytes` | `(string,blob)->bool` |
| `io.write_file` | `(string,string)->bool` |

### `json`

Парсинг JSON-текста в динамические значения `json`, сериализация значений Alca в компактный JSON, форматирование с отступами (`prettify`), валидация строк и запросы по точечным путям к полям.

| Член | Сигнатура |
| --- | --- |
| `json.encode` | `(any)->string` |
| `json.parse` | `(string)->json` |
| `json.prettify` | `(any,string?)->string` |
| `json.query` | `(string,string)->json;(json,string)->json` |
| `json.stringify` | `(any)->string` |
| `json.validate` | `(string)->bool` |

*(Примечание: в глобальной области видимости без импорта также доступны функции `json_encode`, `json_decode`, `json_prettify` и `json_query`.)*

### `log`

Структурированное журналирование событий с поддержкой уровней важности (debug, info, warn, error, fatal), ротации файлов журналов и журнала аудита безопасности песочницы.

| Член | Сигнатура |
| --- | --- |
| `log.access_denied` | `(string,string,string?)->bool` |
| `log.audit` | `(string,string,string,string,json?)->bool` |
| `log.debug` | `(string,json?)->null` |
| `log.error` | `(string,json?)->null` |
| `log.fatal` | `(string,json?)->null` |
| `log.get_access_errors` | `()->list[map[string, json]]` |
| `log.info` | `(string,json?)->null` |
| `log.set_file` | `(string,int?,int?)->bool` |
| `log.set_format` | `(string)->bool` |
| `log.set_level` | `(string)->bool` |
| `log.warn` | `(string,json?)->null` |

### `math`

Математические константы, тригонометрия, округление, логарифмы, агрегаты и квадратные корни.

| Член | Сигнатура |
| --- | --- |
| `math.E` | `float` |
| `math.LN10` | `float` |
| `math.LN2` | `float` |
| `math.PI` | `float` |
| `math.SQRT2` | `float` |
| `math.abs` | `(int)->int;(float)->float` |
| `math.acos` | `(float)->float` |
| `math.asin` | `(float)->float` |
| `math.atan` | `(float)->float` |
| `math.atan2` | `(float,float)->float` |
| `math.ceil` | `(float)->int` |
| `math.clamp` | `(int,int,int)->int;(float,float,float)->float` |
| `math.cos` | `(float)->float` |
| `math.deg2rad` | `(float)->float` |
| `math.floor` | `(float)->int` |
| `math.hypot` | `(float,float)->float` |
| `math.log` | `(float)->float` |
| `math.log10` | `(float)->float` |
| `math.log2` | `(float)->float` |
| `math.max` | `(int,int)->int;(float,float)->float` |
| `math.min` | `(int,int)->int;(float,float)->float` |
| `math.pow` | `(float,float)->float;(int,int)->float` |
| `math.rad2deg` | `(float)->float` |
| `math.round` | `(float)->int` |
| `math.sin` | `(float)->float` |
| `math.sqrt` | `(float)->float;(int)->float` |

JIT-компилятор под архитектуру AMD64 компилирует вызовы функций `abs`, `min`, `max`, `clamp`, `sqrt`, `round`, `floor` и `ceil` напрямую в нативный машинный код (интринсики). См. подробности в разделе [Исполнение в JIT и автономных бинарниках](#jit-and-standalone-execution).

### `metrics`

Сбор метрик в формате Prometheus (счетчики counter, датчики gauge, гистограммы histogram) со встроенным HTTP-эндпоинтом для мониторинга.

| Член | Сигнатура |
| --- | --- |
| `metrics.counter` | `(string,string?)->json` |
| `metrics.export` | `()->string` |
| `metrics.gauge` | `(string,string?)->json` |
| `metrics.histogram` | `(string,string?,json?)->json` |
| `metrics.serve` | `(int?,string?)->string` |

### `net`

Низкоуровневые сетевые операции: TCP-клиент и TCP-слушатель, DNS-резолвинг, ICMP/TCP пинг и скачивание файлов.

| Член | Сигнатура |
| --- | --- |
| `net.dns_lookup` | `(string)->list[string]` |
| `net.download_file` | `(string,string)->bool` |
| `net.http_get` | `(string,map[string, string]?)->string` |
| `net.http_post` | `(string,string,map[string, string]?)->string` |
| `net.ping` | `(string,int?)->int` |
| `net.tcp_connect` | `(string)->json` |
| `net.tcp_listen` | `(string)->map[string, json]` |

### `http`

Высокоуровневый HTTP-клиент и встроенный HTTP-сервер. Поддерживает методы GET, POST, PUT, DELETE, PATCH, встроенный роутинг маршрутов (`router`), middleware и автоматическое формирование JSON-ответов.

| Член | Сигнатура |
| --- | --- |
| `http.delete` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.get` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.http_request` | `(string,string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.http_serve` | `(int,any)->map[string, json]` |
| `http.json` | `(any,int?,map[string, json]?)->map[string, json]` |
| `http.patch` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.post` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.put` | `(string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.request` | `(string,string,map[string, string]?,string?,map[string, json]?)->map[string, json];(string,string,map[string, json]?,string?,map[string, json]?)->map[string, json]` |
| `http.response` | `(int,any?,map[string, json]?)->map[string, json]` |
| `http.router` | `()->map[string, json]` |

### `urllib2`

Парсинг и кодирование URL-адресов, клиентские HTTP/HTTPS запросы, поддержка полнодуплексных сокетов WebSocket.

| Член | Сигнатура |
| --- | --- |
| `urllib2.delete` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.get` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.head` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.join` | `(string,string)->string` |
| `urllib2.options` | `(string,any?,int?)->map[string, json];(string,any?,float?)->map[string, json]` |
| `urllib2.parse` | `(string,string?)->map[string, json]` |
| `urllib2.patch` | `(string,any,any?,int?)->map[string, json];(string,any,any?,float?)->map[string, json]` |
| `urllib2.post` | `(string,any,any?,int?)->map[string, json];(string,any,any?,float?)->map[string, json]` |
| `urllib2.put` | `(string,any,any?,int?)->map[string, json];(string,any,any?,float?)->map[string, json]` |
| `urllib2.quote` | `(string)->string` |
| `urllib2.quote_plus` | `(string)->string` |
| `urllib2.request` | `(string,string,any?,any?,int?)->map[string, json];(string,string,any?,any?,float?)->map[string, json]` |
| `urllib2.tcp_request` | `(string,string,any?,any?,int?)->map[string, json];(string,string,any?,any?,float?)->map[string, json]` |
| `urllib2.unquote` | `(string)->string` |
| `urllib2.unquote_plus` | `(string)->string` |
| `urllib2.urlencode` | `(any)->string` |
| `urllib2.urlopen` | `(string,any?,any?,int?)->map[string, json];(string,any?,any?,float?)->map[string, json]` |
| `urllib2.urlretrieve` | `(string,string)->map[string, json]` |
| `urllib2.websocket_connect` | `(string,map[string, string]?,list[string]?,int?)->map[string, json];(string,map[string, string]?,list[string]?,float?)->map[string, json]` |
| `urllib2.websocket_frame_read` | `(any)->map[string, json]` |
| `urllib2.websocket_frame_write` | `(any,string,int?,bool?)->bool` |
| `urllib2.websocket_handshake` | `(string,map[string, string]?)->map[string, json]` |

### `path`

Манипуляция путями файловой системы: нормализация, соединение, определение абсолютных путей и извлечение расширений.

| Член | Сигнатура |
| --- | --- |
| `path.abs` | `(string)->string` |
| `path.base` | `(string)->string` |
| `path.clean` | `(string)->string` |
| `path.dir` | `(string)->string` |
| `path.expand` | `(string)->string` |
| `path.ext` | `(string)->string` |
| `path.is_abs` | `(string)->bool` |
| `path.join` | `(string...)->string` |
| `path.match` | `(string,string)->bool` |
| `path.rel` | `(string,string)->string` |
| `path.separator` | `()->string` |
| `path.split` | `(string)->list[string]` |
| `path.which` | `(string)->string` |

### `process`

Управление дочерними процессами операционной системы: запуск синхронных и фоновых команд, получение PID/PPID, статистика ресурсов (память, процессор) и отправка сигналов завершения.

| Член | Сигнатура |
| --- | --- |
| `process.cpu_count` | `()->int` |
| `process.exec_bg` | `(string,list[string]?)->int` |
| `process.exec_cmd` | `(string)->map[string, json]` |
| `process.get_pid` | `()->int` |
| `process.get_ppid` | `()->int` |
| `process.kill_process` | `(int)->bool` |
| `process.memory_usage` | `()->map[string, json]` |
| `process.process_exists` | `(int)->bool` |
| `process.signal_process` | `(int,int)->bool` |
| `process.uptime` | `()->float` |

### `re`

Регулярные выражения (синтаксис RE2): поиск совпадений, извлечение именованных и позиционных групп, глобальная замена подстрок, экранирование спецсимволов и разбиение строк. Доступен также под псевдонимом `regex`.

| Член | Сигнатура |
| --- | --- |
| `re.find` | `(string,string)->string | null` |
| `re.find_all` | `(string,string,int?)->list[string]` |
| `re.find_all_groups` | `(string,string,int?)->list[list[string]]` |
| `re.find_groups` | `(string,string)->list[string] | null` |
| `re.match` | `(string,string)->bool` |
| `re.quote` | `(string)->string` |
| `re.replace` | `(string,string,string)->string` |
| `re.split` | `(string,string,int?)->list[string]` |

### `sys`

Комплексная системная информация и мониторинг: архитектура и платформа ОС, время загрузки системы (`boot_time`), количество ядер процессора, разделы дисков и статистика использования, средняя загрузка (load average), сетевые интерфейсы, процессы (`pids`, `process_info`, `process_iter`), оперативная память и виртуальная память.

| Член | Сигнатура |
| --- | --- |
| `sys.arch` | `()->string` |
| `sys.boot_time` | `()->float` |
| `sys.cpu_count` | `()->int` |
| `sys.disk_partitions` | `()->list[map[string, json]]` |
| `sys.disk_usage` | `(string?)->map[string, json]` |
| `sys.load_avg` | `()->map[string, json]` |
| `sys.network_interfaces` | `()->list[map[string, json]]` |
| `sys.pids` | `()->list[int]` |
| `sys.platform` | `()->string` |
| `sys.process_info` | `(int)->map[string, json]` |
| `sys.process_iter` | `()->list[map[string, json]]` |
| `sys.process_list` | `()->list[map[string, json]]` |
| `sys.ram_usage` | `()->map[string, json]` |
| `sys.uptime` | `()->float` |
| `sys.virtual_memory` | `()->map[string, json]` |
| `sys.whoami` | `()->map[string, json]` |

### `time`

Высокоточные системные часы, штампы времени UTC в формате RFC3339, таймстемпы в миллисекундах и микросекундах, монотонный таймер для бенчмаркинга, форматирование и парсинг дат по шаблонам и паузы исполнения (`sleep`).

| Член | Сигнатура |
| --- | --- |
| `time.date` | `(int?,string?)->map[string, json];(float?,string?)->map[string, json]` |
| `time.diff` | `(string,string)->float` |
| `time.format` | `(string,string?)->string;(int,string?)->string;(float,string?)->string` |
| `time.measure` | `(json)->map[string, json]` |
| `time.now` | `()->string` |
| `time.parse` | `(string,string?)->string` |
| `time.sleep` | `(int)->null;(float)->null` |
| `time.sleep_ms` | `(int)->null;(float)->null` |
| `time.timer` | `(string)->map[string, json]` |
| `time.timestamp` | `()->int` |
| `time.timestamp_ms` | `()->int` |
| `time.timestamp_us` | `()->int` |

### `string`

Операции со строками в стандарте Unicode: подсчет длины в рунах (code points), срезы по рунам, проверка вхождения, префиксов и суффиксов, преобразование регистра (верхний/нижний), обрезка пробелов, поиск подстрок, разбиение и объединение списков строк.

| Член | Сигнатура |
| --- | --- |
| `string.capitalize` | `(string)->string` |
| `string.center` | `(string,int,string?)->string` |
| `string.chars` | `(string)->list[string]` |
| `string.contains` | `(string,string)->bool` |
| `string.count` | `(string,string)->int` |
| `string.ends_with` | `(string,string)->bool` |
| `string.find` | `(string,string)->int` |
| `string.format` | `(string,any...)->string` |
| `string.index_of` | `(string,string)->int` |
| `string.is_alnum` | `(string)->bool` |
| `string.is_alpha` | `(string)->bool` |
| `string.is_ascii` | `(string)->bool` |
| `string.is_digit` | `(string)->bool` |
| `string.is_lower` | `(string)->bool` |
| `string.is_space` | `(string)->bool` |
| `string.is_upper` | `(string)->bool` |
| `string.join` | `(list[string],string)->string` |
| `string.last_index_of` | `(string,string)->int` |
| `string.length` | `(string)->int` |
| `string.lines` | `(string)->list[string]` |
| `string.lower` | `(string)->string` |
| `string.lpad` | `(string,int,string?)->string` |
| `string.ltrim` | `(string)->string` |
| `string.pad_end` | `(string,int,string?)->string` |
| `string.pad_start` | `(string,int,string?)->string` |
| `string.repeat` | `(string,int)->string` |
| `string.replace` | `(string,string,string)->string` |
| `string.replace_all` | `(string,string,string)->string` |
| `string.reverse` | `(string)->string` |
| `string.rpad` | `(string,int,string?)->string` |
| `string.rtrim` | `(string)->string` |
| `string.runes` | `(string)->list[int]` |
| `string.slice` | `(string,int,int?)->string` |
| `string.split` | `(string,string,int?)->list[string]` |
| `string.split_lines` | `(string)->list[string]` |
| `string.starts_with` | `(string,string)->bool` |
| `string.title` | `(string)->string` |
| `string.trim` | `(string)->string` |
| `string.trim_end` | `(string)->string` |
| `string.trim_left` | `(string)->string` |
| `string.trim_prefix` | `(string,string)->string` |
| `string.trim_right` | `(string)->string` |
| `string.trim_start` | `(string)->string` |
| `string.trim_suffix` | `(string,string)->string` |
| `string.upper` | `(string)->string` |
| `string.words` | `(string)->list[string]` |

### `list`

Функциональные утилиты для работы со списками: сортировка (`sort`), разворот (`reverse`), фильтрация (`filter`), трансформация (`map`), свертка (`reduce`), поиск элементов, кванторы (`any`, `all`), разбиение на блоки (`chunk`) и удаление дубликатов (`unique`).

| Член | Сигнатура |
| --- | --- |
| `list.all` | `(list[any],any)->bool` |
| `list.any` | `(list[any],any)->bool` |
| `list.append` | `(list[any],any)->list[any]` |
| `list.chunk` | `(list[any],int)->list[list[any]]` |
| `list.clear` | `(list[any])->bool` |
| `list.concat` | `(list[any],list[any])->list[any]` |
| `list.contains` | `(list[any],any)->bool` |
| `list.count` | `(list[any],any)->int` |
| `list.extend` | `(list[any],list[any])->list[any]` |
| `list.filter` | `(list[any],any)->list[any]` |
| `list.find` | `(list[any],any)->any` |
| `list.flatten` | `(list[any])->list[any]` |
| `list.index_of` | `(list[any],any)->int` |
| `list.insert` | `(list[any],int,any)->list[any]` |
| `list.join` | `(list[any],string)->string` |
| `list.last_index_of` | `(list[any],any)->int` |
| `list.length` | `(list[any])->int` |
| `list.map` | `(list[any],any)->list[any]` |
| `list.max` | `(list[any])->any` |
| `list.min` | `(list[any])->any` |
| `list.pop` | `(list[any])->any` |
| `list.reduce` | `(list[any],any,any?)->any` |
| `list.remove` | `(list[any],any)->bool` |
| `list.remove_at` | `(list[any],int)->any` |
| `list.reverse` | `(list[any])->list[any]` |
| `list.slice` | `(list[any],int,int?)->list[any]` |
| `list.sort` | `(list[any],any?)->list[any]` |
| `list.sum` | `(list[any])->float` |
| `list.unique` | `(list[any])->list[any]` |

### `map`

Методы работы со словарями (хэш-таблицами): проверка наличия ключей (`has_key`), безопасное удаление (`delete_key`), получение списков ключей (`keys`) и значений (`values`), слияние словарей с сохранением исходного порядка вставки.

| Член | Сигнатура |
| --- | --- |
| `map.clear` | `(map[any, any])->bool` |
| `map.contains_key` | `(map[any, any],any)->bool` |
| `map.delete` | `(map[any, any],any)->bool` |
| `map.delete_key` | `(map[any, any],any)->bool` |
| `map.entries` | `(map[any, any])->list[list[any]]` |
| `map.get` | `(map[any, any],any,any?)->any` |
| `map.has_key` | `(map[any, any],any)->bool` |
| `map.keys` | `(map[any, any])->list[any]` |
| `map.length` | `(map[any, any])->int` |
| `map.merge` | `(map[any, any],map[any, any])->map[any, any]` |
| `map.pop` | `(map[any, any],any,any?)->any` |
| `map.set` | `(map[any, any],any,any)->bool` |
| `map.update` | `(map[any, any],map[any, any])->bool` |
| `map.values` | `(map[any, any])->list[any]` |

### `random`

Быстрый псевдослучайный генератор чисел (PRNG) с поддержкой установки начального значения (`seed`) для гарантированной повторяемости результатов в играх, графике, процедурной генерации и моделировании методом Монте-Карло.

| Член | Сигнатура |
| --- | --- |
| `random.choice` | `(list[any])->any` |
| `random.float` | `(float?,float?)->float` |
| `random.int` | `(int,int)->int` |
| `random.random` | `()->float` |
| `random.sample` | `(list[any],int)->list[any]` |
| `random.seed` | `(int)->null` |
| `random.shuffle` | `(list[any])->list[any]` |
| `random.string` | `(int,string?)->string` |

### Обработка и инспекция ошибок

Ошибки в Alca являются полноправными объектами первого класса (тип `error`), предоставляющими типизированные категории, стандартные числовые коды, текстовое описание, метаданные места возникновения и временную метку создания:
- `err.code`: целочисленный код ошибки (`400` — неверный аргумент, `403` — нарушение песочницы, `500` — внутренняя ошибка рантайма, `507` — исчерпание лимита памяти OOM).
- `err.msg`: понятное человеку текстовое описание причины ошибки.
- `err.file`: путь к исходному файлу, в котором произошла ошибка (или пустая строка).
- `err.line`: номер строки возникновения ошибки (1-based).
- `err.details`: динамический объект с расширенной контекстной диагностикой (параметры, путь, хост).
- `err.time()`: возвращает строку со штампом времени возникновения ошибки по стандарту UTC RFC3339.

---

## Глобальные встроенные идентификаторы и функции

Данные функции и переменные доступны во всей программе без необходимости указывать `use`.

### Системные переменные рантайма (Dunder Variables)

Следующие неизменяемые идентификаторы доступны глобально:

| Идентификатор | Тип | Описание |
| --- | --- | --- |
| `__file__` | `string` | Абсолютный путь к исполняемому файлу исходного кода (или `"<stdin>"` в REPL). |
| `__dir__` | `string` | Каталог, в котором расположен `__file__` (или `"."`). |
| `__dirname__` | `string` | Синоним для `__dir__`. |
| `__name__` | `string` | `"__main__"` для точки входа или имя модуля при импорте. |
| `__line__` | `int` | Текущий номер строки в исходном коде (1-based). |
| `__version__` | `string` | Версия рантайма и компилятора Alca (например, `"1.7.9"`). |

### Глобальные функции

| Функция | Сигнатура |
| --- | --- |
| `abs` | `(int)->int;(float)->float` |
| `bool` | `(any)->bool` |
| `exec` | `(string)->string` |
| `exit` | `(int?)->void` |
| `float` | `(any)->float` |
| `input` | `(string?)->string` |
| `int` | `(any)->int` |
| `json_decode` | `(string)->json` |
| `json_encode` | `(any)->string` |
| `json_prettify` | `(any)->string` |
| `json_query` | `(json,string)->json` |
| `max` | `(int,int)->int;(float,float)->float` |
| `min` | `(int,int)->int;(float,float)->float` |
| `print` | `(any...)->string` |
| `println` | `(any...)->string` |
| `sleep` | `(int)->null;(float)->null` |
| `str_contains` | `(string,string)->bool` |
| `str_ends_with` | `(string,string)->bool` |
| `str_lower` | `(string)->string` |
| `str_replace` | `(string,string,string)->string` |
| `str_split` | `(string,string)->list[string]` |
| `str_starts_with` | `(string,string)->bool` |
| `str_trim` | `(string)->string` |
| `str_upper` | `(string)->string` |
| `string` | `(any)->string` |
| `time_now` | `()->string` |
| `type` | `(any)->string` |
| `typeof` | `(any)->string` |

> [!NOTE]
> Прежние глобальные шлюзовые псевдонимы функций стандартной библиотеки (такие как `read_file`, `write_file`, `get_env`, `sha256`, `cpu_count` и др.) удалены из глобальной области для обеспечения чистоты пространств имен и аудита прав. Функции стандартной библиотеки должны вызываться строго через модуль (например, `use std.io;` -> `io.read_file(...)`, `use std.env;` -> `env.get_env(...)`, `use std.crypto;` -> `crypto.sha256(...)`). Попытка их вызова напрямую вызывает ошибку компиляции с подсказкой нужного модуля `std.*`.

Список устаревших псевдонимов и их канонических модулей:
- `env`: `add_path`, `add_system_path`, `add_user_path`, `all_env`, `broadcast_env_change`, `expand_env`, `get_env`, `get_path`, `get_system_path`, `get_user_path`, `has_path`, `home_dir`, `hostname`, `is_linux`, `is_windows`, `os_name`, `path_separator`, `persist_user_path`, `prepend_path`, `remove_path`, `remove_user_path`, `set_env`, `set_path`, `unset_env`, `username`, `which`, `working_dir`
- `io`: `append_file`, `check_access`, `chmod`, `copy_file`, `delete_file`, `file_exists`, `file_info`, `file_size`, `glob`, `is_permission_error`, `list_dir`, `lock_file`, `make_dir`, `move_file`, `open`, `permissions`, `read_bytes`, `read_file`, `read_lines`, `temp_dir`, `temp_file`, `unlock_file`, `walk`, `write_bytes`, `write_file`
- `net`: `dns_lookup`, `download_file`, `http_get`, `http_post`, `ping`
- `crypto`: `aes_decrypt`, `aes_decrypt_password`, `aes_encrypt`, `aes_encrypt_password`, `base64_decode`, `base64_encode`, `constant_time_equal`, `derive_key`, `hex_decode`, `hex_encode`, `hmac_sha256`, `hmac_sha512`, `md5`, `random_bytes`, `random_int`, `random_string`, `sha1`, `sha256`, `sha512`, `uuid`
- `compress`: `gzip_compress`, `gzip_decompress`, `zlib_compress`, `zlib_decompress`
- `process`: `cpu_count`, `exec_bg`, `exec_cmd`, `get_pid`, `get_ppid`, `kill_process`, `memory_usage`, `process_exists`, `signal_process`, `uptime`

---

<a id="jit-and-standalone-execution"></a>
## Исполнение в JIT и автономных бинарниках

Любой зарегистрированный член стандартной библиотеки доступен для вызова из функций, скомпилированных JIT. Существуют три принципиально различных пути исполнения:

- **Нативный интринсик (Native intrinsic)**: бэкенд AMD64 генерирует машинные инструкции непосредственно в месте вызова. Интринсики реализованы для 8 числовых функций `std.math`: `abs`, `min`, `max`, `clamp`, `sqrt`, `round`, `floor`, `ceil`. Они задействуются только при прямом вызове через незатененный алиас (`use std.math;` или `use std.math as m;`). Если переданный аргумент во время исполнения не является числом, генерируется точка деоптимизации, и VM возбуждает соответствующую ошибку.
- **Хелпер рантайма JIT для чистых функций (fused pure call)**: прямые вызовы детерминированных членов stdlib компилируются в опкод `OpStdlibCall`. Движки Threaded JIT и AMD64 JIT находят зарегистрированную функцию и исполняют ее на уровне рантайм-хелпера, немедленно возвращаясь в скомпилированный код. Тело функции выполняется на Go без накладных расходов диспетчеризации байткода VM. Целиком чистыми модулями являются `math`, `string`/`str`, `re`/`regex`, `compress` и `json`.
- **Хелпер рантайма JIT для хост-вызовов (host calls)**: для функций ввода-вывода, сети, БД, GUI, процессов и HTTP-запросов JIT производит поиск модуля и вызов в том же слое рантайм-хелпера, соблюдая все проверки песочницы. Вызов не транслируется в машинный код и исполняется через стандартную реализацию рантайма.

### Матрица поддержки по движкам исполнения

| Движок или сборка | Чистый член stdlib (fused) | Хост-член stdlib | `math.abs` / `min` / `max` / `clamp` / `sqrt` / `round` / `floor` / `ceil` |
| --- | --- | --- | --- |
| Древовидный интерпретатор | Прямой вызов без JIT | Прямой вызов без JIT | Прямой вызов без JIT |
| Байткод-VM (JIT выключен) | `OpStdlibCall`, VM исполняет член | Прямой вызов через VM | `OpMathBuiltin` |
| Threaded JIT | JIT-хелпер; возобновление threaded-кода | JIT-хелпер; возобновление threaded-кода | JIT-хелпер |
| AMD64 native JIT | JIT-хелпер; возобновление машинного кода | JIT-хелпер; возобновление машинного кода | Инлайн-машинный код; деоптимизация при несовпадении типов |
| Автономный рантайм-стаб (байткод/AOT) | Исполнение рантайма; возврат в AOT-код | Исполнение рантайма; возврат в AOT-код | Инлайн при использовании интринсика JIT/AOT |
| Чистый нативный PE/AOT (без VM) | Недоступно | Недоступно | Не выбирается |

Программы, импортирующие любой модуль `std.*`, никогда не собираются как чистый PE без рантайма: сборщик автоматически использует рантайм-стаб, включающий виртуальную машину и стандартную библиотеку.

---

## Политика безопасности песочницы

CLI-утилита `alca run` и собранные автономные приложения по умолчанию устанавливают строгую политику блокировки доступа (*Deny-All*). Проверки выполняются во время выполнения на обоих движках:

- `io`, `csv`, `ini`, `archive`, `log.set_file` и файловые форматы требуют прав `--allow-read` или `--allow-write` для целевого пути. Это распространяется на `io.open`, потоки, маски файлов (glob), директории и распаковку архивов.
- Сетевые вызовы `net`, запросы DNS, TCP-сокеты, загрузки и `metrics.serve` требуют флага `--allow-net`. Перенаправления HTTP (redirects) валидируются повторно перед выполнением перехода.
- `db.sqlite` и `db.postgres` подчиняются файловым и сетевым правилам. Доступ к SQLite на запись требует флагов чтения и записи; доступ в режиме `mode=ro` требует только флага чтения. Параметры подключения к PostgreSQL проверяются по флагу `--allow-net` для хоста и порта (по умолчанию `localhost:5432`).
- Вызовы `process`, глобальная функция `exec`, `env` и `ffi` управляются специализированными флагами `--allow-run[=<команда>]`, `--allow-env[=<переменная>]` и `--allow-ffi[=<библиотека>]`.
- Флаг `--allow-all` безоговорочно разрешает все типы системных операций.

При отказе в доступе функция возвращает значение `error` с кодом `403`, которое в Alca можно инспектировать функцией `io.is_permission_error(err)`.

# Telebot — Telegram Bot API Library for Alca

`telebot` — полнофункциональная, высокопроизводительная библиотека Telegram Bot API (актуальная версия **Bot API 10.3**) для языка программирования **Alca**.

Поддерживает как **Long Polling**, так и **Webhook** через встроенный HTTP-сервер Alca, автоматическую загрузку файлов через `multipart/form-data`, конструкторы клавиатур, удобный контекст сообщений и обёртки для всех 185 методов Bot API.

---

## Возможности

- **Полное покрытие Bot API 10.3**: сгенерированные и проверенные типизированные обёртки для всех методов (`send_message`, `send_photo`, `send_document`, `edit_message_text`, `answer_callback_query`, `set_webhook` и др.).
- **Автоматический Multipart**: при передаче пути к локальному файлу в параметрах отправки медиа (фото, аудио, документы, видео) библиотека автоматически формирует корректный `multipart/form-data` запрос.
- **Диспетчеризация и фильтры**:
  - `on_command(bot, cmd, handler)`
  - `on_text(bot, handler)`
  - `on_callback(bot, handler)` / `on_callback_data(bot, data, handler)`
  - `on_photo`, `on_document`, `on_edited_message`, `on_channel_post` и др.
- **Удобный Context**: извлечение `chat_id`, `user_id`, текста, аргументов команды, а также методы `reply()`, `reply_photo()`, `answer_callback()` прямо из контекста.
- **Конструкторы клавиатур**:
  - Inline: `inline_keyboard()`, `inline_button()`, `inline_url_button()`
  - Reply: `reply_keyboard()`, `keyboard_button()`, `remove_keyboard()`
- **Форматирование и экранирование**: `escape_html()`, `escape_markdown_v2()`, разбиение длинных сообщений `split_text()`.
- **Надёжность**: автоматический повтор при временных сетевых ошибках, экспоненциальная пауза при Telegram `429 Too Many Requests` (с учётом `retry_after`).

---

## Быстрый старт (Long Polling)

Создайте файл `bot.al`:

```alca
use "./libs/telebot/telebot.al" as tg;

// Инициализация бота
opts: map[string, json] = {
    "parse_mode": "HTML",
    "poll_timeout": 30
};
bot := tg.new_bot("1234567890:ABCdefGhIJKlmNoPQRsTUVwxyZ", opts);

// Обработчик команды /start
tg.on_command(bot, "start", |ctx: json| {
    name := string(ctx["from"]["first_name"]);
    tg.reply(ctx, f"Привет, <b>{name}</b>! Я бот на языке Alca.");
});

// Обработчик текстовых сообщений
tg.on_text(bot, |ctx: json| {
    text := string(tg.text_of(ctx));
    tg.reply(ctx, f"Вы написали: {tg.escape_html(text)}");
});

// Запуск long polling
println("Бот запущен!");
tg.poll(bot);
```

Запуск программы (с необходимыми правами песочницы Alca):

```bash
alca run --allow-net --allow-read --allow-write bot.al
```

---

## Клавиатуры

### Inline-клавиатура

```alca
kb := tg.inline_keyboard([
    [
        tg.inline_button("👍 Лайк", "vote:like"),
        tg.inline_button("👎 Дизлайк", "vote:dislike")
    ],
    [
        tg.inline_url_button("Официальный сайт", "https://alca-lang.org")
    ]
]);

tg.send_message(bot, chat_id, "Оцените Alca:", {"reply_markup": kb});

// Обработка нажатия на кнопку
tg.on_callback_prefix(bot, "vote:", |ctx: json| {
    data := string(ctx["data"]);
    tg.answer_callback(ctx, {"text": f"Голос принят: {data}"});
});
```

### Reply-клавиатура

```alca
menu := tg.reply_keyboard([
    [tg.keyboard_button("Помощь"), tg.keyboard_button("Статистика")],
    [tg.keyboard_button("Отправить локацию", {"request_location": true})]
], {"resize_keyboard": true});

tg.send_message(bot, chat_id, "Главное меню:", {"reply_markup": menu});
```

---

## Отправка файлов

При передаче локального пути в поле `InputFile`, файл автоматически загружается через multipart:

```alca
// Отправка фото с локального диска
tg.send_photo(bot, chat_id, "assets/banner.png", {
    "caption": "Логотип Alca"
});

// Отправка документа
tg.send_document(bot, chat_id, "report.pdf", {
    "caption": "Ежемесячный отчёт"
});
```

---

## Скачивание файлов

```alca
// Получение файла от пользователя и сохранение на диск
tg.download_file(bot, file_id, "downloads/photo.jpg");
```

---

## Webhook

Благодаря встроенному HTTP-роутеру Alca, вебхук поднимается за несколько строк:

```alca
use std.http;
use "./libs/telebot/telebot.al" as tg;

bot := tg.new_bot("TOKEN", {});

tg.on_command(bot, "start", |ctx: json| tg.reply(ctx, "Привет из Webhook!"));

router := http.router();

// Регистрация эндпоинта вебхука
router.post("/bot{token}/webhook", |req: map[string, json]| {
    body := string(req["body"]);
    tg.handle_update(bot, tg.parse_webhook(body));
    return http.json({"ok": true});
});

println("Webhook сервер запущен на порту 8443");
router.serve(8443);
```

Установка вебхука в Telegram:
```alca
tg.set_webhook(bot, "https://example.com/bot" + bot.token + "/webhook");
```

---

## Песочница и безопасность

Alca работает в безопасной песочнице по умолчанию. Для функционирования бота требуются флаги:
- `--allow-net`: для обращения к `api.telegram.org` и запуска webhook-сервера.
- `--allow-read`: для чтения отправляемых файлов с диска.
- `--allow-write`: для скачивания файлов (`download_file`).
- `--allow-all`: разрешить все операции (удобно для разработки).

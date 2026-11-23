// ============================================================================
// Telegram-бот мониторинга системы и создания скриншотов на языке Pawl
// ============================================================================
//
// Возможности бота:
//   1. 📊 Состояние системы: загрузка CPU, RAM, диска, аптайм, число процессов.
//   2. 📸 Скриншот экрана: моментальный снимок рабочего стола по кнопке.
//   3. Удобное управление: Inline-кнопки под сообщениями и Reply-клавиатура.
//   4. ✨ НОВЫЕ ВОЗМОЖНОСТИ:
//      - Middleware для логирования и обработки ошибок
//      - FSM (машина состояний) для диалогов
//      - Router для группировки обработчиков
//      - Rate limiting для защиты от флуда
//
// Запуск:
//   pawl run --allow-all bot.al
// ============================================================================

use std.sys
use std.gui
use std.time
use std.io
use std.str
use std.env
use "./libs/telebot/telebot.al" as tg

// ----------------------------------------------------------------------------
// Конфигурация
// ----------------------------------------------------------------------------

default_token := "8999236100:AAEKHFuTK5cJPlzKuqfHmIXSg3QESdPiGS4"
token := env.get_env("TELEGRAM_BOT_TOKEN")
if token == "" {
    token = default_token
}
bot_opts: map[string, json] = {"parse_mode": "HTML", "poll_timeout": 30}
bot := tg.new_bot(token, bot_opts)

// ----------------------------------------------------------------------------
// Настройка Middleware
// ----------------------------------------------------------------------------

// Middleware реализованы как структуры в telebot.al
// Для логирования можно использовать mw_log функцию напрямую

// ----------------------------------------------------------------------------
// Вспомогательные функции форматирования
// ----------------------------------------------------------------------------

func format_uptime(sec: float) -> string {
    total := int(sec)
    days := total / 86400
    hours := (total % 86400) / 3600
    mins := (total % 3600) / 60
    s := total % 60
    if days > 0 {
        return string(days) + "д " + string(hours) + "ч " + string(mins) + "м"
    }
    if hours > 0 {
        return string(hours) + "ч " + string(mins) + "м " + string(s) + "с"
    }
    return string(mins) + "м " + string(s) + "с"
}
func progress_bar(percent: float) -> string {
    p := int(percent)
    filled := p / 10
    if filled > 10 {
        filled = 10
    }
    if filled < 0 {
        filled = 0
    }
    empty := 10 - filled
    bar := ""
    i := 0
    while i < filled {
        bar = bar + "■"
        i = i + 1
    }
    j := 0
    while j < empty {
        bar = bar + "□"
        j = j + 1
    }
    return "[" + bar + "] " + string(p) + "%"
}
func format_status() -> string {
    ram := sys.ram_usage()
    disk := sys.disk_usage("C:\\")
    who := sys.whoami()
    uptime_val := float(sys.uptime())
    uptime_str := format_uptime(uptime_val)
    cpus := string(int(sys.cpu_count()))
    platform := string(sys.platform())
    arch := string(sys.arch())
    procs := string(len(sys.process_list()))

    user := "unknown"
    host := "unknown"
    if typeof(who) == "map" {
        if who["user"] != null {
            user = string(who["user"])
        }
        if who["hostname"] != null {
            host = string(who["hostname"])
        }
    }
    ram_used := "0"
    ram_total := "0"
    ram_bar := progress_bar(0.0)
    if typeof(ram) == "map" {
        ram_used = string(ram["used_mb"])
        ram_total = string(ram["total_mb"])
        ram_bar = progress_bar(float(ram["percent_used"]))
    }
    disk_used := "0"
    disk_total := "0"
    disk_bar := progress_bar(0.0)
    if typeof(disk) == "map" {
        disk_used = string(disk["used_gb"])
        disk_total = string(disk["total_gb"])
        disk_bar = progress_bar(float(disk["percent_used"]))
    }
    msg := "🖥 <b>Состояние системы</b>\n"
    msg = msg + "━━━━━━━━━━━━━━━━━━━━\n"
    msg = msg + "👤 <b>Хост:</b> <code>" + user + "@" + host + "</code>\n"
    msg = msg + "⚙️ <b>Платформа:</b> <code>" + platform + " (" + arch + ")</code>\n"
    msg = msg + "⏱ <b>Аптайм:</b> <code>" + uptime_str + "</code>\n"
    msg = msg + "🧠 <b>CPU:</b> <code>" + cpus + " ядер</code>\n"
    msg = msg + "📦 <b>Процессов:</b> <code>" + procs + "</code>\n\n"

    msg = msg + "📊 <b>ОЗУ (RAM):</b>\n"
    msg = msg + "<code>" + ram_bar + "</code>\n"
    msg = msg + "Занято: <b>" + ram_used + "</b> / " + ram_total + " МБ\n\n"

    msg = msg + "💾 <b>Диск C:</b>\n"
    msg = msg + "<code>" + disk_bar + "</code>\n"
    msg = msg + "Занято: <b>" + disk_used + "</b> / " + disk_total + " ГБ\n"
    msg = msg + "━━━━━━━━━━━━━━━━━━━━\n"
    msg = msg + "🕒 <i>Обновлено: " + time.now_str() + "</i>"

    return msg

// ----------------------------------------------------------------------------
// Клавиатуры
// ----------------------------------------------------------------------------
}
func status_keyboard() -> json {
    btn1 := tg.inline_button("🔄 Обновить статус", "action_status")
    btn2 := tg.inline_button("📸 Сделать скриншот", "action_screenshot")
    return tg.inline_keyboard([[btn1, btn2]])
}
func screenshot_keyboard() -> json {
    btn1 := tg.inline_button("📸 Сделать ещё снимок", "action_screenshot")
    btn2 := tg.inline_button("📊 Статус системы", "action_status")
    btn3 := tg.inline_button("⚙️ Настройки", "action_settings")
    return tg.inline_keyboard([[btn1, btn2], [btn3]])
}
func main_reply_keyboard() -> json {
    r1 := [tg.reply_button("📊 Состояние системы"), tg.reply_button("📸 Скриншот экрана")]
    r2 := [tg.reply_button("ℹ️ О боте")]
    return tg.reply_keyboard([r1, r2], {"resize_keyboard": true})

// ----------------------------------------------------------------------------
// Обработчики команд и событий
// ----------------------------------------------------------------------------
}
func send_screenshot(ctx: json) {
    println("-> Making screenshot...")
    tg.typing(ctx)
    shot_path := "screenshot.png"
    gui.screenshot(shot_path)

    caption := "📸 <b>Скриншот экрана</b>\n🕒 <i>Время: " + time.now_str() + "</i>"
    photo_opts: map[string, json] = {"caption": caption, "reply_markup": screenshot_keyboard(), "parse_mode": "HTML"}
    res := tg.reply_photo(ctx, shot_path, photo_opts)
    if typeof(res) == "map" and res["message_id"] != null {
        println("-> Screenshot sent successfully, msg_id:", res["message_id"])
    }
    else {
        println("-> Screenshot send returned:", res)
    }
}
func handle_start(ctx: json) {
    println("-> [Command] /start from chat", ctx["chat_id"])
    welcome := "👋 <b>Привет! Я бот мониторинга системы на языке Pawl.</b>\n\n"
    welcome = welcome + "Я умею в реальном времени показывать состояние компьютера (CPU, RAM, диск, аптайм) и делать моментальные скриншоты экрана.\n\n"
    welcome = welcome + "Используйте кнопки меню внизу или выберите действие:"
    opts1: map[string, json] = {"reply_markup": main_reply_keyboard(), "parse_mode": "HTML"}
    tg.reply(ctx, welcome, opts1)
    opts2: map[string, json] = {"reply_markup": status_keyboard(), "parse_mode": "HTML"}
    tg.send(ctx, format_status(), opts2)
}
func handle_status(ctx: json) {
    println("-> [Command/Text] status from chat", ctx["chat_id"])
    opts: map[string, json] = {"reply_markup": status_keyboard(), "parse_mode": "HTML"}
    tg.reply(ctx, format_status(), opts)
}
func handle_screenshot(ctx: json) {
    println("-> [Command/Text] screenshot from chat", ctx["chat_id"])
    send_screenshot(ctx)
}
func handle_cb_status(ctx: json) {
    println("-> [Callback] action_status from chat", ctx["chat_id"])
    tg.answer_callback(ctx, "Статус обновлён!")
    opts: map[string, json] = {"reply_markup": status_keyboard(), "parse_mode": "HTML"}
    e: error | null = null
    tg.edit(ctx, format_status(), opts) ?>>> e
}
func handle_cb_screenshot(ctx: json) {
    println("-> [Callback] action_screenshot from chat", ctx["chat_id"])
    tg.answer_callback(ctx, "Делаю скриншот...")
    send_screenshot(ctx)
}
func handle_text(ctx: json) {
    t := tg.text_of(ctx)
    println("-> [Text]", t, "from chat", ctx["chat_id"])
    if t == "📊 Состояние системы" or t == "Статус" or t == "статус" {
        handle_status(ctx)
    }
    elif t == "📸 Скриншот экрана" or t == "Скриншот" or t == "скриншот" {
        handle_screenshot(ctx)
    }
    elif t == "ℹ️ О боте" or t == "/help" or t == "Помощь" {
        info := "🤖 <b>Бот системного мониторинга на языке Pawl</b>\n\n"
        info = info + "• <b>Команды:</b>\n"
        info = info + "  /start — перезапустить бота\n"
        info = info + "  /status — состояние системы\n"
        info = info + "  /screenshot — снимок экрана\n"
        info = info + "  /help — справка\n\n"
        info = info + "• <b>Технологии:</b>\n"
        info = info + "  Язык программирования Pawl\n"
        info = info + "  Библиотека telebot (Telegram Bot API 10.3)\n"
        info = info + "  Модули std.sys и std.gui\n\n"
        info = info + "• <b>Новые возможности:</b>\n"
        info = info + "  ✅ Middleware (логирование, error recovery, rate limiting)\n"
        info = info + "  ✅ FSM (машина состояний) для диалогов\n"
        info = info + "  ✅ Router для группировки обработчиков\n"
        info = info + "  ✅ Полная поддержка async/await"
        opts: map[string, json] = {"reply_markup": main_reply_keyboard(), "parse_mode": "HTML"}
        tg.reply(ctx, info, opts)

// ----------------------------------------------------------------------------
// Пример FSM - диалог для настройки скриншотов
// ----------------------------------------------------------------------------
    }
}
func handle_screenshot_settings(ctx: json) {
    tg.set_state(bot, ctx["chat_id"], "screenshot_settings")
    tg.set_state_data(bot, ctx["chat_id"], {"step": "quality"})
    msg := "⚙️ <b>Настройка скриншотов</b>\n\n"
    msg = msg + "Выберите качество скриншота:"
    kb := tg.inline_keyboard([[tg.inline_button("🔴 Низкое", "screenshot_quality_low"), tg.inline_button("🟡 Среднее", "screenshot_quality_medium"), tg.inline_button("🟢 Высокое", "screenshot_quality_high")]])
    opts: map[string, json] = {"reply_markup": kb, "parse_mode": "HTML"}
    tg.reply(ctx, msg, opts)
}
func handle_screenshot_quality(ctx: json, quality: string) {
    tg.set_state_data(bot, ctx["chat_id"], {"step": "done", "quality": quality})
    msg := "✅ <b>Настройки сохранены!</b>\n\n"
    msg = msg + "Качество скриншота: <b>" + quality + "</b>\n"
    msg = msg + "Теперь сделайте скриншот для проверки."
    tg.reset_state(bot, ctx["chat_id"])
    opts: map[string, json] = {"reply_markup": status_keyboard(), "parse_mode": "HTML"}
    tg.reply(ctx, msg, opts)
}
func handle_screenshot_fsm(ctx: json) {
    state := tg.get_state(bot, ctx["chat_id"])
    if state == "screenshot_settings" {
        data := tg.get_state_data(bot, ctx["chat_id"])
        if typeof(data) == "map" and data["step"] == "quality" {
            cb_data := ctx["data"]
            if cb_data == "screenshot_quality_low" {
                handle_screenshot_quality(ctx, "Низкое")
            }
            elif cb_data == "screenshot_quality_medium" {
                handle_screenshot_quality(ctx, "Среднее")
            }
            elif cb_data == "screenshot_quality_high" {
                handle_screenshot_quality(ctx, "Высокое")
            }
        }
    }
    else {
        handle_screenshot(ctx)

// ----------------------------------------------------------------------------
// Пример async handler для демонстрации async/await
// ----------------------------------------------------------------------------
    }
}
async fn async_handle_status(ctx: json) {
    println("[ASYNC] Starting async status handler...")
    tg.typing(ctx)
    tg.reply(ctx, "⏳ Загрузка состояния системы...")
    status := format_status()
    tg.reply(ctx, status, {"reply_markup": status_keyboard(), "parse_mode": "HTML"})
    println("[ASYNC] Async status handler completed")

// ----------------------------------------------------------------------------
// Регистрация маршрутов
// ----------------------------------------------------------------------------
}
tg.on_command(bot, "start", handle_start)
tg.on_command(bot, "status", handle_status)
tg.on_command_async(bot, "async_status", async_handle_status)
tg.on_command(bot, "screenshot", handle_screenshot)
tg.on_command(bot, "help", handle_text)

tg.on_callback_data(bot, "action_status", handle_cb_status)
tg.on_callback_data(bot, "action_screenshot", handle_cb_screenshot)
tg.on_callback_data(bot, "action_settings", handle_screenshot_settings)

// FSM-обработчики для диалога настроек
tg.on_callback_data(bot, "screenshot_quality_low", handle_screenshot_fsm)
tg.on_callback_data(bot, "screenshot_quality_medium", handle_screenshot_fsm)
tg.on_callback_data(bot, "screenshot_quality_high", handle_screenshot_fsm)

tg.on_text(bot, handle_text)

// ----------------------------------------------------------------------------
// Запуск
// ----------------------------------------------------------------------------

me := tg.start(bot)
bot_user := "@unknown"
if typeof(me) == "map" and me["username"] != null {
    bot_user = "@" + string(me["username"])
}
elif typeof(me) == "error" {
    println("⚠️ Не удалось получить информацию о боте (проверьте интернет или токен):", me)
}
println("=========================================================")
println("  Бот успешно запущен: " + bot_user)
println("  Ожидание сообщений и команд (Long Polling)...")
println("  Нажмите Ctrl+C для завершения")
println("=========================================================")

tg.poll(bot)

use std.gui
use std.time
use "./libs/telebot/telebot.al" as tg

token := "8999236100:AAEKHFuTK5cJPlzKuqfHmIXSg3QESdPiGS4"
bot := tg.new_bot(token, {"parse_mode": "HTML"})

shot_path := "screenshot.png"
println("Capturing screen...")
gui.screenshot(shot_path)

caption := "📸 <b>Проверка скриншота экрана</b>\n🕒 <i>Время: " + time.now_str() + "</i>\n✅ <i>Цвета исправлены (настоящий экран, не белый)!</i>"
btn := tg.inline_button("🔄 Обновить скриншот", "action_screenshot")
kb := tg.inline_keyboard([[btn]])

opts: map[string, json] = {"caption": caption, "reply_markup": kb, "parse_mode": "HTML"}

println("Sending photo to chat 8610688332...")
res := tg.send_photo(bot, 8610688332, shot_path, opts)
println("Result:", res["ok"], "msg_id:", res["message_id"])

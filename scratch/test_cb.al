use "./libs/telebot/telebot.al" as tg

bot := tg.new_bot("TESTTOKEN", {})

func cb(ctx: json) {
    println("Callback handler reached! data:", ctx["data"], "cb_id:", ctx["callback_query_id"])
}
tg.on_callback_data(bot, "action_status", cb)

fake_upd := json_decode('{"update_id":123,"callback_query":{"id":"cb_123","from":{"id":777,"first_name":"Test"},"message":{"message_id":99,"chat":{"id":777,"type":"private"},"text":"hello"},"data":"action_status"}}')

ev := tg.process_update(bot, fake_upd)
println("handlers found:", len(ev.handlers))
tg.handle_update(bot, fake_upd)

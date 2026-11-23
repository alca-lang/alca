use "./core.al"

bot := core.new_bot("123:abc")
println(bot.api_url)
println(bot.timeout)
core.on_command(bot, "start", |ctx: json| println("start " + string(ctx["text"])))
core.on_message(bot, core.Filter("any", "", null, "", false), |ctx: json| println("any"))
println(len(bot.handlers))
update := json_decode("{\"update_id\":10,\"message\":{\"message_id\":5,\"from\":{\"id\":7,\"first_name\":\"A\",\"username\":\"a\"},\"chat\":{\"id\":7,\"type\":\"private\"},\"text\":\"/start hello world\"}}")
println(type(update))
println(update)
ctx := core.context(bot, update)
println("type", type(ctx))
println(ctx)
println(ctx["kind"])
println(ctx["command"])
println(ctx["args_text"])
println(ctx["chat_id"])
hs := core.dispatch(bot, ctx)
println(len(hs))
for h in hs {
    h.callback(ctx)
}

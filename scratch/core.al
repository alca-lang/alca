// Telebot core prototype
use std.json
use std.urllib2
use std.io
use std.path
use std.crypto
use std.str
use std.re

pub struct Filter {
    mode: string = "any"
    value: string = ""
    chat_id: json = null
    chat_type: string = ""
    case_sensitive: bool = false
}
pub struct Handler {
    kind: string
    filter: Filter
    callback: func
}
pub struct Bot {
    token: string
    api_url: string = "https://api.telegram.org"
    timeout: int = 30
    parse_mode: string = ""
    max_retries: int = 2
    retry_delay: int = 1
    last_update_id: int = 0
    poll_timeout: int = 30
    poll_limit: int = 100
    allowed_updates: list[string] = []
    handlers: list[Handler] = []
    me: json = null
    username: string = ""
}
pub fn new_bot(token: string, opts: map[string, json] = {}) -> Bot {
    b := Bot(token)
    if "api_url" in opts {
        b.api_url = string(opts["api_url"])
    }
    if "timeout" in opts {
        b.timeout = int(opts["timeout"])
    }
    if "parse_mode" in opts {
        b.parse_mode = string(opts["parse_mode"])
    }
    if "max_retries" in opts {
        b.max_retries = int(opts["max_retries"])
    }
    if "retry_delay" in opts {
        b.retry_delay = int(opts["retry_delay"])
    }
    if "poll_timeout" in opts {
        b.poll_timeout = int(opts["poll_timeout"])
    }
    if "poll_limit" in opts {
        b.poll_limit = int(opts["poll_limit"])
    }
    if "allowed_updates" in opts {
        raw_allowed := list(opts["allowed_updates"])
        b.allowed_updates = []
        for x in raw_allowed {
            push(b.allowed_updates, string(x))
        }
    }
    return b
}
pub fn merge_into(target: map[string, json], extra: map[string, json]) -> map[string, json] {
    for k in keys(extra) {
        target[k] = extra[k]
    }
    return target
}
pub fn copy_map(source: map[string, json]) -> map[string, json] {
    out: map[string, json] = {}
    for k in keys(source) {
        out[k] = source[k]
    }
    return out
}
pub fn params() -> map[string, json] {
    empty: map[string, json] = {}
    return empty
}
fn api_url(bot: Bot, method: string) -> string {
    return bot.api_url + "/bot" + bot.token + "/" + method
}
fn parse_body(body: string) -> json {
    parsed: json = null
    e: error | null = null
    parsed = json.parse(body) ?>>> e
    if e != null {
        return null
    }
    return parsed
}
fn telegram_error(parsed: json, status: int) -> error {
    code := status
    if parsed["error_code"] != null {
        code = int(parsed["error_code"])
    }
    desc := "Telegram API error"
    if parsed["description"] != null {
        desc = string(parsed["description"])
    }
    return err(desc, code=code)
}
fn call_raw(token: string, api_url: string, timeout: int, max_retries: int, retry_delay: int, method: string, p: map[string, json]) -> json {
    url := api_url + "/bot" + token + "/" + method
    headers: map[string, string] = {"Content-Type": "application/json", "User-Agent": "Pawl-Telebot/1.0"}
    attempt := 0
    while attempt <= max_retries {
        e: error | null = null
        resp := urllib2.request("POST", url, json.encode(p), headers, timeout) ?>>> e
        if e != null {
            if attempt < max_retries {
                sleep(retry_delay * 1000 * (attempt + 1))
                attempt = attempt + 1
                continue
            }
            err("Telegram network error: " + e.msg, code=e.code)
        }
        status := int(resp["status"])
        body := string(resp["body"])
        parsed := parse_body(body)
        if parsed == null {
            if status >= 500 and attempt < max_retries {
                sleep(retry_delay * 1000 * (attempt + 1))
                attempt = attempt + 1
                continue
            }
            err("Telegram API returned invalid JSON (HTTP " + string(status) + ")", code=502)
        }
        ok := false
        if parsed["ok"] != null {
            ok = bool(parsed["ok"])
        }
        if ok {
            return parsed["result"]
        }
        error_code := status
        if parsed["error_code"] != null {
            error_code = int(parsed["error_code"])
        }
        if error_code == 429 or error_code >= 500 {
            if attempt < max_retries {
                delay := retry_delay
                if parsed["parameters"] != null {
                    if parsed["parameters"]["retry_after"] != null {
                        delay = int(parsed["parameters"]["retry_after"])
                    }
                }
                sleep(delay * 1000)
                attempt = attempt + 1
                continue
            }
        }
        e2 := telegram_error(parsed, status)
        err(e2.msg, code=e2.code)
    }
    err("Telegram API request failed after retries", code=500)
    return null
}
pub fn call(bot: Bot, method: string, p: map[string, json]) -> json {
    return call_raw(bot.token, bot.api_url, bot.timeout, bot.max_retries, bot.retry_delay, method, p)
}
pub fn call_safe(bot: Bot, method: string, p: map[string, json]) -> map[string, json] {
    e: error | null = null
    r := call(bot, method, p) ?>>> e
    if e != null {
        return {"ok": false, "error_code": e.code, "description": e.msg}
    }
    return {"ok": true, "result": r}
}
pub fn api(bot: Bot, method: string, p: map[string, json]) -> json {
    return call(bot, method, p)
}
pub fn get_me(bot: Bot) -> json {
    r := call(bot, "getMe", {})
    bot.me = r
    if r["username"] != null {
        bot.username = string(r["username"])
    }
    return r
}
pub fn start(bot: Bot) -> json {
    return get_me(bot)
}
pub fn get_updates(bot: Bot, opts: map[string, json] = {}) -> list[json] {
    p: map[string, json] = {"offset": bot.last_update_id + 1, "timeout": bot.poll_timeout, "limit": bot.poll_limit}
    if len(bot.allowed_updates) > 0 {
        p["allowed_updates"] = bot.allowed_updates
    }
    merge_into(p, opts)
    r := call(bot, "getUpdates", p)
    updates: list[json] = []
    if type(r) == "LIST" {
        updates = list(r)
    }
    for u in updates {
        if u["update_id"] != null {
            uid := int(u["update_id"])
            if uid > bot.last_update_id {
                bot.last_update_id = uid
            }
        }
    }
    return updates
}
fn event_payload(update: json, kind: string) -> json {
    if update[kind] != null {
        return update[kind]
    }
    return null
}
pub fn update_kind(update: json) -> string {
    for k in ["message", "edited_message", "channel_post", "edited_channel_post", "business_message", "edited_business_message", "guest_message", "callback_query", "inline_query", "chosen_inline_result", "shipping_query", "pre_checkout_query", "poll", "poll_answer", "my_chat_member", "chat_member", "chat_join_request", "message_reaction", "message_reaction_count", "business_connection", "deleted_business_messages", "purchased_paid_media", "managed_bot", "subscription", "stopped_message_generation", "community_chat_joined"] {
        if update[k] != null {
            return k
        }
    }
    return ""
}
fn event_message(update: json, kind: string) -> json {
    if kind == "callback_query" {
        cq := update["callback_query"]
        if cq["message"] != null {
            return cq["message"]
        }
        return null
    }
    if kind == "guest_message" {
        return update["guest_message"]
    }
    if update[kind] != null {
        msg := update[kind]
        if msg["message"] != null {
            return msg["message"]
        }
        return msg
    }
    return null
}
fn event_user(update: json, kind: string, msg: json) -> json {
    if kind == "callback_query" {
        return update["callback_query"]["from"]
    }
    if kind == "inline_query" {
        return update["inline_query"]["from"]
    }
    if kind == "chosen_inline_result" {
        return update["chosen_inline_result"]["from"]
    }
    if kind == "poll_answer" {
        return update["poll_answer"]["user"]
    }
    if msg != null {
        if msg["from"] != null {
            return msg["from"]
        }
    }
    return null
}
fn event_chat(update: json, kind: string, msg: json) -> json {
    if msg != null {
        if msg["chat"] != null {
            return msg["chat"]
        }
    }
    if kind == "my_chat_member" {
        return update["my_chat_member"]["chat"]
    }
    if kind == "chat_member" {
        return update["chat_member"]["chat"]
    }
    if kind == "chat_join_request" {
        return update["chat_join_request"]["chat"]
    }
    return null
}
fn event_text(kind: string, update: json, msg: json) -> string {
    if kind == "callback_query" {
        if update["callback_query"]["data"] != null {
            return string(update["callback_query"]["data"])
        }
        return ""
    }
    if kind == "inline_query" {
        if update["inline_query"]["query"] != null {
            return string(update["inline_query"]["query"])
        }
        return ""
    }
    if kind == "chosen_inline_result" {
        if update["chosen_inline_result"]["query"] != null {
            return string(update["chosen_inline_result"]["query"])
        }
        return ""
    }
    if kind == "poll" {
        if update["poll"]["question"] != null {
            return string(update["poll"]["question"])
        }
        return ""
    }
    if msg != null {
        if msg["text"] != null {
            return string(msg["text"])
        }
        if msg["caption"] != null {
            return string(msg["caption"])
        }
    }
    return ""
}
fn command_of(text: string) -> string {
    if not str_starts_with(text, "/") {
        return ""
    }
    words := str.words(text)
    if len(words) == 0 {
        return ""
    }
    first := words[0]
    cmd := str.slice(first, 1, len(first))
    at := str.index_of(cmd, "@")
    if at >= 0 {
        cmd = str.slice(cmd, 0, at)
    }
    return cmd
}
fn args_of(text: string) -> list[string] {
    words := str.words(text)
    out: list[string] = []
    if len(words) <= 1 {
        return out
    }
    i := 1
    while i < len(words) {
        push(out, words[i])
        i = i + 1
    }
    return out
}
fn serialize_param(value: json) -> string {
    if value == null {
        return ""
    }
    t := type(value)
    if t == "STRING" {
        return string(value)
    }
    if t == "INTEGER" {
        return string(int(value))
    }
    if t == "FLOAT" {
        return string(float(value))
    }
    if t == "BOOL" {
        return string(bool(value))
    }
    return json.encode(value)
}
pub fn context(bot: Bot, update: json) -> json {
    kind := update_kind(update)
    println("ctx1", kind)
    msg := event_message(update, kind)
    println("ctx2", type(msg))
    user := event_user(update, kind, msg)
    println("ctx3", type(user))
    chat := event_chat(update, kind, msg)
    println("ctx4", type(chat))
    text := event_text(kind, update, msg)
    println("ctx5", text)
    ctx: map[string, json] = {}
    ctx["__token"] = bot.token
    ctx["__api_url"] = bot.api_url
    ctx["__timeout"] = bot.timeout
    ctx["__max_retries"] = bot.max_retries
    ctx["__retry_delay"] = bot.retry_delay
    ctx["__parse_mode"] = bot.parse_mode
    ctx["update"] = update
    ctx["update_id"] = update["update_id"]
    ctx["kind"] = kind
    ctx["message"] = msg
    ctx["chat"] = chat
    ctx["from"] = user
    ctx["text"] = text
    ctx["command"] = command_of(text)
    args := args_of(text)
    ctx["args"] = args
    ctx["args_text"] = str.join(args, " ")
    if kind == "callback_query" {
        ctx["callback_query_id"] = update["callback_query"]["id"]
        ctx["data"] = update["callback_query"]["data"]
        ctx["callback_query"] = update["callback_query"]
    }
    if kind == "inline_query" {
        ctx["inline_query_id"] = update["inline_query"]["id"]
        ctx["query"] = update["inline_query"]["query"]
        ctx["inline_query"] = update["inline_query"]
    }
    if msg != null {
        ctx["message_id"] = msg["message_id"]
        if msg["message_thread_id"] != null {
            ctx["message_thread_id"] = msg["message_thread_id"]
        }
        if msg["is_topic_message"] != null {
            ctx["is_topic_message"] = msg["is_topic_message"]
        }
    }
    if chat != null {
        ctx["chat_id"] = chat["id"]
        ctx["chat_type"] = chat["type"]
        if chat["title"] != null {
            ctx["chat_title"] = chat["title"]
        }
        if chat["username"] != null {
            ctx["chat_username"] = chat["username"]
        }
    }
    if user != null {
        ctx["user_id"] = user["id"]
        if user["username"] != null {
            ctx["username"] = user["username"]
        }
        if user["first_name"] != null {
            ctx["first_name"] = user["first_name"]
        }
        if user["last_name"] != null {
            ctx["last_name"] = user["last_name"]
        }
        if user["language_code"] != null {
            ctx["language_code"] = user["language_code"]
        }
        if user["is_bot"] != null {
            ctx["is_bot"] = user["is_bot"]
        }
    }
    return ctx
}
pub fn contexts(bot: Bot, updates: list[json]) -> list[json] {
    out: list[json] = []
    for u in updates {
        push(out, context(bot, u))
    }
    return out
}
pub fn poll_contexts(bot: Bot, opts: map[string, json] = {}) -> list[json] {
    return contexts(bot, get_updates(bot, opts))
}
fn matches_text(mode: string, value: string, text: string, case_sensitive: bool) -> bool {
    subject := text
    needle := value
    if not case_sensitive {
        subject = str_lower(subject)
        needle = str_lower(needle)
    }
    if mode == "text" {
        return subject == needle
    }
    if mode == "contains" {
        return str_contains(subject, needle)
    }
    if mode == "prefix" {
        return str_starts_with(subject, needle)
    }
    if mode == "suffix" {
        return str_ends_with(subject, needle)
    }
    if mode == "regex" {
        return re.match(value, text)
    }
    return true
}
pub fn filter_matches(f: Filter, ctx: json) -> bool {
    if f.chat_id != null {
        if ctx["chat_id"] == null {
            return false
        }
        if string(ctx["chat_id"]) != string(f.chat_id) {
            return false
        }
    }
    if f.chat_type != "" {
        if ctx["chat_type"] == null {
            return false
        }
        if string(ctx["chat_type"]) != f.chat_type {
            return false
        }
    }
    if f.mode == "any" {
        return true
    }
    text := string(ctx["text"])
    if f.mode == "command" {
        actual := string(ctx["command"])
        expected := f.value
        if not f.case_sensitive {
            actual = str_lower(actual)
            expected = str_lower(expected)
        }
        return actual == expected
    }
    if f.mode == "command_prefix" {
        actual := string(ctx["command"])
        expected := f.value
        if not f.case_sensitive {
            actual = str_lower(actual)
            expected = str_lower(expected)
        }
        return str_starts_with(actual, expected)
    }
    if f.mode == "callback" {
        data := string(ctx["data"])
        if f.case_sensitive {
            return data == f.value
        }
        return str_lower(data) == str_lower(f.value)
    }
    if f.mode == "callback_prefix" {
        data := string(ctx["data"])
        if f.case_sensitive {
            return str_starts_with(data, f.value)
        }
        return str_starts_with(str_lower(data), str_lower(f.value))
    }
    return matches_text(f.mode, f.value, text, f.case_sensitive)
}
pub fn dispatch(bot: Bot, ctx: json) -> list[Handler] {
    out: list[Handler] = []
    kind := string(ctx["kind"])
    for h in bot.handlers {
        if h.kind != "*" and h.kind != kind {
            continue
        }
        if filter_matches(h.filter, ctx) {
            push(out, h)
        }
    }
    return out
}
pub fn on(bot: Bot, kind: string, f: Filter, callback: func) -> bool {
    push(bot.handlers, Handler(kind, f, callback))
    return true
}
pub fn on_any(bot: Bot, callback: func) -> bool {
    return on(bot, "*", Filter("any", "", null, "", false), callback)
}
pub fn on_message(bot: Bot, f: Filter, callback: func) -> bool {
    return on(bot, "message", f, callback)
}
pub fn on_command(bot: Bot, command: string, callback: func) -> bool {
    cmd := command
    if str_starts_with(cmd, "/") {
        cmd = str.slice(cmd, 1, len(cmd))
    }
    return on(bot, "message", Filter("command", cmd, null, "", false), callback)
}
pub fn send_message(bot: Bot, chat_id: json, text: string, opts: map[string, json] = {}) -> json {
    p: map[string, json] = {"chat_id": chat_id, "text": text}
    merge_into(p, opts)
    if bot.parse_mode != "" and not ("parse_mode" in p) and not ("entities" in p) {
        p["parse_mode"] = bot.parse_mode
    }
    return call(bot, "sendMessage", p)
}
fn ctx_call(ctx: json, method: string, p: map[string, json]) -> json {
    return call_raw(string(ctx["__token"]), string(ctx["__api_url"]), int(ctx["__timeout"]), int(ctx["__max_retries"]), int(ctx["__retry_delay"]), method, p)
}
pub fn reply(ctx: json, text: string, opts: map[string, json] = {}) -> json {
    p := copy_map(opts)
    if not ("reply_parameters" in p) and ctx["message_id"] != null {
        p["reply_parameters"] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true}
    }
    p["chat_id"] = ctx["chat_id"]
    p["text"] = text
    if string(ctx["__parse_mode"]) != "" and not ("parse_mode" in p) and not ("entities" in p) {
        p["parse_mode"] = string(ctx["__parse_mode"])
    }
    return ctx_call(ctx, "sendMessage", p)
}

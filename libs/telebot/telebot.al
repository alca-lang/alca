// ============================================================================
// telebot — полноценная библиотека Telegram Bot API 10.3 для Alca
// ============================================================================
//
// Быстрый старт:
//
//   use "./libs/telebot/telebot.al" as tg;
//
//   bot := tg.new_bot("123456:ABC-DEF...", {"parse_mode": "HTML"});
//   tg.on_command(bot, "start", |ctx: json| tg.reply(ctx, "Привет!"));
//   tg.on_text(bot, |ctx: json| tg.reply(ctx, "Эхо: " + string(tg.text_of(ctx))));
//
//   tg.poll(bot);
//
// Пошаговый опрос:
//
//   while true:
//       tg.poll_once(bot);
//
// Ручная обработка событий:
//
//   for ev in tg.poll_events(bot):
//       for h in ev.handlers:
//           h.callback(ev.context);
//
// Для сети и файлов нужны sandbox-разрешения:
//   alca run bot.al --allow-net --allow-read --allow-write
// ============================================================================

use std.json;
use std.io;
use std.path;
use std.crypto;
use std.urllib2;
use std.re;
use std.str;

// ----------------------------------------------------------------------------
// Модели
// ----------------------------------------------------------------------------

// Декларативный фильтр обработчика. Комбинируется с условиями по чату,
// пользователю и типу чата.
pub struct Filter:
    mode: string = "any";
    value: string = "";
    chat_id: json = null;
    chat_type: string = "";
    user_id: json = null;
    case_sensitive: bool = false;

// Один обработчик. callback вызывается в main-коде.
pub struct Handler:
    kind: string;
    filter: Filter;
    callback: func;

// Результат обработки одного update.
pub struct Event:
    context: json;
    handlers: list[Handler];

// Клиент бота.
pub struct Bot:
    token: string;
    api_url: string = "https://api.telegram.org";
    timeout: int = 30;
    parse_mode: string = "";
    max_retries: int = 2;
    retry_delay: int = 1;
    last_update_id: int = 0;
    poll_timeout: int = 30;
    poll_limit: int = 100;
    allowed_updates: list[string] = [];
    handlers: list[Handler] = [];
    me: json = null;
    username: string = "";

// ----------------------------------------------------------------------------
// Создание клиента и базовые утилиты
// ----------------------------------------------------------------------------

pub fn new_bot(token: string, opts: map[string, json] = {}) -> Bot:
    b := Bot(token);
    if "api_url" in opts:
        b.api_url = string(opts["api_url"]);
    if "timeout" in opts:
        b.timeout = int(opts["timeout"]);
    if "parse_mode" in opts:
        b.parse_mode = string(opts["parse_mode"]);
    if "max_retries" in opts:
        b.max_retries = int(opts["max_retries"]);
    if "retry_delay" in opts:
        b.retry_delay = int(opts["retry_delay"]);
    if "poll_timeout" in opts:
        b.poll_timeout = int(opts["poll_timeout"]);
    if "poll_limit" in opts:
        b.poll_limit = int(opts["poll_limit"]);
    if "allowed_updates" in opts:
        raw := list(opts["allowed_updates"]);
        b.allowed_updates = [];
        for x in raw:
            push(b.allowed_updates, string(x));
    return b;

pub fn copy_map(source: map[string, json]) -> map[string, json]:
    out: map[string, json] = {};
    for k in keys(source):
        out[k] = source[k];
    return out;

pub fn merge_into(target: map[string, json], extra: map[string, json]) -> map[string, json]:
    for k in keys(extra):
        target[k] = extra[k];
    return target;

pub fn params() -> map[string, json]:
    empty: map[string, json] = {};
    return empty;

pub fn method_url(bot: Bot, method: string) -> string:
    return bot.api_url + "/bot" + bot.token + "/" + method;

// ----------------------------------------------------------------------------
// HTTP-вызовы
// ----------------------------------------------------------------------------

fn parse_body(body: string) -> json:
    parsed := json.parse(body);
    if typeof(parsed) == "error":
        return null;
    return parsed;

fn telegram_error(parsed: json, status: int) -> error:
    err_code := status;
    if parsed["error_code"] != null:
        err_code = int(parsed["error_code"]);
    desc := "Telegram API error";
    if parsed["description"] != null:
        desc = string(parsed["description"]);
    return err(desc, code=err_code);

fn call_json(token: string, api_url: string, timeout: int, max_retries: int, retry_delay: int, method: string, p: map[string, json]) -> map[string, json]:
    url := api_url + "/bot" + token + "/" + method;
    payload := json.encode(p);
    headers: map[string, string] = {"Content-Type": "application/json", "User-Agent": "Alca-Telebot/1.0"};
    attempt := 0;
    while attempt <= max_retries:
        resp: json = urllib2.request("POST", url, payload, headers, timeout);
        if resp == null or resp["status"] == null:
            if attempt < max_retries:
                sleep(retry_delay * 1000 * (attempt + 1));
                attempt = attempt + 1;
                continue;
            err_msg := "Telegram network error";
            if resp != null and resp["msg"] != null:
                err_msg = err_msg + ": " + string(resp["msg"]);
            err_c := 500;
            if resp != null and resp["code"] != null:
                err_c = int(resp["code"]);
            return {"ok": false, "error_code": err_c, "description": err_msg};
        status := int(resp["status"]);
        body := string(resp["body"]);
        parsed := parse_body(body);
        if parsed == null:
            if status >= 500 and attempt < max_retries:
                sleep(retry_delay * 1000 * (attempt + 1));
                attempt = attempt + 1;
                continue;
            return {"ok": false, "error_code": 502, "description": "Telegram API returned invalid JSON (HTTP " + string(status) + ")"};
        ok := false;
        if parsed["ok"] != null:
            ok = bool(parsed["ok"]);
        if ok:
            return {"ok": true, "result": parsed["result"]};
        error_code := status;
        if parsed["error_code"] != null:
            error_code = int(parsed["error_code"]);
        if error_code == 429 or error_code >= 500:
            if attempt < max_retries:
                delay := retry_delay;
                if parsed["parameters"] != null:
                    if parsed["parameters"]["retry_after"] != null:
                        delay = int(parsed["parameters"]["retry_after"]);
                sleep(delay * 1000);
                attempt = attempt + 1;
                continue;
        desc := "Telegram API error";
        if parsed["description"] != null:
            desc = string(parsed["description"]);
        return {"ok": false, "error_code": error_code, "description": desc};
    return {"ok": false, "error_code": 500, "description": "Telegram API request failed after retries"};

fn call_multipart(token: string, api_url: string, timeout: int, max_retries: int, retry_delay: int, method: string, p: map[string, json], files: map[string, string]) -> map[string, json]:
    url := api_url + "/bot" + token + "/" + method;
    boundary := "AlcaTelebot" + crypto.random_string(32);
    body := "";
    for k in keys(p):
        v := p[k];
        if v == null:
            continue;
        body = body + "--" + boundary + "\r\n";
        if k in files:
            file_path := files[k];
            content: json = io.read_file(file_path);
            if content == null:
                return {"ok": false, "error_code": 400, "description": "cannot read file " + file_path};
            filename := path.base(file_path);
            body = body + "Content-Disposition: form-data; name=\"" + k + "\"; filename=\"" + filename + "\"\r\n";
            body = body + "Content-Type: " + mime_type(file_path) + "\r\n\r\n";
            body = body + string(content) + "\r\n";
        else:
            body = body + "Content-Disposition: form-data; name=\"" + k + "\"\r\n\r\n";
            body = body + param_to_string(v) + "\r\n";
    body = body + "--" + boundary + "--\r\n";
    headers: map[string, string] = {"Content-Type": "multipart/form-data; boundary=" + boundary, "User-Agent": "Alca-Telebot/1.0"};
    attempt := 0;
    while attempt <= max_retries:
        resp: json = urllib2.request("POST", url, body, headers, timeout);
        if resp == null or resp["status"] == null:
            if attempt < max_retries:
                sleep(retry_delay * 1000 * (attempt + 1));
                attempt = attempt + 1;
                continue;
            err_msg := "Telegram network error";
            if resp != null and resp["msg"] != null:
                err_msg = err_msg + ": " + string(resp["msg"]);
            err_c := 500;
            if resp != null and resp["code"] != null:
                err_c = int(resp["code"]);
            return {"ok": false, "error_code": err_c, "description": err_msg};
        status := int(resp["status"]);
        body_text := string(resp["body"]);
        parsed := parse_body(body_text);
        if parsed == null:
            if status >= 500 and attempt < max_retries:
                sleep(retry_delay * 1000 * (attempt + 1));
                attempt = attempt + 1;
                continue;
            return {"ok": false, "error_code": 502, "description": "Telegram API returned invalid JSON (HTTP " + string(status) + ")"};
        ok := false;
        if parsed["ok"] != null:
            ok = bool(parsed["ok"]);
        if ok:
            return {"ok": true, "result": parsed["result"]};
        error_code := status;
        if parsed["error_code"] != null:
            error_code = int(parsed["error_code"]);
        if error_code == 429 or error_code >= 500:
            if attempt < max_retries:
                delay := retry_delay;
                if parsed["parameters"] != null:
                    if parsed["parameters"]["retry_after"] != null:
                        delay = int(parsed["parameters"]["retry_after"]);
                sleep(delay * 1000);
                attempt = attempt + 1;
                continue;
        desc := "Telegram API error";
        if parsed["description"] != null:
            desc = string(parsed["description"]);
        return {"ok": false, "error_code": error_code, "description": desc};
    return {"ok": false, "error_code": 500, "description": "Telegram API request failed after retries"};

// Универсальный вызов любого метода Bot API.
// Если среди параметров есть путь к локальному файлу в поле InputFile,
// запрос автоматически отправляется как multipart/form-data.
pub fn call_safe(bot: Bot, method: string, p: map[string, json]) -> map[string, json]:
    files := file_params(p);
    if len(files) > 0:
        return call_multipart(bot.token, bot.api_url, bot.timeout, bot.max_retries, bot.retry_delay, method, p, files);
    return call_json(bot.token, bot.api_url, bot.timeout, bot.max_retries, bot.retry_delay, method, p);

pub fn call(bot: Bot, method: string, p: map[string, json]) -> json:
    res := call_safe(bot, method, p);
    if bool(res["ok"]):
        return res["result"];
    err(string(res["description"]), code=int(res["error_code"]));
    return null;

pub fn api(bot: Bot, method: string, p: map[string, json]) -> json:
    return call(bot, method, p);

// ----------------------------------------------------------------------------
// Upload helpers
// ----------------------------------------------------------------------------

fn is_file_field(name: string) -> bool:
    for n in ["certificate", "photo", "live_photo", "audio", "thumbnail", "document", "video", "cover", "animation", "voice", "video_note", "sticker"]:
        if name == n:
            return true;
    return false;

fn is_local_file(value: json) -> bool:
    if type(value) != "STRING":
        return false;
    s := string(value);
    if len(s) == 0 or len(s) > 1024:
        return false;
    if str_contains(s, "://"):
        return false;
    if str_starts_with(s, "attach://"):
        return false;
    e: error | null = null;
    exists := io.file_exists(s) ?>>> e;
    if e != null:
        return false;
    return exists;

fn file_params(p: map[string, json]) -> map[string, string]:
    out: map[string, string] = {};
    for k in keys(p):
        if is_file_field(k) and is_local_file(p[k]):
            out[k] = string(p[k]);
    return out;

fn param_to_string(value: json) -> string:
    if value == null:
        return "";
    t := type(value);
    if t == "STRING":
        return string(value);
    if t == "INTEGER":
        return string(int(value));
    if t == "FLOAT":
        return string(float(value));
    if t == "BOOL":
        if bool(value):
            return "true";
        return "false";
    return json_encode(value);

fn mime_type(file_path: string) -> string:
    ext := str_lower(path.ext(file_path));
    if ext == ".jpg" or ext == ".jpeg":
        return "image/jpeg";
    if ext == ".png":
        return "image/png";
    if ext == ".webp":
        return "image/webp";
    if ext == ".gif":
        return "image/gif";
    if ext == ".mp4":
        return "video/mp4";
    if ext == ".mov":
        return "video/quicktime";
    if ext == ".mp3":
        return "audio/mpeg";
    if ext == ".ogg" or ext == ".oga":
        return "audio/ogg";
    if ext == ".wav":
        return "audio/wav";
    if ext == ".pdf":
        return "application/pdf";
    if ext == ".zip":
        return "application/zip";
    return "application/octet-stream";

// ----------------------------------------------------------------------------
// Update / context
// ----------------------------------------------------------------------------

pub fn update_kinds() -> list[string]:
    return ["message", "edited_message", "channel_post", "edited_channel_post", "business_connection", "business_message", "edited_business_message", "deleted_business_messages", "guest_message", "message_reaction", "message_reaction_count", "inline_query", "chosen_inline_result", "callback_query", "shipping_query", "pre_checkout_query", "purchased_paid_media", "poll", "poll_answer", "my_chat_member", "chat_member", "chat_join_request", "chat_boost", "removed_chat_boost", "managed_bot", "subscription", "stopped_message_generation"];

pub fn update_kind(update: json) -> string:
    if update == null:
        return "";
    for k in update_kinds():
        if update[k] != null:
            return k;
    return "";

fn event_message(update: json, kind: string) -> json:
    if kind == "callback_query":
        cq := update["callback_query"];
        if cq["message"] != null:
            return cq["message"];
        return null;
    if kind == "guest_message":
        return update["guest_message"];
    if kind == "stopped_message_generation":
        payload := update["stopped_message_generation"];
        if payload["message"] != null:
            return payload["message"];
        return null;
    if update[kind] != null:
        payload := update[kind];
        if type(payload) == "MAP":
            if payload["message"] != null:
                return payload["message"];
            if payload["pinned_message"] != null:
                return payload["pinned_message"];
        return payload;
    return null;

fn event_user(update: json, kind: string, msg: json) -> json:
    if kind == "callback_query":
        return update["callback_query"]["from"];
    if kind == "inline_query":
        return update["inline_query"]["from"];
    if kind == "chosen_inline_result":
        return update["chosen_inline_result"]["from"];
    if kind == "poll_answer":
        return update["poll_answer"]["user"];
    if kind == "shipping_query":
        return update["shipping_query"]["from"];
    if kind == "pre_checkout_query":
        return update["pre_checkout_query"]["from"];
    if kind == "chat_join_request":
        return update["chat_join_request"]["from"];
    if kind == "message_reaction" or kind == "message_reaction_count":
        if update[kind]["user"] != null:
            return update[kind]["user"];
    if kind == "business_connection":
        return update["business_connection"]["user"];
    if kind == "my_chat_member" or kind == "chat_member":
        if update[kind]["from"] != null:
            return update[kind]["from"];
        if update[kind]["new_chat_member"] != null:
            return update[kind]["new_chat_member"]["user"];
    if kind == "chat_boost":
        if update["chat_boost"]["boost"] != null:
            if update["chat_boost"]["boost"]["source"] != null:
                return update["chat_boost"]["boost"]["source"]["user"];
    if msg != null:
        if msg["from"] != null:
            return msg["from"];
    return null;

fn event_chat(update: json, kind: string, msg: json) -> json:
    if msg != null:
        if msg["chat"] != null:
            return msg["chat"];
    if update[kind] != null:
        payload := update[kind];
        if type(payload) == "MAP":
            if payload["chat"] != null:
                return payload["chat"];
    return null;

fn event_text(update: json, kind: string, msg: json) -> string:
    if kind == "callback_query":
        cq := update["callback_query"];
        if cq["data"] != null:
            return string(cq["data"]);
        return "";
    if kind == "inline_query":
        return string(update["inline_query"]["query"]);
    if kind == "chosen_inline_result":
        return string(update["chosen_inline_result"]["query"]);
    if kind == "poll":
        return string(update["poll"]["question"]);
    if kind == "shipping_query":
        return string(update["shipping_query"]["payload"]);
    if kind == "pre_checkout_query":
        return string(update["pre_checkout_query"]["payload"]);
    if kind == "message_reaction":
        return "";
    if msg != null:
        if msg["text"] != null:
            return string(msg["text"]);
        if msg["caption"] != null:
            return string(msg["caption"]);
    return "";

pub fn message_content_type(msg: json) -> string:
    if msg == null:
        return "";
    for k in ["text", "photo", "document", "audio", "video", "animation", "voice", "video_note", "sticker", "location", "venue", "contact", "poll", "dice", "game", "story", "paid_media", "live_photo", "checklist", "rich_message"]:
        if msg[k] != null:
            return k;
    return "";

fn command_of(text: string) -> string:
    if not str_starts_with(text, "/"):
        return "";
    words := str.words(text);
    if len(words) == 0:
        return "";
    first := words[0];
    cmd := str.slice(first, 1, str.length(first));
    at := str.index_of(cmd, "@");
    if at >= 0:
        cmd = str.slice(cmd, 0, at);
    return cmd;

fn args_of(text: string) -> list[string]:
    out: list[string] = [];
    if not str_starts_with(text, "/"):
        return out;
    words := str.words(text);
    if len(words) <= 1:
        return out;
    i := 1;
    while i < len(words):
        push(out, words[i]);
        i = i + 1;
    return out;

fn has_field(obj: json, name: string) -> bool:
    if obj == null:
        return false;
    if type(obj) != "MAP":
        return false;
    return obj[name] != null;

// Собирает нормализованный контекст из update.
pub fn context(bot: Bot, update: json) -> json:
    kind := update_kind(update);
    msg := event_message(update, kind);
    user := event_user(update, kind, msg);
    chat := event_chat(update, kind, msg);
    text := event_text(update, kind, msg);
    ctx: map[string, json] = {};
    ctx["__token"] = bot.token;
    ctx["__api_url"] = bot.api_url;
    ctx["__timeout"] = bot.timeout;
    ctx["__max_retries"] = bot.max_retries;
    ctx["__retry_delay"] = bot.retry_delay;
    ctx["__parse_mode"] = bot.parse_mode;
    ctx["update"] = update;
    ctx["update_id"] = update["update_id"];
    ctx["kind"] = kind;
    ctx["payload"] = update[kind];
    ctx["message"] = msg;
    ctx["chat"] = chat;
    ctx["from"] = user;
    ctx["text"] = text;
    ctx["command"] = command_of(text);
    ctx["is_command"] = str_starts_with(text, "/");
    args := args_of(text);
    ctx["args"] = args;
    ctx["args_text"] = str.join(args, " ");
    ctx["content_type"] = message_content_type(msg);
    if kind == "callback_query":
        cq := update["callback_query"];
        ctx["callback_query"] = cq;
        ctx["callback_query_id"] = cq["id"];
        ctx["data"] = cq["data"];
        ctx["callback_data"] = cq["data"];
        ctx["inline_message_id"] = cq["inline_message_id"];
        ctx["chat_instance"] = cq["chat_instance"];
    if kind == "inline_query":
        iq := update["inline_query"];
        ctx["inline_query"] = iq;
        ctx["inline_query_id"] = iq["id"];
        ctx["query"] = iq["query"];
        ctx["offset"] = iq["offset"];
    if kind == "chosen_inline_result":
        cr := update["chosen_inline_result"];
        ctx["chosen_inline_result"] = cr;
        ctx["result_id"] = cr["result_id"];
        ctx["query"] = cr["query"];
        ctx["inline_message_id"] = cr["inline_message_id"];
    if kind == "shipping_query":
        sq := update["shipping_query"];
        ctx["shipping_query"] = sq;
        ctx["shipping_query_id"] = sq["id"];
        ctx["payload"] = sq["payload"];
    if kind == "pre_checkout_query":
        pq := update["pre_checkout_query"];
        ctx["pre_checkout_query"] = pq;
        ctx["pre_checkout_query_id"] = pq["id"];
        ctx["payload"] = pq["payload"];
        ctx["currency"] = pq["currency"];
        ctx["total_amount"] = pq["total_amount"];
    if kind == "poll":
        ctx["poll"] = update["poll"];
        ctx["poll_id"] = update["poll"]["id"];
    if kind == "poll_answer":
        ctx["poll_answer"] = update["poll_answer"];
        ctx["poll_id"] = update["poll_answer"]["poll_id"];
    if kind == "chat_join_request":
        cjr := update["chat_join_request"];
        ctx["chat_join_request"] = cjr;
        ctx["user_chat_id"] = cjr["user_chat_id"];
        ctx["bio"] = cjr["bio"];
    if kind == "message_reaction":
        ctx["message_reaction"] = update["message_reaction"];
    if kind == "message_reaction_count":
        ctx["message_reaction_count"] = update["message_reaction_count"];
    if kind == "business_connection":
        bc := update["business_connection"];
        ctx["business_connection"] = bc;
        ctx["business_connection_id"] = bc["id"];
    if kind == "business_message" or kind == "edited_business_message":
        payload := update[kind];
        if payload["business_connection_id"] != null:
            ctx["business_connection_id"] = payload["business_connection_id"];
    if kind == "deleted_business_messages":
        ctx["deleted_business_messages"] = update["deleted_business_messages"];
        ctx["business_connection_id"] = update["deleted_business_messages"]["business_connection_id"];
    if kind == "my_chat_member" or kind == "chat_member":
        ctx["chat_member"] = update[kind];
    if kind == "chat_boost":
        ctx["chat_boost"] = update["chat_boost"];
    if kind == "removed_chat_boost":
        ctx["removed_chat_boost"] = update["removed_chat_boost"];
    if msg != null:
        ctx["message_id"] = msg["message_id"];
        if msg["message_thread_id"] != null:
            ctx["message_thread_id"] = msg["message_thread_id"];
        if msg["is_topic_message"] != null:
            ctx["is_topic_message"] = msg["is_topic_message"];
        if msg["reply_to_message"] != null:
            ctx["reply_to_message"] = msg["reply_to_message"];
            ctx["reply_to_message_id"] = msg["reply_to_message"]["message_id"];
        if msg["date"] != null:
            ctx["date"] = msg["date"];
        if msg["caption"] != null:
            ctx["caption"] = msg["caption"];
        if msg["business_connection_id"] != null:
            ctx["business_connection_id"] = msg["business_connection_id"];
    if chat != null:
        ctx["chat_id"] = chat["id"];
        ctx["chat_type"] = chat["type"];
        if chat["title"] != null:
            ctx["chat_title"] = chat["title"];
        if chat["username"] != null:
            ctx["chat_username"] = chat["username"];
        if chat["is_forum"] != null:
            ctx["is_forum"] = chat["is_forum"];
    if user != null:
        ctx["user_id"] = user["id"];
        if user["username"] != null:
            ctx["username"] = user["username"];
        if user["first_name"] != null:
            ctx["first_name"] = user["first_name"];
        if user["last_name"] != null:
            ctx["last_name"] = user["last_name"];
        if user["language_code"] != null:
            ctx["language_code"] = user["language_code"];
        if user["is_bot"] != null:
            ctx["is_bot"] = user["is_bot"];
    return ctx;

pub fn contexts(bot: Bot, updates: list[json]) -> list[json]:
    out: list[json] = [];
    for u in updates:
        push(out, context(bot, u));
    return out;

// ----------------------------------------------------------------------------
// getUpdates / polling
// ----------------------------------------------------------------------------

pub fn get_updates(bot: Bot, opts: map[string, json] = {}) -> list[json]:
    p: map[string, json] = {"offset": bot.last_update_id + 1, "timeout": bot.poll_timeout, "limit": bot.poll_limit};
    if len(bot.allowed_updates) > 0:
        p["allowed_updates"] = bot.allowed_updates;
    merge_into(p, opts);
    r := call(bot, "getUpdates", p);
    updates: list[json] = [];
    if typeof(r) == "list" or type(r) == "LIST":
        updates = list(r);
    for u in updates:
        if u["update_id"] != null:
            uid := int(u["update_id"]);
            if uid > bot.last_update_id:
                bot.last_update_id = uid;
    return updates;

pub fn poll_updates(bot: Bot, opts: map[string, json] = {}) -> list[json]:
    return get_updates(bot, opts);

pub fn poll_contexts(bot: Bot, opts: map[string, json] = {}) -> list[json]:
    return contexts(bot, get_updates(bot, opts));

pub fn process_update(bot: Bot, update: json) -> Event:
    ctx := context(bot, update);
    return Event(ctx, dispatch(bot, ctx));

// Обработать один апдейт: найти зарегистрированные обработчики и вызвать их callback
pub fn handle_update(bot: Bot, update: json) -> int:
    ev := process_update(bot, update);
    count := 0;
    for h in ev.handlers:
        h.callback(ev.context);
        count = count + 1;
    return count;

// Сделать один раунд polling и автоматически вызвать подходящие обработчики
pub fn poll_once(bot: Bot, opts: map[string, json] = {}) -> int:
    updates := get_updates(bot, opts);
    total := 0;
    for u in updates:
        total = total + handle_update(bot, u);
    return total;

// Запустить непрерывный цикл long polling
pub fn poll(bot: Bot, opts: map[string, json] = {}):
    if bot.username == "":
        get_me(bot);
    while true:
        poll_once(bot, opts);

pub fn poll_events(bot: Bot, opts: map[string, json] = {}) -> list[Event]:
    out: list[Event] = [];
    for u in get_updates(bot, opts):
        push(out, process_update(bot, u));
    return out;

pub fn next_context(bot: Bot, opts: map[string, json] = {}) -> json:
    ctxs := poll_contexts(bot, opts);
    if len(ctxs) == 0:
        return null;
    return ctxs[0];

pub fn get_me(bot: Bot) -> json:
    r := call(bot, "getMe", {});
    bot.me = r;
    if r != null:
        if r["username"] != null:
            bot.username = string(r["username"]);
    return r;

pub fn start(bot: Bot) -> json:
    return get_me(bot);

// ----------------------------------------------------------------------------
// Фильтры
// ----------------------------------------------------------------------------

fn base_filter() -> Filter:
    return Filter("any", "", null, "", null, false);

pub fn filter_any() -> Filter:
    return base_filter();

pub fn filter_command(command: string) -> Filter:
    cmd := command;
    if str_starts_with(cmd, "/"):
        cmd = str.slice(cmd, 1, str.length(cmd));
    return Filter("command", cmd, null, "", null, false);

pub fn filter_command_prefix(prefix: string) -> Filter:
    p := prefix;
    if str_starts_with(p, "/"):
        p = str.slice(p, 1, str.length(p));
    return Filter("command_prefix", p, null, "", null, false);

pub fn filter_text(value: string) -> Filter:
    return Filter("text", value, null, "", null, false);

pub fn filter_contains(value: string) -> Filter:
    return Filter("contains", value, null, "", null, false);

pub fn filter_prefix(value: string) -> Filter:
    return Filter("prefix", value, null, "", null, false);

pub fn filter_suffix(value: string) -> Filter:
    return Filter("suffix", value, null, "", null, false);

pub fn filter_regex(pattern: string) -> Filter:
    return Filter("regex", pattern, null, "", null, false);

pub fn filter_callback(data: string) -> Filter:
    return Filter("callback", data, null, "", null, false);

pub fn filter_callback_prefix(prefix: string) -> Filter:
    return Filter("callback_prefix", prefix, null, "", null, false);

pub fn filter_chat(chat_id: json) -> Filter:
    return Filter("any", "", chat_id, "", null, false);

pub fn filter_chat_type(chat_type: string) -> Filter:
    return Filter("any", "", null, chat_type, null, false);

pub fn filter_user(user_id: json) -> Filter:
    return Filter("any", "", null, "", user_id, false);

pub fn filter_has(what: string) -> Filter:
    return Filter("has_" + what, "", null, "", null, false);

pub fn filter_is_bot() -> Filter:
    return Filter("is_bot", "", null, "", null, false);

pub fn filter_is_forwarded() -> Filter:
    return Filter("is_forwarded", "", null, "", null, false);

pub fn filter_is_reply() -> Filter:
    return Filter("is_reply", "", null, "", null, false);

pub fn filter_case_insensitive(f: Filter) -> Filter:
    f.case_sensitive = false;
    return f;

pub fn filter_case_sensitive(f: Filter) -> Filter:
    f.case_sensitive = true;
    return f;

pub fn filter_in_chat(f: Filter, chat_id: json) -> Filter:
    f.chat_id = chat_id;
    return f;

pub fn filter_in_chat_type(f: Filter, chat_type: string) -> Filter:
    f.chat_type = chat_type;
    return f;

pub fn filter_from_user(f: Filter, user_id: json) -> Filter:
    f.user_id = user_id;
    return f;

fn matches_text(mode: string, value: string, text: string, case_sensitive: bool) -> bool:
    subject := text;
    needle := value;
    if not case_sensitive:
        subject = str_lower(subject);
        needle = str_lower(needle);
    if mode == "text":
        return subject == needle;
    if mode == "contains":
        return str_contains(subject, needle);
    if mode == "prefix":
        return str_starts_with(subject, needle);
    if mode == "suffix":
        return str_ends_with(subject, needle);
    if mode == "regex":
        return re.match(value, text);
    return true;

fn msg_has(msg: json, field: string) -> bool:
    if msg == null:
        return false;
    return msg[field] != null;

pub fn filter_matches(f: Filter, ctx: json) -> bool:
    if f.chat_id != null:
        if ctx["chat_id"] == null:
            return false;
        if string(ctx["chat_id"]) != string(f.chat_id):
            return false;
    if f.chat_type != "":
        if ctx["chat_type"] == null:
            return false;
        if string(ctx["chat_type"]) != f.chat_type:
            return false;
    if f.user_id != null:
        if ctx["user_id"] == null:
            return false;
        if string(ctx["user_id"]) != string(f.user_id):
            return false;
    mode := f.mode;
    if mode == "any":
        return true;
    if mode == "has_text":
        return ctx["text"] != null and string(ctx["text"]) != "";
    if mode == "has_caption":
        return ctx["message"] != null and msg_has(ctx["message"], "caption");
    if mode == "is_forwarded":
        msg := ctx["message"];
        if msg == null:
            return false;
        return msg["forward_origin"] != null or msg["forward_date"] != null;
    if mode == "is_reply":
        return ctx["reply_to_message"] != null;
    if mode == "is_bot":
        return ctx["is_bot"] != null and bool(ctx["is_bot"]);
    if str_starts_with(mode, "has_"):
        field := str.slice(mode, 4, str.length(mode));
        if field == "media":
            msg := ctx["message"];
            for m in ["photo", "document", "audio", "video", "animation", "voice", "video_note", "sticker", "live_photo", "paid_media"]:
                if msg_has(msg, m):
                    return true;
            return false;
        return msg_has(ctx["message"], field);
    text := "";
    if ctx["text"] != null:
        text = string(ctx["text"]);
    if mode == "command":
        actual := string(ctx["command"]);
        expected := f.value;
        if not f.case_sensitive:
            actual = str_lower(actual);
            expected = str_lower(expected);
        return actual == expected;
    if mode == "command_prefix":
        actual := string(ctx["command"]);
        expected := f.value;
        if not f.case_sensitive:
            actual = str_lower(actual);
            expected = str_lower(expected);
        return str_starts_with(actual, expected);
    if mode == "callback":
        data := "";
        if ctx["data"] != null:
            data = string(ctx["data"]);
        if f.case_sensitive:
            return data == f.value;
        return str_lower(data) == str_lower(f.value);
    if mode == "callback_prefix":
        data := "";
        if ctx["data"] != null:
            data = string(ctx["data"]);
        if f.case_sensitive:
            return str_starts_with(data, f.value);
        return str_starts_with(str_lower(data), str_lower(f.value));
    return matches_text(mode, f.value, text, f.case_sensitive);

// ----------------------------------------------------------------------------
// Регистрация обработчиков и dispatch
// ----------------------------------------------------------------------------

pub fn on(bot: Bot, kind: string, f: Filter, callback: func) -> bool:
    push(bot.handlers, Handler(kind, f, callback));
    return true;

pub fn on_any(bot: Bot, callback: func) -> bool:
    return on(bot, "*", filter_any(), callback);

pub fn on_message(bot: Bot, callback: func) -> bool:
    return on(bot, "message", filter_any(), callback);

pub fn on_text(bot: Bot, callback: func) -> bool:
    return on(bot, "message", filter_has("text"), callback);

pub fn on_command(bot: Bot, command: string, callback: func) -> bool:
    return on(bot, "message", filter_command(command), callback);

pub fn on_command_prefix(bot: Bot, prefix: string, callback: func) -> bool:
    return on(bot, "message", filter_command_prefix(prefix), callback);

pub fn on_callback(bot: Bot, callback: func) -> bool:
    return on(bot, "callback_query", filter_any(), callback);

pub fn on_callback_data(bot: Bot, data: string, callback: func) -> bool:
    return on(bot, "callback_query", filter_callback(data), callback);

pub fn on_callback_prefix(bot: Bot, prefix: string, callback: func) -> bool:
    return on(bot, "callback_query", filter_callback_prefix(prefix), callback);

pub fn on_edited_message(bot: Bot, callback: func) -> bool:
    return on(bot, "edited_message", filter_any(), callback);

pub fn on_channel_post(bot: Bot, callback: func) -> bool:
    return on(bot, "channel_post", filter_any(), callback);

pub fn on_edited_channel_post(bot: Bot, callback: func) -> bool:
    return on(bot, "edited_channel_post", filter_any(), callback);

pub fn on_business_message(bot: Bot, callback: func) -> bool:
    return on(bot, "business_message", filter_any(), callback);

pub fn on_guest_message(bot: Bot, callback: func) -> bool:
    return on(bot, "guest_message", filter_any(), callback);

pub fn on_inline_query(bot: Bot, callback: func) -> bool:
    return on(bot, "inline_query", filter_any(), callback);

pub fn on_chosen_inline_result(bot: Bot, callback: func) -> bool:
    return on(bot, "chosen_inline_result", filter_any(), callback);

pub fn on_poll(bot: Bot, callback: func) -> bool:
    return on(bot, "poll", filter_any(), callback);

pub fn on_poll_answer(bot: Bot, callback: func) -> bool:
    return on(bot, "poll_answer", filter_any(), callback);

pub fn on_chat_member(bot: Bot, callback: func) -> bool:
    return on(bot, "chat_member", filter_any(), callback);

pub fn on_my_chat_member(bot: Bot, callback: func) -> bool:
    return on(bot, "my_chat_member", filter_any(), callback);

pub fn on_chat_join_request(bot: Bot, callback: func) -> bool:
    return on(bot, "chat_join_request", filter_any(), callback);

pub fn on_shipping_query(bot: Bot, callback: func) -> bool:
    return on(bot, "shipping_query", filter_any(), callback);

pub fn on_pre_checkout_query(bot: Bot, callback: func) -> bool:
    return on(bot, "pre_checkout_query", filter_any(), callback);

pub fn on_message_reaction(bot: Bot, callback: func) -> bool:
    return on(bot, "message_reaction", filter_any(), callback);

pub fn dispatch(bot: Bot, ctx: json) -> list[Handler]:
    out: list[Handler] = [];
    kind := "";
    if ctx["kind"] != null:
        kind = string(ctx["kind"]);
    for h in bot.handlers:
        if h.kind != "*" and h.kind != kind:
            continue;
        if filter_matches(h.filter, ctx):
            push(out, h);
    return out;

// ----------------------------------------------------------------------------
// Ответы из контекста
// ----------------------------------------------------------------------------

fn apply_parse_mode(ctx: json, p: map[string, json]):
    pm := string(ctx["__parse_mode"]);
    if pm != "" and not ("parse_mode" in p) and not ("entities" in p) and not ("caption_entities" in p):
        p["parse_mode"] = pm;

fn ctx_call(ctx: json, method: string, p: map[string, json]) -> json:
    return call_json(string(ctx["__token"]), string(ctx["__api_url"]), int(ctx["__timeout"]), int(ctx["__max_retries"]), int(ctx["__retry_delay"]), method, p);

pub fn reply(ctx: json, text: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    p["chat_id"] = ctx["chat_id"];
    p["text"] = text;
    apply_parse_mode(ctx, p);
    return ctx_call(ctx, "sendMessage", p);

pub fn answer(ctx: json, text: string, opts: map[string, json] = {}) -> json:
    return reply(ctx, text, opts);

pub fn send(ctx: json, text: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["text"] = text;
    apply_parse_mode(ctx, p);
    return ctx_call(ctx, "sendMessage", p);

pub fn send_message(bot: Bot, chat_id: json, text: string, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "text": text};
    merge_into(p, opts);
    if bot.parse_mode != "" and not ("parse_mode" in p) and not ("entities" in p):
        p["parse_mode"] = bot.parse_mode;
    return call(bot, "sendMessage", p);

pub fn edit_message_text(bot: Bot, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    if bot.parse_mode != "" and not ("parse_mode" in p) and not ("entities" in p):
        p["parse_mode"] = bot.parse_mode;
    return call(bot, "editMessageText", p);

pub fn edit(ctx: json, text: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["text"] = text;
    if ctx["inline_message_id"] != null:
        p["inline_message_id"] = ctx["inline_message_id"];
    else:
        p["chat_id"] = ctx["chat_id"];
        p["message_id"] = ctx["message_id"];
    apply_parse_mode(ctx, p);
    return ctx_call(ctx, "editMessageText", p);

pub fn edit_caption(ctx: json, caption: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["caption"] = caption;
    if ctx["inline_message_id"] != null:
        p["inline_message_id"] = ctx["inline_message_id"];
    else:
        p["chat_id"] = ctx["chat_id"];
        p["message_id"] = ctx["message_id"];
    apply_parse_mode(ctx, p);
    return ctx_call(ctx, "editMessageCaption", p);

pub fn edit_markup(ctx: json, reply_markup: json) -> json:
    p: map[string, json] = {"reply_markup": reply_markup};
    if ctx["inline_message_id"] != null:
        p["inline_message_id"] = ctx["inline_message_id"];
    else:
        p["chat_id"] = ctx["chat_id"];
        p["message_id"] = ctx["message_id"];
    return ctx_call(ctx, "editMessageReplyMarkup", p);

pub fn delete(ctx: json) -> json:
    p: map[string, json] = {"chat_id": ctx["chat_id"], "message_id": ctx["message_id"]};
    return ctx_call(ctx, "deleteMessage", p);

pub fn answer_callback(ctx: json, text: string = "", opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["callback_query_id"] = ctx["callback_query_id"];
    if text != "":
        p["text"] = text;
    return ctx_call(ctx, "answerCallbackQuery", p);

pub fn answer_callback_alert(ctx: json, text: string) -> json:
    return answer_callback(ctx, text, {"show_alert": true});

pub fn reply_photo(ctx: json, photo: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["photo"] = photo;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendPhoto", p);

pub fn reply_document(ctx: json, document: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["document"] = document;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendDocument", p);

pub fn reply_video(ctx: json, video: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["video"] = video;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendVideo", p);

pub fn reply_audio(ctx: json, audio: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["audio"] = audio;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendAudio", p);

pub fn reply_voice(ctx: json, voice: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["voice"] = voice;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendVoice", p);

pub fn reply_sticker(ctx: json, sticker: string, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["sticker"] = sticker;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendSticker", p);

pub fn reply_location(ctx: json, latitude: float, longitude: float, opts: map[string, json] = {}) -> json:
    p := copy_map(opts);
    p["chat_id"] = ctx["chat_id"];
    p["latitude"] = latitude;
    p["longitude"] = longitude;
    if not ("reply_parameters" in p) and ctx["message_id"] != null:
        rp: map[string, json] = {"message_id": ctx["message_id"], "allow_sending_without_reply": true};
        p["reply_parameters"] = rp;
    return ctx_call(ctx, "sendLocation", p);

pub fn typing(ctx: json) -> json:
    p: map[string, json] = {"chat_id": ctx["chat_id"], "action": "typing"};
    return ctx_call(ctx, "sendChatAction", p);

pub fn send_long(bot: Bot, chat_id: json, text: string, opts: map[string, json] = {}, limit: int = 4000) -> list[json]:
    out: list[json] = [];
    for chunk in split_text(text, limit):
        push(out, send_message(bot, chat_id, chunk, opts));
    return out;

// ----------------------------------------------------------------------------
// Клавиатуры
// ----------------------------------------------------------------------------

pub fn inline_button(text: string, callback_data: string) -> map[string, json]:
    return {"text": text, "callback_data": callback_data};

pub fn url_button(text: string, url: string) -> map[string, json]:
    return {"text": text, "url": url};

pub fn web_app_button(text: string, url: string) -> map[string, json]:
    app: map[string, json] = {"url": url};
    return {"text": text, "web_app": app};

pub fn switch_inline_button(text: string, query: string = "") -> map[string, json]:
    return {"text": text, "switch_inline_query": query};

pub fn switch_inline_current_button(text: string, query: string = "") -> map[string, json]:
    return {"text": text, "switch_inline_query_current_chat": query};

pub fn login_url_button(text: string, url: string, opts: map[string, json] = {}) -> map[string, json]:
    login: map[string, json] = {"url": url};
    merge_into(login, opts);
    return {"text": text, "login_url": login};

pub fn copy_text_button(text: string, copy_text: string) -> map[string, json]:
    ct: map[string, json] = {"text": copy_text};
    return {"text": text, "copy_text": ct};

pub fn callback_game_button(text: string) -> map[string, json]:
    cg: map[string, json] = {};
    return {"text": text, "callback_game": cg};

pub fn pay_button(text: string) -> map[string, json]:
    return {"text": text, "pay": true};

pub fn inline_keyboard(rows: json) -> map[string, json]:
    return {"inline_keyboard": rows};

pub fn reply_button(text: string) -> map[string, json]:
    return {"text": text};

pub fn request_contact_button(text: string) -> map[string, json]:
    return {"text": text, "request_contact": true};

pub fn request_location_button(text: string) -> map[string, json]:
    return {"text": text, "request_location": true};

pub fn request_poll_button(text: string, poll_type: string = "") -> map[string, json]:
    rp: map[string, json] = {};
    if poll_type != "":
        rp["type"] = poll_type;
    return {"text": text, "request_poll": rp};

pub fn request_users_button(text: string, request_id: int, opts: map[string, json] = {}) -> map[string, json]:
    ru: map[string, json] = {"request_id": request_id};
    merge_into(ru, opts);
    return {"text": text, "request_users": ru};

pub fn request_chat_button(text: string, request_id: int, chat_is_channel: bool, opts: map[string, json] = {}) -> map[string, json]:
    rc: map[string, json] = {"request_id": request_id, "chat_is_channel": chat_is_channel};
    merge_into(rc, opts);
    return {"text": text, "request_chat": rc};

pub fn reply_keyboard(rows: json, opts: map[string, json] = {}) -> map[string, json]:
    p: map[string, json] = {"keyboard": rows};
    if "resize_keyboard" in opts:
        p["resize_keyboard"] = opts["resize_keyboard"];
    if "one_time_keyboard" in opts:
        p["one_time_keyboard"] = opts["one_time_keyboard"];
    if "selective" in opts:
        p["selective"] = opts["selective"];
    if "is_persistent" in opts:
        p["is_persistent"] = opts["is_persistent"];
    if "input_field_placeholder" in opts:
        p["input_field_placeholder"] = opts["input_field_placeholder"];
    return p;

pub fn keyboard(rows: json, opts: map[string, json] = {}) -> map[string, json]:
    return reply_keyboard(rows, opts);

pub fn remove_keyboard() -> map[string, json]:
    return {"remove_keyboard": true};

pub fn force_reply(selective: bool = false, input_field_placeholder: string = "") -> map[string, json]:
    p: map[string, json] = {"force_reply": true};
    if selective:
        p["selective"] = true;
    if input_field_placeholder != "":
        p["input_field_placeholder"] = input_field_placeholder;
    return p;

// ----------------------------------------------------------------------------
// Форматирование
// ----------------------------------------------------------------------------

pub fn escape_html(text: string) -> string:
    out := str.replace(text, "&", "&amp;");
    out = str.replace(out, "<", "&lt;");
    out = str.replace(out, ">", "&gt;");
    out = str.replace(out, "\"", "&quot;");
    return out;

pub fn escape_markdown_v2(text: string) -> string:
    out := text;
    for c in ["_", "*", "[", "]", "(", ")", "~", "`", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"]:
        out = str.replace(out, c, "\\" + c);
    return out;

pub fn escape_markdown(text: string) -> string:
    out := text;
    for c in ["_", "*", "[", "]", "`"]:
        out = str.replace(out, c, "\\" + c);
    return out;

pub fn bold(text: string) -> string:
    return "<b>" + text + "</b>";

pub fn italic(text: string) -> string:
    return "<i>" + text + "</i>";

pub fn underline(text: string) -> string:
    return "<u>" + text + "</u>";

pub fn strikethrough(text: string) -> string:
    return "<s>" + text + "</s>";

pub fn spoiler(text: string) -> string:
    return "<tg-spoiler>" + text + "</tg-spoiler>";

pub fn code(text: string) -> string:
    return "<code>" + escape_html(text) + "</code>";

pub fn pre(text: string, language: string = "") -> string:
    if language != "":
        return "<pre><code class=\"language-" + escape_html(language) + "\">" + escape_html(text) + "</code></pre>";
    return "<pre>" + escape_html(text) + "</pre>";

pub fn link(text: string, url: string) -> string:
    return "<a href=\"" + escape_html(url) + "\">" + text + "</a>";

pub fn mention(user_id: json, text: string) -> string:
    return "<a href=\"tg://user?id=" + string(user_id) + "\">" + text + "</a>";

pub fn custom_emoji(emoji_id: string, fallback: string = "🙂") -> string:
    return "<tg-emoji emoji-id=\"" + emoji_id + "\">" + fallback + "</tg-emoji>";

pub fn blockquote(text: string) -> string:
    return "<blockquote>" + text + "</blockquote>";

pub fn expandable_blockquote(text: string) -> string:
    return "<blockquote expandable>" + text + "</blockquote>";

pub fn split_text(text: string, limit: int = 4000) -> list[string]:
    if limit < 100:
        limit = 100;
    out: list[string] = [];
    rest := text;
    while str.length(rest) > limit:
        chunk := str.slice(rest, 0, limit);
        cut := str.last_index_of(chunk, "\n");
        if cut >= limit / 2:
            chunk = str.slice(rest, 0, cut);
            rest = str.slice(rest, cut + 1, str.length(rest));
        else:
            cut = str.last_index_of(chunk, " ");
            if cut >= limit / 2:
                chunk = str.slice(rest, 0, cut);
                rest = str.slice(rest, cut + 1, str.length(rest));
            else:
                rest = str.slice(rest, limit, str.length(rest));
        push(out, chunk);
    if str.length(rest) > 0 or len(out) == 0:
        push(out, rest);
    return out;

// ----------------------------------------------------------------------------
// Webhook
// ----------------------------------------------------------------------------

pub fn parse_webhook(body: string) -> json:
    return parse_body(body);

pub fn webhook_context(bot: Bot, body: string) -> json:
    update := parse_webhook(body);
    if update == null:
        err("invalid webhook JSON", code=400);
    return context(bot, update);

pub fn webhook_event(bot: Bot, body: string) -> Event:
    update := parse_webhook(body);
    if update == null:
        err("invalid webhook JSON", code=400);
    return process_update(bot, update);

pub fn check_webhook_secret(secret: string, received: string) -> bool:
    if secret == "":
        return true;
    return crypto.constant_time_equal(secret, received);

// ----------------------------------------------------------------------------
// Файлы
// ----------------------------------------------------------------------------

pub fn get_file(bot: Bot, file_id: string) -> json:
    return call(bot, "getFile", {"file_id": file_id});

pub fn file_url(bot: Bot, file_path: string) -> string:
    return bot.api_url + "/file/bot" + bot.token + "/" + file_path;

pub fn download_file(bot: Bot, file_id: string, destination: string, opts: map[string, json] = {}) -> string:
    f := get_file(bot, file_id);
    if f == null or f["file_path"] == null:
        err("file has no file_path", code=404);
    url := file_url(bot, string(f["file_path"]));
    headers: map[string, string] = {"User-Agent": "Alca-Telebot/1.0"};
    resp: json = urllib2.request("GET", url, null, headers, bot.timeout);
    if resp == null or resp["status"] == null:
        err_msg := "download failed";
        if resp != null and resp["msg"] != null:
            err_msg = err_msg + ": " + string(resp["msg"]);
        err_c := 500;
        if resp != null and resp["code"] != null:
            err_c = int(resp["code"]);
        err(err_msg, code=err_c);
    target := destination;
    if target == "":
        target = path.base(string(f["file_path"]));
    io.write_file(target, string(resp["body"]));
    return target;

pub fn download_file_path(bot: Bot, file_path: string, destination: string) -> string:
    url := file_url(bot, file_path);
    headers: map[string, string] = {"User-Agent": "Alca-Telebot/1.0"};
    resp: json = urllib2.request("GET", url, null, headers, bot.timeout);
    if resp == null or resp["status"] == null:
        err_msg := "download failed";
        if resp != null and resp["msg"] != null:
            err_msg = err_msg + ": " + string(resp["msg"]);
        err_c := 500;
        if resp != null and resp["code"] != null:
            err_c = int(resp["code"]);
        err(err_msg, code=err_c);
    target := destination;
    if target == "":
        target = path.base(file_path);
    io.write_file(target, string(resp["body"]));
    return target;

// ----------------------------------------------------------------------------
// Мелкие удобства
// ----------------------------------------------------------------------------

pub fn deep_link(username: string, payload: string = "") -> string:
    base := "https://t.me/" + username;
    if payload == "":
        return base;
    return base + "?start=" + urllib2.quote(payload);

pub fn message_link(chat_id: json, message_id: json) -> string:
    cid := string(chat_id);
    if str_starts_with(cid, "-100"):
        cid = str.slice(cid, 4, str.length(cid));
        return "https://t.me/c/" + cid + "/" + string(message_id);
    return "https://t.me/c/" + cid + "/" + string(message_id);

pub fn chat_id_of(ctx: json) -> json:
    return ctx["chat_id"];

pub fn text_of(ctx: json) -> string:
    if ctx["text"] == null:
        return "";
    return string(ctx["text"]);

pub fn args_text_of(ctx: json) -> string:
    if ctx["args_text"] == null:
        return "";
    return string(ctx["args_text"]);

// ----------------------------------------------------------------------------
// Сгенерированные обёртки всех методов Bot API 10.3
//
// Формат: telebot.<snake_case>(bot, <обязательные параметры...>, opts?)
// Необязательные параметры передаются в opts (map[string, json]).
// Локальные файлы в полях InputFile загружаются автоматически.
// ----------------------------------------------------------------------------

// setWebhook: Use this method to specify a URL and receive incoming updates via an outgoing webhook. Whenever there is an update for the bot, we will send an HTT...
pub fn set_webhook(bot: Bot, url: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"url": url};
    merge_into(p, opts);
    return call(bot, "setWebhook", p);

// deleteWebhook: Use this method to remove webhook integration if you decide to switch back to getUpdates. Returns True on success.
pub fn delete_webhook(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "deleteWebhook", p);

// getWebhookInfo: Use this method to get current webhook status. Requires no parameters. On success, returns a WebhookInfo object. If the bot is using getUpdates, wi...
pub fn get_webhook_info(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getWebhookInfo", p);

// logOut: Use this method to log out from the cloud Bot API server before launching the bot locally. You must log out the bot before running it locally, othe...
pub fn log_out(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "logOut", p);

// close: Use this method to close the bot instance before moving it from one local server to another. You need to delete the webhook before calling this met...
pub fn close(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "close", p);

// forwardMessage: Use this method to forward messages of any kind. Service messages and messages with protected content can't be forwarded. On success, the sent Mess...
pub fn forward_message(bot: Bot, chat_id: json, from_chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "from_chat_id": from_chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "forwardMessage", p);

// forwardMessages: Use this method to forward multiple messages of any kind. If some of the specified messages can't be found or forwarded, they are skipped. Service...
pub fn forward_messages(bot: Bot, chat_id: json, from_chat_id: json, message_ids: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "from_chat_id": from_chat_id, "message_ids": message_ids};
    merge_into(p, opts);
    return call(bot, "forwardMessages", p);

// copyMessage: Use this method to copy messages of any kind. Service messages, paid media messages, giveaway messages, giveaway winners messages, and invoice mess...
pub fn copy_message(bot: Bot, chat_id: json, from_chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "from_chat_id": from_chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "copyMessage", p);

// copyMessages: Use this method to copy messages of any kind. If some of the specified messages can't be found or copied, they are skipped. Service messages, paid...
pub fn copy_messages(bot: Bot, chat_id: json, from_chat_id: json, message_ids: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "from_chat_id": from_chat_id, "message_ids": message_ids};
    merge_into(p, opts);
    return call(bot, "copyMessages", p);

// sendPhoto: Use this method to send photos. On success, the sent Message is returned.
pub fn send_photo(bot: Bot, chat_id: json, photo: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "photo": photo};
    merge_into(p, opts);
    return call(bot, "sendPhoto", p);

// sendLivePhoto: Use this method to send live photos. On success, the sent Message is returned.
pub fn send_live_photo(bot: Bot, chat_id: json, live_photo: json, photo: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "live_photo": live_photo, "photo": photo};
    merge_into(p, opts);
    return call(bot, "sendLivePhoto", p);

// sendAudio: Use this method to send audio files, if you want Telegram clients to display them in the music player. Your audio must be in the .MP3 or .M4A forma...
pub fn send_audio(bot: Bot, chat_id: json, audio: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "audio": audio};
    merge_into(p, opts);
    return call(bot, "sendAudio", p);

// sendDocument: Use this method to send general files. On success, the sent Message is returned. Bots can currently send files of any type of up to 50 MB in size,...
pub fn send_document(bot: Bot, chat_id: json, document: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "document": document};
    merge_into(p, opts);
    return call(bot, "sendDocument", p);

// sendVideo: Use this method to send video files, Telegram clients support MPEG4 videos (other formats may be sent as Document). On success, the sent Message is...
pub fn send_video(bot: Bot, chat_id: json, video: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "video": video};
    merge_into(p, opts);
    return call(bot, "sendVideo", p);

// sendAnimation: Use this method to send animation files (GIF or H.264/MPEG-4 AVC video without sound). On success, the sent Message is returned. Bots can currently...
pub fn send_animation(bot: Bot, chat_id: json, animation: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "animation": animation};
    merge_into(p, opts);
    return call(bot, "sendAnimation", p);

// sendVoice: Use this method to send audio files, if you want Telegram clients to display the file as a playable voice message. For this to work, your audio mus...
pub fn send_voice(bot: Bot, chat_id: json, voice: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "voice": voice};
    merge_into(p, opts);
    return call(bot, "sendVoice", p);

// sendVideoNote: Use this method to send a rounded square MPEG4 video of up to 1 minute long. On success, the sent Message is returned.
pub fn send_video_note(bot: Bot, chat_id: json, video_note: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "video_note": video_note};
    merge_into(p, opts);
    return call(bot, "sendVideoNote", p);

// sendPaidMedia: Use this method to send paid media. On success, the sent Message is returned.
pub fn send_paid_media(bot: Bot, chat_id: json, star_count: json, media: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "star_count": star_count, "media": media};
    merge_into(p, opts);
    return call(bot, "sendPaidMedia", p);

// sendMediaGroup: Use this method to send a group of photos, live photos, videos, documents or audios as an album. Documents and audio files can be only grouped in a...
pub fn send_media_group(bot: Bot, chat_id: json, media: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "media": media};
    merge_into(p, opts);
    return call(bot, "sendMediaGroup", p);

// sendLocation: Use this method to send point on the map. On success, the sent Message is returned.
pub fn send_location(bot: Bot, chat_id: json, latitude: json, longitude: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "latitude": latitude, "longitude": longitude};
    merge_into(p, opts);
    return call(bot, "sendLocation", p);

// sendVenue: Use this method to send information about a venue. On success, the sent Message is returned.
pub fn send_venue(bot: Bot, chat_id: json, latitude: json, longitude: json, title: json, address: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "latitude": latitude, "longitude": longitude, "title": title, "address": address};
    merge_into(p, opts);
    return call(bot, "sendVenue", p);

// sendContact: Use this method to send phone contacts. On success, the sent Message is returned.
pub fn send_contact(bot: Bot, chat_id: json, phone_number: json, first_name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "phone_number": phone_number, "first_name": first_name};
    merge_into(p, opts);
    return call(bot, "sendContact", p);

// sendPoll: Use this method to send a native poll. On success, the sent Message is returned.
pub fn send_poll(bot: Bot, chat_id: json, question: json, options: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "question": question, "options": options};
    merge_into(p, opts);
    return call(bot, "sendPoll", p);

// sendChecklist: Use this method to send a checklist on behalf of a connected business account. On success, the sent Message is returned.
pub fn send_checklist(bot: Bot, business_connection_id: json, chat_id: json, checklist: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "chat_id": chat_id, "checklist": checklist};
    merge_into(p, opts);
    return call(bot, "sendChecklist", p);

// sendDice: Use this method to send an animated emoji that will display a random value. On success, the sent Message is returned.
pub fn send_dice(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "sendDice", p);

// sendMessageDraft: Use this method to stream a partial message to a user while the message is being generated. Note that the streamed draft is ephemeral and acts as a...
pub fn send_message_draft(bot: Bot, chat_id: json, draft_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "draft_id": draft_id};
    merge_into(p, opts);
    return call(bot, "sendMessageDraft", p);

// sendChatAction: Use this method when you need to tell the user that something is happening on the bot's side. The status is set for 5 seconds or less (when a messa...
pub fn send_chat_action(bot: Bot, chat_id: json, action: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "action": action};
    merge_into(p, opts);
    return call(bot, "sendChatAction", p);

// setMessageReaction: Use this method to change the chosen reactions on a message. Service messages of some types can't be reacted to. Automatically forwarded messages f...
pub fn set_message_reaction(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "setMessageReaction", p);

// getUserProfilePhotos: Use this method to get a list of profile pictures for a user. Returns a UserProfilePhotos object.
pub fn get_user_profile_photos(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getUserProfilePhotos", p);

// getUserProfileAudios: Use this method to get a list of profile audios for a user. Returns a UserProfileAudios object.
pub fn get_user_profile_audios(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getUserProfileAudios", p);

// setUserEmojiStatus: Changes the emoji status for a given user that previously allowed the bot to manage their emoji status via the Mini App method requestEmojiStatusAc...
pub fn set_user_emoji_status(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "setUserEmojiStatus", p);

// banChatMember: Use this method to ban a user in a group, a supergroup or a channel. In the case of supergroups and channels, the user will not be able to return t...
pub fn ban_chat_member(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "banChatMember", p);

// unbanChatMember: Use this method to unban a previously banned user in a supergroup or channel. The user will not return to the group or channel automatically, but w...
pub fn unban_chat_member(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "unbanChatMember", p);

// restrictChatMember: Use this method to restrict a user in a supergroup. The bot must be an administrator in the supergroup for this to work and must have the appropria...
pub fn restrict_chat_member(bot: Bot, chat_id: json, user_id: json, permissions: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id, "permissions": permissions};
    merge_into(p, opts);
    return call(bot, "restrictChatMember", p);

// promoteChatMember: Use this method to promote or demote a user in a supergroup or a channel. The bot must be an administrator in the chat for this to work and must ha...
pub fn promote_chat_member(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "promoteChatMember", p);

// setChatAdministratorCustomTitle: Use this method to set a custom title for an administrator in a supergroup promoted by the bot. Returns True on success.
pub fn set_chat_administrator_custom_title(bot: Bot, chat_id: json, user_id: json, custom_title: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id, "custom_title": custom_title};
    merge_into(p, opts);
    return call(bot, "setChatAdministratorCustomTitle", p);

// setChatMemberTag: Use this method to set a tag for a regular member in a group or a supergroup. The bot must be an administrator in the chat for this to work and mus...
pub fn set_chat_member_tag(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "setChatMemberTag", p);

// banChatSenderChat: Use this method to ban a channel chat in a supergroup or a channel. Until the chat is unbanned, the owner of the banned chat won't be able to send...
pub fn ban_chat_sender_chat(bot: Bot, chat_id: json, sender_chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "sender_chat_id": sender_chat_id};
    merge_into(p, opts);
    return call(bot, "banChatSenderChat", p);

// unbanChatSenderChat: Use this method to unban a previously banned channel chat in a supergroup or channel. The bot must be an administrator for this to work and must ha...
pub fn unban_chat_sender_chat(bot: Bot, chat_id: json, sender_chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "sender_chat_id": sender_chat_id};
    merge_into(p, opts);
    return call(bot, "unbanChatSenderChat", p);

// setChatPermissions: Use this method to set default chat permissions for all members. The bot must be an administrator in the group or a supergroup for this to work and...
pub fn set_chat_permissions(bot: Bot, chat_id: json, permissions: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "permissions": permissions};
    merge_into(p, opts);
    return call(bot, "setChatPermissions", p);

// exportChatInviteLink: Use this method to generate a new primary invite link for a chat; any previously generated primary link is revoked. The bot must be an administrato...
pub fn export_chat_invite_link(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "exportChatInviteLink", p);

// createChatInviteLink: Use this method to create an additional invite link for a chat. The bot must be an administrator in the chat for this to work and must have the app...
pub fn create_chat_invite_link(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "createChatInviteLink", p);

// editChatInviteLink: Use this method to edit a non-primary invite link created by the bot. The bot must be an administrator in the chat for this to work and must have t...
pub fn edit_chat_invite_link(bot: Bot, chat_id: json, invite_link: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "invite_link": invite_link};
    merge_into(p, opts);
    return call(bot, "editChatInviteLink", p);

// createChatSubscriptionInviteLink: Use this method to create a subscription invite link for a channel chat. The bot must have the can_invite_users administrator rights. The link can...
pub fn create_chat_subscription_invite_link(bot: Bot, chat_id: json, subscription_period: json, subscription_price: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "subscription_period": subscription_period, "subscription_price": subscription_price};
    merge_into(p, opts);
    return call(bot, "createChatSubscriptionInviteLink", p);

// editChatSubscriptionInviteLink: Use this method to edit a subscription invite link created by the bot. The bot must have the can_invite_users administrator rights. Returns the edi...
pub fn edit_chat_subscription_invite_link(bot: Bot, chat_id: json, invite_link: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "invite_link": invite_link};
    merge_into(p, opts);
    return call(bot, "editChatSubscriptionInviteLink", p);

// revokeChatInviteLink: Use this method to revoke an invite link created by the bot. If the primary link is revoked, a new link is automatically generated. The bot must be...
pub fn revoke_chat_invite_link(bot: Bot, chat_id: json, invite_link: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "invite_link": invite_link};
    merge_into(p, opts);
    return call(bot, "revokeChatInviteLink", p);

// approveChatJoinRequest: Use this method to approve a chat join request. The bot must be an administrator in the chat for this to work and must have the can_invite_users ad...
pub fn approve_chat_join_request(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "approveChatJoinRequest", p);

// declineChatJoinRequest: Use this method to decline a chat join request. The bot must be an administrator in the chat for this to work and must have the can_invite_users ad...
pub fn decline_chat_join_request(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "declineChatJoinRequest", p);

// answerChatJoinRequestQuery: Use this method to process a received chat join request query. Returns True on success.
pub fn answer_chat_join_request_query(bot: Bot, chat_join_request_query_id: json, result: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_join_request_query_id": chat_join_request_query_id, "result": result};
    merge_into(p, opts);
    return call(bot, "answerChatJoinRequestQuery", p);

// sendChatJoinRequestWebApp: Use this method to process a received chat join request query by showing a Mini App to the user before deciding the outcome. Call answerChatJoinReq...
pub fn send_chat_join_request_web_app(bot: Bot, chat_join_request_query_id: json, web_app_url: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_join_request_query_id": chat_join_request_query_id, "web_app_url": web_app_url};
    merge_into(p, opts);
    return call(bot, "sendChatJoinRequestWebApp", p);

// setChatPhoto: Use this method to set a new profile photo for the chat. Photos can't be changed for private chats. The bot must be an administrator in the chat fo...
pub fn set_chat_photo(bot: Bot, chat_id: json, photo: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "photo": photo};
    merge_into(p, opts);
    return call(bot, "setChatPhoto", p);

// deleteChatPhoto: Use this method to delete a chat photo. Photos can't be changed for private chats. The bot must be an administrator in the chat for this to work an...
pub fn delete_chat_photo(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "deleteChatPhoto", p);

// setChatTitle: Use this method to change the title of a chat. Titles can't be changed for private chats. The bot must be an administrator in the chat for this to...
pub fn set_chat_title(bot: Bot, chat_id: json, title: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "title": title};
    merge_into(p, opts);
    return call(bot, "setChatTitle", p);

// setChatDescription: Use this method to change the description of a group, a supergroup or a channel. The bot must be an administrator in the chat for this to work and...
pub fn set_chat_description(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "setChatDescription", p);

// pinChatMessage: Use this method to add a message to the list of pinned messages in a chat. In private chats and channel direct messages chats, all non-service mess...
pub fn pin_chat_message(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "pinChatMessage", p);

// unpinChatMessage: Use this method to remove a message from the list of pinned messages in a chat. In private chats and channel direct messages chats, all messages ca...
pub fn unpin_chat_message(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "unpinChatMessage", p);

// unpinAllChatMessages: Use this method to clear the list of pinned messages in a chat. In private chats and channel direct messages chats, no additional rights are requir...
pub fn unpin_all_chat_messages(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "unpinAllChatMessages", p);

// leaveChat: Use this method for your bot to leave a group, supergroup or channel. Returns True on success.
pub fn leave_chat(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "leaveChat", p);

// getChat: Use this method to get up-to-date information about the chat. Returns a ChatFullInfo object on success.
pub fn get_chat(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "getChat", p);

// getChatAdministrators: Use this method to get a list of administrators in a chat. Returns an Array of ChatMember objects.
pub fn get_chat_administrators(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "getChatAdministrators", p);

// getChatMemberCount: Use this method to get the number of members in a chat. Returns Integer on success.
pub fn get_chat_member_count(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "getChatMemberCount", p);

// getChatMember: Use this method to get information about a member of a chat. The method is only guaranteed to work for other users if the bot is an administrator i...
pub fn get_chat_member(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getChatMember", p);

// getUserPersonalChatMessages: Use this method to get the last messages from the personal chat (i.e., the chat currently added to their profile) of a given user. On success, an A...
pub fn get_user_personal_chat_messages(bot: Bot, user_id: json, limit: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "limit": limit};
    merge_into(p, opts);
    return call(bot, "getUserPersonalChatMessages", p);

// setChatStickerSet: Use this method to set a new group sticker set for a supergroup. The bot must be an administrator in the chat for this to work and must have the ap...
pub fn set_chat_sticker_set(bot: Bot, chat_id: json, sticker_set_name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "sticker_set_name": sticker_set_name};
    merge_into(p, opts);
    return call(bot, "setChatStickerSet", p);

// deleteChatStickerSet: Use this method to delete a group sticker set from a supergroup. The bot must be an administrator in the chat for this to work and must have the ap...
pub fn delete_chat_sticker_set(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "deleteChatStickerSet", p);

// getForumTopicIconStickers: Use this method to get custom emoji stickers, which can be used as a forum topic icon by any user. Requires no parameters. Returns an Array of Stic...
pub fn get_forum_topic_icon_stickers(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getForumTopicIconStickers", p);

// createForumTopic: Use this method to create a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot must be an adm...
pub fn create_forum_topic(bot: Bot, chat_id: json, name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "name": name};
    merge_into(p, opts);
    return call(bot, "createForumTopic", p);

// editForumTopic: Use this method to edit name and icon of a topic in a forum supergroup chat or a private chat with a user. In the case of a supergroup chat the bot...
pub fn edit_forum_topic(bot: Bot, chat_id: json, message_thread_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_thread_id": message_thread_id};
    merge_into(p, opts);
    return call(bot, "editForumTopic", p);

// closeForumTopic: Use this method to close an open topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have the...
pub fn close_forum_topic(bot: Bot, chat_id: json, message_thread_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_thread_id": message_thread_id};
    merge_into(p, opts);
    return call(bot, "closeForumTopic", p);

// reopenForumTopic: Use this method to reopen a closed topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have th...
pub fn reopen_forum_topic(bot: Bot, chat_id: json, message_thread_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_thread_id": message_thread_id};
    merge_into(p, opts);
    return call(bot, "reopenForumTopic", p);

// deleteForumTopic: Use this method to delete a forum topic along with all its messages in a forum supergroup chat or a private chat with a user. In the case of a supe...
pub fn delete_forum_topic(bot: Bot, chat_id: json, message_thread_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_thread_id": message_thread_id};
    merge_into(p, opts);
    return call(bot, "deleteForumTopic", p);

// unpinAllForumTopicMessages: Use this method to clear the list of pinned messages in a forum topic in a forum supergroup chat or a private chat with a user. In the case of a su...
pub fn unpin_all_forum_topic_messages(bot: Bot, chat_id: json, message_thread_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_thread_id": message_thread_id};
    merge_into(p, opts);
    return call(bot, "unpinAllForumTopicMessages", p);

// editGeneralForumTopic: Use this method to edit the name of the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work a...
pub fn edit_general_forum_topic(bot: Bot, chat_id: json, name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "name": name};
    merge_into(p, opts);
    return call(bot, "editGeneralForumTopic", p);

// closeGeneralForumTopic: Use this method to close an open 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must...
pub fn close_general_forum_topic(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "closeGeneralForumTopic", p);

// reopenGeneralForumTopic: Use this method to reopen a closed 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and mu...
pub fn reopen_general_forum_topic(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "reopenGeneralForumTopic", p);

// hideGeneralForumTopic: Use this method to hide the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must have...
pub fn hide_general_forum_topic(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "hideGeneralForumTopic", p);

// unhideGeneralForumTopic: Use this method to unhide the 'General' topic in a forum supergroup chat. The bot must be an administrator in the chat for this to work and must ha...
pub fn unhide_general_forum_topic(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "unhideGeneralForumTopic", p);

// unpinAllGeneralForumTopicMessages: Use this method to clear the list of pinned messages in a General forum topic. The bot must be an administrator in the chat for this to work and mu...
pub fn unpin_all_general_forum_topic_messages(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "unpinAllGeneralForumTopicMessages", p);

// answerCallbackQuery: Use this method to send answers to callback queries sent from inline keyboards. The answer will be displayed to the user as a notification at the t...
pub fn answer_callback_query(bot: Bot, callback_query_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"callback_query_id": callback_query_id};
    merge_into(p, opts);
    return call(bot, "answerCallbackQuery", p);

// answerGuestQuery: Use this method to reply to a received guest message. On success, a SentGuestMessage object is returned.
pub fn answer_guest_query(bot: Bot, guest_query_id: json, result: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"guest_query_id": guest_query_id, "result": result};
    merge_into(p, opts);
    return call(bot, "answerGuestQuery", p);

// getUserChatBoosts: Use this method to get the list of boosts added to a chat by a user. Requires administrator rights in the chat. Returns a UserChatBoosts object.
pub fn get_user_chat_boosts(bot: Bot, chat_id: json, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getUserChatBoosts", p);

// getBusinessConnection: Use this method to get information about the connection of the bot with a business account. Returns a BusinessConnection object on success.
pub fn get_business_connection(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "getBusinessConnection", p);

// getManagedBotToken: Use this method to get the token of a managed bot. Returns the token as String on success.
pub fn get_managed_bot_token(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getManagedBotToken", p);

// replaceManagedBotToken: Use this method to revoke the current token of a managed bot and generate a new one. Returns the new token as String on success.
pub fn replace_managed_bot_token(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "replaceManagedBotToken", p);

// getManagedBotAccessSettings: Use this method to get the access settings of a managed bot. Returns a BotAccessSettings object on success.
pub fn get_managed_bot_access_settings(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getManagedBotAccessSettings", p);

// setManagedBotAccessSettings: Use this method to change the access settings of a managed bot. Returns True on success.
pub fn set_managed_bot_access_settings(bot: Bot, user_id: json, is_access_restricted: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "is_access_restricted": is_access_restricted};
    merge_into(p, opts);
    return call(bot, "setManagedBotAccessSettings", p);

// setMyCommands: Use this method to change the list of the bot's commands. See this manual for more details about bot commands. Returns True on success.
pub fn set_my_commands(bot: Bot, commands: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"commands": commands};
    merge_into(p, opts);
    return call(bot, "setMyCommands", p);

// deleteMyCommands: Use this method to delete the list of the bot's commands for the given scope and user language. After deletion, higher level commands will be shown...
pub fn delete_my_commands(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "deleteMyCommands", p);

// getMyCommands: Use this method to get the current list of the bot's commands for the given scope and user language. Returns an Array of BotCommand objects. If com...
pub fn get_my_commands(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyCommands", p);

// setMyName: Use this method to change the bot's name. Returns True on success.
pub fn set_my_name(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "setMyName", p);

// getMyName: Use this method to get the current bot name for the given user language. Returns BotName on success.
pub fn get_my_name(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyName", p);

// setMyDescription: Use this method to change the bot's description, which is shown in the chat with the bot if the chat is empty. Returns True on success.
pub fn set_my_description(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "setMyDescription", p);

// getMyDescription: Use this method to get the current bot description for the given user language. Returns BotDescription on success.
pub fn get_my_description(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyDescription", p);

// setMyShortDescription: Use this method to change the bot's short description, which is shown on the bot's profile page and is sent together with the link when users share...
pub fn set_my_short_description(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "setMyShortDescription", p);

// getMyShortDescription: Use this method to get the current bot short description for the given user language. Returns BotShortDescription on success.
pub fn get_my_short_description(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyShortDescription", p);

// setMyProfilePhoto: Changes the profile photo of the bot. Returns True on success.
pub fn set_my_profile_photo(bot: Bot, photo: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"photo": photo};
    merge_into(p, opts);
    return call(bot, "setMyProfilePhoto", p);

// removeMyProfilePhoto: Removes the profile photo of the bot. Requires no parameters. Returns True on success.
pub fn remove_my_profile_photo(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "removeMyProfilePhoto", p);

// setChatMenuButton: Use this method to change the bot's menu button in a private chat, or the default menu button. Returns True on success.
pub fn set_chat_menu_button(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "setChatMenuButton", p);

// getChatMenuButton: Use this method to get the current value of the bot's menu button in a private chat, or the default menu button. Returns MenuButton on success.
pub fn get_chat_menu_button(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getChatMenuButton", p);

// setMyDefaultAdministratorRights: Use this method to change the default administrator rights requested by the bot when it's added as an administrator to groups or channels. These ri...
pub fn set_my_default_administrator_rights(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "setMyDefaultAdministratorRights", p);

// getMyDefaultAdministratorRights: Use this method to get the current default administrator rights of the bot. Returns ChatAdministratorRights on success.
pub fn get_my_default_administrator_rights(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyDefaultAdministratorRights", p);

// getAvailableGifts: Returns the list of gifts that can be sent by the bot to users and channel chats. Requires no parameters. Returns a Gifts object.
pub fn get_available_gifts(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getAvailableGifts", p);

// sendGift: Sends a gift to the given user or channel chat. The gift can't be converted to Telegram Stars by the receiver. Returns True on success.
pub fn send_gift(bot: Bot, gift_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"gift_id": gift_id};
    merge_into(p, opts);
    return call(bot, "sendGift", p);

// giftPremiumSubscription: Gifts a Telegram Premium subscription to the given user. Returns True on success.
pub fn gift_premium_subscription(bot: Bot, user_id: json, month_count: json, star_count: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "month_count": month_count, "star_count": star_count};
    merge_into(p, opts);
    return call(bot, "giftPremiumSubscription", p);

// verifyUser: Verifies a user on behalf of the organization which is represented by the bot. Returns True on success.
pub fn verify_user(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "verifyUser", p);

// verifyChat: Verifies a chat on behalf of the organization which is represented by the bot. Returns True on success.
pub fn verify_chat(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "verifyChat", p);

// removeUserVerification: Removes verification from a user who is currently verified on behalf of the organization represented by the bot. Returns True on success.
pub fn remove_user_verification(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "removeUserVerification", p);

// removeChatVerification: Removes verification from a chat that is currently verified on behalf of the organization represented by the bot. Returns True on success.
pub fn remove_chat_verification(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "removeChatVerification", p);

// readBusinessMessage: Marks incoming message as read on behalf of a business account. Requires the can_read_messages business bot right. Returns True on success.
pub fn read_business_message(bot: Bot, business_connection_id: json, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "readBusinessMessage", p);

// deleteBusinessMessages: Delete messages on behalf of a business account. Requires the can_delete_sent_messages business bot right to delete messages sent by the bot itself...
pub fn delete_business_messages(bot: Bot, business_connection_id: json, message_ids: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "message_ids": message_ids};
    merge_into(p, opts);
    return call(bot, "deleteBusinessMessages", p);

// setBusinessAccountName: Changes the first and last name of a managed business account. Requires the can_change_name business bot right. Returns True on success.
pub fn set_business_account_name(bot: Bot, business_connection_id: json, first_name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "first_name": first_name};
    merge_into(p, opts);
    return call(bot, "setBusinessAccountName", p);

// setBusinessAccountUsername: Changes the username of a managed business account. Requires the can_change_username business bot right. Returns True on success.
pub fn set_business_account_username(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "setBusinessAccountUsername", p);

// setBusinessAccountBio: Changes the bio of a managed business account. Requires the can_change_bio business bot right. Returns True on success.
pub fn set_business_account_bio(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "setBusinessAccountBio", p);

// setBusinessAccountProfilePhoto: Changes the profile photo of a managed business account. Requires the can_edit_profile_photo business bot right. Returns True on success.
pub fn set_business_account_profile_photo(bot: Bot, business_connection_id: json, photo: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "photo": photo};
    merge_into(p, opts);
    return call(bot, "setBusinessAccountProfilePhoto", p);

// removeBusinessAccountProfilePhoto: Removes the current profile photo of a managed business account. Requires the can_edit_profile_photo business bot right. Returns True on success.
pub fn remove_business_account_profile_photo(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "removeBusinessAccountProfilePhoto", p);

// setBusinessAccountGiftSettings: Changes the privacy settings pertaining to incoming gifts in a managed business account. Requires the can_change_gift_settings business bot right....
pub fn set_business_account_gift_settings(bot: Bot, business_connection_id: json, show_gift_button: json, accepted_gift_types: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "show_gift_button": show_gift_button, "accepted_gift_types": accepted_gift_types};
    merge_into(p, opts);
    return call(bot, "setBusinessAccountGiftSettings", p);

// getBusinessAccountStarBalance: Returns the amount of Telegram Stars owned by a managed business account. Requires the can_view_gifts_and_stars business bot right. Returns StarAmo...
pub fn get_business_account_star_balance(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "getBusinessAccountStarBalance", p);

// transferBusinessAccountStars: Transfers Telegram Stars from the business account balance to the bot's balance. Requires the can_transfer_stars business bot right. Returns True o...
pub fn transfer_business_account_stars(bot: Bot, business_connection_id: json, star_count: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "star_count": star_count};
    merge_into(p, opts);
    return call(bot, "transferBusinessAccountStars", p);

// getBusinessAccountGifts: Returns the gifts received and owned by a managed business account. Requires the can_view_gifts_and_stars business bot right. Returns OwnedGifts on...
pub fn get_business_account_gifts(bot: Bot, business_connection_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id};
    merge_into(p, opts);
    return call(bot, "getBusinessAccountGifts", p);

// getUserGifts: Returns the gifts owned and hosted by a user. Returns OwnedGifts on success.
pub fn get_user_gifts(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getUserGifts", p);

// getChatGifts: Returns the gifts owned by a chat. Returns OwnedGifts on success.
pub fn get_chat_gifts(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "getChatGifts", p);

// convertGiftToStars: Converts a given regular gift to Telegram Stars. Requires the can_convert_gifts_to_stars business bot right. Returns True on success.
pub fn convert_gift_to_stars(bot: Bot, business_connection_id: json, owned_gift_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "owned_gift_id": owned_gift_id};
    merge_into(p, opts);
    return call(bot, "convertGiftToStars", p);

// upgradeGift: Upgrades a given regular gift to a unique gift. Requires the can_transfer_and_upgrade_gifts business bot right. Additionally requires the can_trans...
pub fn upgrade_gift(bot: Bot, business_connection_id: json, owned_gift_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "owned_gift_id": owned_gift_id};
    merge_into(p, opts);
    return call(bot, "upgradeGift", p);

// transferGift: Transfers an owned unique gift to another user. Requires the can_transfer_and_upgrade_gifts business bot right. Requires can_transfer_stars busines...
pub fn transfer_gift(bot: Bot, business_connection_id: json, owned_gift_id: json, new_owner_chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "owned_gift_id": owned_gift_id, "new_owner_chat_id": new_owner_chat_id};
    merge_into(p, opts);
    return call(bot, "transferGift", p);

// postStory: Posts a story on behalf of a managed business account. Requires the can_manage_stories business bot right. Returns Story on success.
pub fn post_story(bot: Bot, business_connection_id: json, content: json, active_period: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "content": content, "active_period": active_period};
    merge_into(p, opts);
    return call(bot, "postStory", p);

// repostStory: Reposts a story on behalf of a business account from another business account. Both business accounts must be managed by the same bot, and the stor...
pub fn repost_story(bot: Bot, business_connection_id: json, from_chat_id: json, from_story_id: json, active_period: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "from_chat_id": from_chat_id, "from_story_id": from_story_id, "active_period": active_period};
    merge_into(p, opts);
    return call(bot, "repostStory", p);

// editStory: Edits a story previously posted by the bot on behalf of a managed business account. Requires the can_manage_stories business bot right. Returns Sto...
pub fn edit_story(bot: Bot, business_connection_id: json, story_id: json, content: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "story_id": story_id, "content": content};
    merge_into(p, opts);
    return call(bot, "editStory", p);

// deleteStory: Deletes a story previously posted by the bot on behalf of a managed business account. Requires the can_manage_stories business bot right. Returns T...
pub fn delete_story(bot: Bot, business_connection_id: json, story_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "story_id": story_id};
    merge_into(p, opts);
    return call(bot, "deleteStory", p);

// answerWebAppQuery: Use this method to set the result of an interaction with a Web App and send a corresponding message on behalf of the user to the chat from which th...
pub fn answer_web_app_query(bot: Bot, web_app_query_id: json, result: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"web_app_query_id": web_app_query_id, "result": result};
    merge_into(p, opts);
    return call(bot, "answerWebAppQuery", p);

// savePreparedInlineMessage: Stores a message that can be sent by a user of a Mini App. Returns a PreparedInlineMessage object.
pub fn save_prepared_inline_message(bot: Bot, user_id: json, result: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "result": result};
    merge_into(p, opts);
    return call(bot, "savePreparedInlineMessage", p);

// savePreparedKeyboardButton: Stores a keyboard button that can be used by a user within a Mini App. Returns a PreparedKeyboardButton object.
pub fn save_prepared_keyboard_button(bot: Bot, user_id: json, button: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "button": button};
    merge_into(p, opts);
    return call(bot, "savePreparedKeyboardButton", p);

// editMessageCaption: Use this method to edit captions of messages. On success, if the edited message is not an inline message, the edited Message is returned, otherwise...
pub fn edit_message_caption(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "editMessageCaption", p);

// editMessageMedia: Use this method to edit animation, audio, document, live photo, photo, or video messages, or to replace a text or a rich message with a media. If a...
pub fn edit_message_media(bot: Bot, media: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"media": media};
    merge_into(p, opts);
    return call(bot, "editMessageMedia", p);

// editMessageLiveLocation: Use this method to edit live location messages. A location can be edited until its live_period expires or editing is explicitly disabled by a call...
pub fn edit_message_live_location(bot: Bot, latitude: json, longitude: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"latitude": latitude, "longitude": longitude};
    merge_into(p, opts);
    return call(bot, "editMessageLiveLocation", p);

// stopMessageLiveLocation: Use this method to stop updating a live location message before live_period expires. On success, if the message is not an inline message, the edite...
pub fn stop_message_live_location(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "stopMessageLiveLocation", p);

// editMessageChecklist: Use this method to edit a checklist on behalf of a connected business account. On success, the edited Message is returned.
pub fn edit_message_checklist(bot: Bot, business_connection_id: json, chat_id: json, message_id: json, checklist: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"business_connection_id": business_connection_id, "chat_id": chat_id, "message_id": message_id, "checklist": checklist};
    merge_into(p, opts);
    return call(bot, "editMessageChecklist", p);

// editMessageReplyMarkup: Use this method to edit only the reply markup of messages. On success, if the edited message is not an inline message, the edited Message is return...
pub fn edit_message_reply_markup(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "editMessageReplyMarkup", p);

// stopPoll: Use this method to stop a poll which was sent by the bot. On success, the stopped Poll is returned.
pub fn stop_poll(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "stopPoll", p);

// editEphemeralMessageText: Use this method to edit an ephemeral text or rich message. Note that it is not guaranteed that the user will receive the message edit event, especi...
pub fn edit_ephemeral_message_text(bot: Bot, chat_id: json, receiver_user_id: json, ephemeral_message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "receiver_user_id": receiver_user_id, "ephemeral_message_id": ephemeral_message_id};
    merge_into(p, opts);
    return call(bot, "editEphemeralMessageText", p);

// editEphemeralMessageMedia: Use this method to edit the media of an ephemeral message. Note that it is not guaranteed that the user will receive the message edit event, especi...
pub fn edit_ephemeral_message_media(bot: Bot, chat_id: json, receiver_user_id: json, ephemeral_message_id: json, media: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "receiver_user_id": receiver_user_id, "ephemeral_message_id": ephemeral_message_id, "media": media};
    merge_into(p, opts);
    return call(bot, "editEphemeralMessageMedia", p);

// editEphemeralMessageCaption: Use this method to edit the caption of an ephemeral message. Note that it is not guaranteed that the user will receive the message edit event, espe...
pub fn edit_ephemeral_message_caption(bot: Bot, chat_id: json, receiver_user_id: json, ephemeral_message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "receiver_user_id": receiver_user_id, "ephemeral_message_id": ephemeral_message_id};
    merge_into(p, opts);
    return call(bot, "editEphemeralMessageCaption", p);

// editEphemeralMessageReplyMarkup: Use this method to edit only the reply markup of an ephemeral message. Note that it is not guaranteed that the user will receive the message edit e...
pub fn edit_ephemeral_message_reply_markup(bot: Bot, chat_id: json, receiver_user_id: json, ephemeral_message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "receiver_user_id": receiver_user_id, "ephemeral_message_id": ephemeral_message_id};
    merge_into(p, opts);
    return call(bot, "editEphemeralMessageReplyMarkup", p);

// approveSuggestedPost: Use this method to approve a suggested post in a direct messages chat. The bot must have the 'can_post_messages' administrator right in the corresp...
pub fn approve_suggested_post(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "approveSuggestedPost", p);

// declineSuggestedPost: Use this method to decline a suggested post in a direct messages chat. The bot must have the 'can_manage_direct_messages' administrator right in th...
pub fn decline_suggested_post(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "declineSuggestedPost", p);

// deleteMessage: Use this method to delete a message, including service messages, with the following limitations: - A message can only be deleted if it was sent les...
pub fn delete_message(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "deleteMessage", p);

// deleteMessages: Use this method to delete multiple messages simultaneously. If some of the specified messages can't be found, they are skipped. Returns True on suc...
pub fn delete_messages(bot: Bot, chat_id: json, message_ids: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_ids": message_ids};
    merge_into(p, opts);
    return call(bot, "deleteMessages", p);

// deleteEphemeralMessage: Use this method to delete an ephemeral message. Note that it is not guaranteed that the user will receive the message deletion event, especially if...
pub fn delete_ephemeral_message(bot: Bot, chat_id: json, receiver_user_id: json, ephemeral_message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "receiver_user_id": receiver_user_id, "ephemeral_message_id": ephemeral_message_id};
    merge_into(p, opts);
    return call(bot, "deleteEphemeralMessage", p);

// deleteMessageReaction: Use this method to remove a reaction from a message in a group or a supergroup chat. The bot must have the 'can_delete_messages' administrator righ...
pub fn delete_message_reaction(bot: Bot, chat_id: json, message_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "message_id": message_id};
    merge_into(p, opts);
    return call(bot, "deleteMessageReaction", p);

// deleteAllMessageReactions: Use this method to remove up to 10000 recent reactions in a group or a supergroup chat added by a given user or chat. The bot must have the 'can_de...
pub fn delete_all_message_reactions(bot: Bot, chat_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id};
    merge_into(p, opts);
    return call(bot, "deleteAllMessageReactions", p);

// sendSticker: Use this method to send static .WEBP, animated .TGS, or video .WEBM stickers. On success, the sent Message is returned.
pub fn send_sticker(bot: Bot, chat_id: json, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "sticker": sticker};
    merge_into(p, opts);
    return call(bot, "sendSticker", p);

// getStickerSet: Use this method to get a sticker set. On success, a StickerSet object is returned.
pub fn get_sticker_set(bot: Bot, name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"name": name};
    merge_into(p, opts);
    return call(bot, "getStickerSet", p);

// getCustomEmojiStickers: Use this method to get information about custom emoji stickers by their identifiers. Returns an Array of Sticker objects.
pub fn get_custom_emoji_stickers(bot: Bot, custom_emoji_ids: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"custom_emoji_ids": custom_emoji_ids};
    merge_into(p, opts);
    return call(bot, "getCustomEmojiStickers", p);

// uploadStickerFile: Use this method to upload a file with a sticker for later use in the createNewStickerSet, addStickerToSet, or replaceStickerInSet methods (the file...
pub fn upload_sticker_file(bot: Bot, user_id: json, sticker: json, sticker_format: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "sticker": sticker, "sticker_format": sticker_format};
    merge_into(p, opts);
    return call(bot, "uploadStickerFile", p);

// createNewStickerSet: Use this method to create a new sticker set owned by a user. The bot will be able to edit the sticker set thus created. Returns True on success.
pub fn create_new_sticker_set(bot: Bot, user_id: json, name: json, title: json, stickers: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "name": name, "title": title, "stickers": stickers};
    merge_into(p, opts);
    return call(bot, "createNewStickerSet", p);

// addStickerToSet: Use this method to add a new sticker to a set created by the bot. Emoji sticker sets can have up to 200 stickers. Other sticker sets can have up to...
pub fn add_sticker_to_set(bot: Bot, user_id: json, name: json, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "name": name, "sticker": sticker};
    merge_into(p, opts);
    return call(bot, "addStickerToSet", p);

// setStickerPositionInSet: Use this method to move a sticker in a set created by the bot to a specific position. Returns True on success.
pub fn set_sticker_position_in_set(bot: Bot, sticker: json, position: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"sticker": sticker, "position": position};
    merge_into(p, opts);
    return call(bot, "setStickerPositionInSet", p);

// deleteStickerFromSet: Use this method to delete a sticker from a set created by the bot. Returns True on success.
pub fn delete_sticker_from_set(bot: Bot, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"sticker": sticker};
    merge_into(p, opts);
    return call(bot, "deleteStickerFromSet", p);

// replaceStickerInSet: Use this method to replace an existing sticker in a sticker set with a new one. The method is equivalent to calling deleteStickerFromSet, then addS...
pub fn replace_sticker_in_set(bot: Bot, user_id: json, name: json, old_sticker: json, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "name": name, "old_sticker": old_sticker, "sticker": sticker};
    merge_into(p, opts);
    return call(bot, "replaceStickerInSet", p);

// setStickerEmojiList: Use this method to change the list of emoji assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the...
pub fn set_sticker_emoji_list(bot: Bot, sticker: json, emoji_list: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"sticker": sticker, "emoji_list": emoji_list};
    merge_into(p, opts);
    return call(bot, "setStickerEmojiList", p);

// setStickerKeywords: Use this method to change search keywords assigned to a regular or custom emoji sticker. The sticker must belong to a sticker set created by the bo...
pub fn set_sticker_keywords(bot: Bot, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"sticker": sticker};
    merge_into(p, opts);
    return call(bot, "setStickerKeywords", p);

// setStickerMaskPosition: Use this method to change the mask position of a mask sticker. The sticker must belong to a sticker set that was created by the bot. Returns True o...
pub fn set_sticker_mask_position(bot: Bot, sticker: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"sticker": sticker};
    merge_into(p, opts);
    return call(bot, "setStickerMaskPosition", p);

// setStickerSetTitle: Use this method to set the title of a created sticker set. Returns True on success.
pub fn set_sticker_set_title(bot: Bot, name: json, title: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"name": name, "title": title};
    merge_into(p, opts);
    return call(bot, "setStickerSetTitle", p);

// setStickerSetThumbnail: Use this method to set the thumbnail of a regular or mask sticker set. The format of the thumbnail file must match the format of the stickers in th...
pub fn set_sticker_set_thumbnail(bot: Bot, name: json, user_id: json, format: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"name": name, "user_id": user_id, "format": format};
    merge_into(p, opts);
    return call(bot, "setStickerSetThumbnail", p);

// setCustomEmojiStickerSetThumbnail: Use this method to set the thumbnail of a custom emoji sticker set. Returns True on success.
pub fn set_custom_emoji_sticker_set_thumbnail(bot: Bot, name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"name": name};
    merge_into(p, opts);
    return call(bot, "setCustomEmojiStickerSetThumbnail", p);

// deleteStickerSet: Use this method to delete a sticker set that was created by the bot. Returns True on success.
pub fn delete_sticker_set(bot: Bot, name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"name": name};
    merge_into(p, opts);
    return call(bot, "deleteStickerSet", p);

// sendRichMessage: Use this method to send rich messages. If the message contains a block with a media element, then the bot must have the right to send the media to...
pub fn send_rich_message(bot: Bot, chat_id: json, rich_message: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "rich_message": rich_message};
    merge_into(p, opts);
    return call(bot, "sendRichMessage", p);

// sendRichMessageDraft: Use this method to stream a partial rich message to a user while the message is being generated. Note that the streamed draft is ephemeral and acts...
pub fn send_rich_message_draft(bot: Bot, chat_id: json, draft_id: json, rich_message: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "draft_id": draft_id, "rich_message": rich_message};
    merge_into(p, opts);
    return call(bot, "sendRichMessageDraft", p);

// answerInlineQuery: Use this method to send answers to an inline query. On success, True is returned. No more than 50 results per query are allowed.
pub fn answer_inline_query(bot: Bot, inline_query_id: json, results: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"inline_query_id": inline_query_id, "results": results};
    merge_into(p, opts);
    return call(bot, "answerInlineQuery", p);

// sendInvoice: Use this method to send invoices. On success, the sent Message is returned.
pub fn send_invoice(bot: Bot, chat_id: json, title: json, description: json, payload: json, currency: json, prices: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "title": title, "description": description, "payload": payload, "currency": currency, "prices": prices};
    merge_into(p, opts);
    return call(bot, "sendInvoice", p);

// createInvoiceLink: Use this method to create a link for an invoice. Returns the created invoice link as String on success.
pub fn create_invoice_link(bot: Bot, title: json, description: json, payload: json, currency: json, prices: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"title": title, "description": description, "payload": payload, "currency": currency, "prices": prices};
    merge_into(p, opts);
    return call(bot, "createInvoiceLink", p);

// answerShippingQuery: If you sent an invoice requesting a shipping address and the parameter is_flexible was specified, the Bot API will send an Update with a shipping_q...
pub fn answer_shipping_query(bot: Bot, shipping_query_id: json, ok: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"shipping_query_id": shipping_query_id, "ok": ok};
    merge_into(p, opts);
    return call(bot, "answerShippingQuery", p);

// answerPreCheckoutQuery: Once the user has confirmed their payment and shipping details, the Bot API sends the final confirmation in the form of an Update with the field pr...
pub fn answer_pre_checkout_query(bot: Bot, pre_checkout_query_id: json, ok: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"pre_checkout_query_id": pre_checkout_query_id, "ok": ok};
    merge_into(p, opts);
    return call(bot, "answerPreCheckoutQuery", p);

// getMyStarBalance: A method to get the current Telegram Stars balance of the bot. Requires no parameters. On success, returns a StarAmount object.
pub fn get_my_star_balance(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getMyStarBalance", p);

// getStarTransactions: Returns the bot's Telegram Star transactions in chronological order. On success, returns a StarTransactions object.
pub fn get_star_transactions(bot: Bot, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {};
    merge_into(p, opts);
    return call(bot, "getStarTransactions", p);

// refundStarPayment: Refunds a successful payment in Telegram Stars. Returns True on success.
pub fn refund_star_payment(bot: Bot, user_id: json, telegram_payment_charge_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "telegram_payment_charge_id": telegram_payment_charge_id};
    merge_into(p, opts);
    return call(bot, "refundStarPayment", p);

// editUserStarSubscription: Allows the bot to cancel or re-enable extension of a subscription paid in Telegram Stars. Returns True on success.
pub fn edit_user_star_subscription(bot: Bot, user_id: json, telegram_payment_charge_id: json, is_canceled: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "telegram_payment_charge_id": telegram_payment_charge_id, "is_canceled": is_canceled};
    merge_into(p, opts);
    return call(bot, "editUserStarSubscription", p);

// setPassportDataErrors: Informs a user that some of the Telegram Passport elements they provided contains errors. The user will not be able to re-submit their Passport to...
pub fn set_passport_data_errors(bot: Bot, user_id: json, errors: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "errors": errors};
    merge_into(p, opts);
    return call(bot, "setPassportDataErrors", p);

// sendGame: Use this method to send a game. On success, the sent Message is returned.
pub fn send_game(bot: Bot, chat_id: json, game_short_name: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"chat_id": chat_id, "game_short_name": game_short_name};
    merge_into(p, opts);
    return call(bot, "sendGame", p);

// setGameScore: Use this method to set the score of the specified user in a game message. On success, if the message is not an inline message, the Message is retur...
pub fn set_game_score(bot: Bot, user_id: json, score: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id, "score": score};
    merge_into(p, opts);
    return call(bot, "setGameScore", p);

// getGameHighScores: Use this method to get data for high score tables. Will return the score of the specified user and several of their neighbors in a game. Returns an...
pub fn get_game_high_scores(bot: Bot, user_id: json, opts: map[string, json] = {}) -> json:
    p: map[string, json] = {"user_id": user_id};
    merge_into(p, opts);
    return call(bot, "getGameHighScores", p);


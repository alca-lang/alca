fn msg_has(msg: json, field: string) -> bool {
    if msg == null {
        return false
    }
    return msg[field] != null
}
}

fn command_of(text: string) -> string {
    if not str_starts_with(text, "/") {
        return ""
    }
    words := str.words(text)
    if len(words) == 0 {
        return ""
    }
    first := words[0]
    cmd := str.slice(first, 1, str.length(first))
    at := str.index_of(cmd, "@")
    if at >= 0 {
        cmd = str.slice(cmd, 0, at)
    }
    return cmd
}
}

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
}

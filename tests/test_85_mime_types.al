// Copyright 2026 Pawl Programming Language
// Test 85: MIME types - MIME detection and handling.

// 1. Simple MIME detection by extension
func get_mime_type(filename: string) -> string {
    ext := ""
    if string.contains(filename, ".") {
        parts := string.split(filename, ".")
        ext := parts[len(parts) - 1]
    }
    mime_types := {
        "txt": "text/plain",
        "html": "text/html",
        "json": "application/json",
        "xml": "application/xml",
        "pdf": "application/pdf",
        "png": "image/png",
        "jpg": "image/jpeg",
        "gif": "image/gif",
        "mp3": "audio/mpeg",
        "mp4": "video/mp4"
    }
    if ext in mime_types {
        return mime_types[ext]
    }
    return "application/octet-stream"
}
assert get_mime_type("file.txt") == "text/plain"
assert get_mime_type("file.png") == "image/png"

// 2. MIME validation
func is_valid_mime(mime: string) -> bool {
    return string.contains(mime, "/")
}
assert is_valid_mime("text/plain")
assert !is_valid_mime("invalid")

// 3. Text MIME types
func is_text_mime(mime: string) -> bool {
    return string.starts_with(mime, "text/")
}
assert is_text_mime("text/plain")
assert !is_text_mime("image/png")

// 4. Image MIME types
func is_image_mime(mime: string) -> bool {
    return string.starts_with(mime, "image/")
}
assert is_image_mime("image/png")
assert !is_image_mime("text/plain")

// 5. Audio MIME types
func is_audio_mime(mime: string) -> bool {
    return string.starts_with(mime, "audio/")
}
assert is_audio_mime("audio/mpeg")
assert !is_audio_mime("text/plain")

// 6. Video MIME types
func is_video_mime(mime: string) -> bool {
    return string.starts_with(mime, "video/")
}
assert is_video_mime("video/mp4")
assert !is_video_mime("text/plain")

// 7. Application MIME types
func is_application_mime(mime: string) -> bool {
    return string.starts_with(mime, "application/")
}
assert is_application_mime("application/json")
assert !is_application_mime("text/plain")

// 8. MIME category
func get_mime_category(mime: string) -> string {
    if string.starts_with(mime, "text/") {
        return "text"
    }
    if string.starts_with(mime, "image/") {
        return "image"
    }
    if string.starts_with(mime, "audio/") {
        return "audio"
    }
    if string.starts_with(mime, "video/") {
        return "video"
    }
    if string.starts_with(mime, "application/") {
        return "application"
    }
    return "unknown"
}
assert get_mime_category("text/plain") == "text"
assert get_mime_category("image/png") == "image"

// 9. Common MIME types list
func common_mime_types() -> list[string] {
    return [
        "text/plain",
        "text/html",
        "application/json",
        "application/xml",
        "image/png",
        "image/jpeg",
        "image/gif",
        "application/pdf"
    ]
}
mimes := common_mime_types()
assert len(mimes) == 8

// 10. MIME to extension
func mime_to_extension(mime: string) -> string {
    ext_map := {
        "text/plain": "txt",
        "text/html": "html",
        "application/json": "json",
        "image/png": "png",
        "image/jpeg": "jpg"
    }
    if mime in ext_map {
        return ext_map[mime]
    }
    return "bin"
}
assert mime_to_extension("text/plain") == "txt"
assert mime_to_extension("image/png") == "png"

println("✓ test_85_mime_types passed")

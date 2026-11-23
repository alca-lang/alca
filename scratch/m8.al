use std.json
use std.io
use std.path
use std.crypto
use std.re

pub fn js(x: string) -> json {
    return json.parse(x)
}
pub fn exist(x: string) -> bool {
    return io.file_exists(x)
}
pub fn base(x: string) -> string {
    return path.base(x)
}
pub fn hash(x: string) -> string {
    return crypto.sha256(x)
}
pub fn rematch(x: string) -> bool {
    return re.match("^a", x)
}

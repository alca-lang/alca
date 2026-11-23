struct Router {
    prefix: string = ''
    handlers: list = []
}
r := Router()
println('empty router prefix:', r.prefix, len(r.handlers))

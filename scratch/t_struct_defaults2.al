struct Router {
    prefix: string = ''
    handlers: list = []
}
r := Router('/admin')
println(r.prefix, len(r.handlers))

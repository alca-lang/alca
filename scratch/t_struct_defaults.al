struct Bot {
    token: string
    timeout: int = 30
    middlewares: list = []
}
b := Bot('123')
println(b.token, b.timeout, len(b.middlewares))

use "./m5.al"

f := |x| x + 1
println(f(1))
hs := [m5.H(|x| x + 1)]
for h in hs {
    println(h.cb(2))
}

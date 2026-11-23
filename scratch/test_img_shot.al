use std.image

img := image.screenshot()
image.write(img, "scratch/gdi_shot.png")
println("saved gdi_shot.png")

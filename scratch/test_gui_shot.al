use std.gui
e: error | null = null
res := gui.screenshot("scratch/test_pawl_shot.png") ?>>> e
if e != null {
    println("ERROR OCCURRED:", e)
}
else {
    println("SUCCESS:", res)
}

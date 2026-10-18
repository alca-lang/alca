use std.gui

size = gui.screen_size()
print("Detected Screen Size:")
print("Width:  " + string(size["width"]))
print("Height: " + string(size["height"]))

print("Taking screenshot...")
path = gui.screenshot("tests/my_screen.png")
print("Screenshot successfully saved to: " + path)

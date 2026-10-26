// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

// Interactive example: queries the screen size and saves a screenshot.
use std.gui;

size := gui.screen_size();
println("Detected Screen Size:");
println("Width:  " + string(size["width"]));
println("Height: " + string(size["height"]));

println("Taking screenshot...");
path: string = string(gui.screenshot("tests/my_screen.png"));
println("Screenshot successfully saved to: " + path);

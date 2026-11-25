// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// main_runner.cpp - Lightweight CLI Entrypoint for ULang++
// Dynamically binds and executes libulang.dll / libulang.so

#include <iostream>
#include "libulang.h"

int main(int argc, char** argv) {
    return ulang_main(argc, argv);
}

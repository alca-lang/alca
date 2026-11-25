// Copyright 2026 ULang++ Programming Language
// SPDX-License-Identifier: Apache-2.0
//
// libulang.h - Shared Dynamic Library API for ULang++ Compiler

#pragma once

#ifdef _WIN32
  #ifdef BUILDING_ULANG_DLL
    #define ULANG_API __declspec(dllexport)
  #else
    #define ULANG_API __declspec(dllimport)
  #endif
#else
  #define ULANG_API __attribute__((visibility("default")))
#endif

#ifdef __cplusplus
extern "C" {
#endif

// Returns the full compiler version string: "ULang++ 2.1"
ULANG_API const char* ulang_version();

// Runs the CLI command pipeline directly: e.g. ulang_main(3, ["ulang++", "build", "main.al"])
ULANG_API int ulang_main(int argc, char** argv);

// Compiles a single source file to target
ULANG_API int ulang_compile_file(const char* src_path, const char* out_path, const char* target);

// Typechecks a source file. Returns 0 if success, >0 if errors found.
ULANG_API int ulang_check_file(const char* src_path);

#ifdef __cplusplus
}
#endif

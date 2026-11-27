cmake_minimum_required(VERSION 3.16)
project(pawl_native LANGUAGES CXX)

set(CMAKE_CXX_STANDARD 17)
set(CMAKE_CXX_STANDARD_REQUIRED ON)
set(CMAKE_CXX_EXTENSIONS OFF)

if(MSVC)
  add_compile_options(/W4 /WX /permissive-)
else()
  add_compile_options(-Wall -Wextra -Werror)
endif()

add_library(pawl_core STATIC
  src/token/token.cpp
  src/lexer/lexer.cpp
  src/diag/diagnostic.cpp
  src/ast/ast.cpp
  src/parser/parser.cpp
  src/dunder/dunder.cpp
  src/mod/mod.cpp
  src/typecheck/builtins_table.cpp
  src/typecheck/types.cpp
  src/typecheck/typecheck.cpp
  src/ir/ir.cpp
  src/ir/lower.cpp
  src/codegen/x64/codegen.cpp
  src/link/pe/pe_writer.cpp
  src/link/pe/efi_writer.cpp
  src/link/elf/elf_writer.cpp
)
target_include_directories(pawl_core PUBLIC src)

add_executable(pawl src/cli/main.cpp)
target_link_libraries(pawl PRIVATE pawl_core)

add_executable(pawl_tests
  tests/test_main.cpp
  tests/test_token.cpp
  tests/test_lexer.cpp
  tests/test_ast.cpp
  tests/test_parser.cpp
  tests/test_typecheck.cpp
  tests/test_ir.cpp
  tests/test_codegen.cpp
  tests/test_differential.cpp
)
target_include_directories(pawl_tests PRIVATE third_party)
target_compile_definitions(pawl_tests PRIVATE
  PAWL_REPO_ROOT="${CMAKE_CURRENT_SOURCE_DIR}/.."
)
target_link_libraries(pawl_tests PRIVATE pawl_core)

enable_testing()
add_test(NAME unit COMMAND pawl_tests)

# Differential harness: compares the C++ token dump against golden files
# generated from the frozen Go oracle.  See compiler/tests/README.md.
add_test(NAME differential COMMAND pawl_tests differential)

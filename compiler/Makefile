CXX ?= g++
CXXFLAGS ?= -std=c++17 -O2 -Wall -Wextra -Werror
INCLUDES = -Isrc -Ithird_party
ROOT := $(abspath ..)
DEFS = -DPAWL_REPO_ROOT=\"$(ROOT)\"
CORE = src/token/token.cpp src/lexer/lexer.cpp
TESTS = tests/test_main.cpp tests/test_token.cpp tests/test_lexer.cpp tests/test_differential.cpp
OUT = build

all: $(OUT)/pawl $(OUT)/pawl_tests

$(OUT):
	mkdir -p $(OUT)

$(OUT)/pawl: $(CORE) src/cli/main.cpp | $(OUT)
	$(CXX) $(CXXFLAGS) $(INCLUDES) -o $@ $^

$(OUT)/pawl_tests: $(CORE) $(TESTS) | $(OUT)
	$(CXX) $(CXXFLAGS) $(INCLUDES) $(DEFS) -o $@ $^

test: $(OUT)/pawl_tests
	$(OUT)/pawl_tests

clean:
	rm -rf $(OUT)

.PHONY: all test clean

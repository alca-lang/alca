// Minimal single-header test harness, vendored for the Pawl C++ compiler.
// SPDX-License-Identifier: Apache-2.0
//
// Deliberately tiny and dependency-free so tests build offline with any
// C++17 compiler.  Not a general-purpose framework.

#pragma once

#include <chrono>
#include <cstdio>
#include <exception>
#include <functional>
#include <iostream>
#include <sstream>
#include <string>
#include <vector>

namespace microtest {

struct Failure : std::runtime_error {
    explicit Failure(const std::string& message) : std::runtime_error(message) {}
};

struct Case {
    std::string suite;
    std::string name;
    void (*fn)();
};

inline std::vector<Case>& registry() {
    static std::vector<Case> cases;
    return cases;
}

struct Registrar {
    Registrar(std::string suite, std::string name, void (*fn)()) {
        registry().push_back({std::move(suite), std::move(name), fn});
    }
};

inline std::string describe(const std::string& expr, const char* file, int line) {
    std::ostringstream out;
    out << expr << " (" << file << ':' << line << ')';
    return out.str();
}

template <typename T>
std::string show(const T& value) {
    std::ostringstream out;
    out << value;
    return out.str();
}

inline std::string show(const std::string& value) { return '"' + value + '"'; }

inline int run(int argc, char** argv) {
    std::string filter;
    if (argc > 1) {
        filter = argv[1];
    }

    int passed = 0;
    int failed = 0;
    std::vector<std::string> failures;

    const auto start = std::chrono::steady_clock::now();
    for (const auto& test : registry()) {
        const std::string full = test.suite + "." + test.name;
        if (!filter.empty() && full.find(filter) == std::string::npos) {
            continue;
        }
        try {
            test.fn();
            ++passed;
            std::cout << "[  PASS  ] " << full << '\n';
        } catch (const std::exception& e) {
            ++failed;
            std::cout << "[  FAIL  ] " << full << ": " << e.what() << '\n';
            failures.push_back(full + ": " + e.what());
        } catch (...) {
            ++failed;
            std::cout << "[  FAIL  ] " << full << ": unknown exception\n";
            failures.push_back(full + ": unknown exception");
        }
    }

    const auto end = std::chrono::steady_clock::now();
    const auto ms =
        std::chrono::duration_cast<std::chrono::milliseconds>(end - start)
            .count();

    std::cout << "\n" << passed << " passed, " << failed << " failed in "
              << ms << " ms\n";
    if (failed != 0) {
        std::cout << "\nfailures:\n";
        for (const auto& f : failures) {
            std::cout << "  - " << f << '\n';
        }
        return 1;
    }
    return 0;
}

} // namespace microtest

#define MT_CONCAT_INNER(a, b) a##b
#define MT_CONCAT(a, b) MT_CONCAT_INNER(a, b)

#define MT_TEST(suite, name)                                                  \
    static void MT_CONCAT(suite, MT_CONCAT(_, name))();                        \
    static ::microtest::Registrar MT_CONCAT(                                   \
        mt_reg_, MT_CONCAT(suite, MT_CONCAT(_, name))){                        \
        #suite, #name, &MT_CONCAT(suite, MT_CONCAT(_, name))};                 \
    static void MT_CONCAT(suite, MT_CONCAT(_, name))()

#define MT_FAIL(msg)                                                           \
    throw ::microtest::Failure(::microtest::describe((msg), __FILE__, __LINE__))

#define MT_CHECK(cond)                                                         \
    do {                                                                       \
        if (!(cond)) {                                                         \
            MT_FAIL("check failed: " #cond);                                   \
        }                                                                      \
    } while (false)

#define MT_CHECK_EQ(actual, expected)                                          \
    do {                                                                       \
        const auto& mt_a = (actual);                                           \
        const auto& mt_e = (expected);                                         \
        if (!(mt_a == mt_e)) {                                                 \
            MT_FAIL(std::string("expected ") + ::microtest::show(mt_e) +       \
                    ", got " + ::microtest::show(mt_a) + " for " #actual);     \
        }                                                                      \
    } while (false)

#define MT_REQUIRE(cond) MT_CHECK(cond)

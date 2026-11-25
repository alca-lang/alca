CXX ?= g++
CXXFLAGS ?= -std=c++17 -O2 -Wall -Wextra -static
SRC = src/front.cpp src/codegen.cpp src/link.cpp src/main.cpp
OUT = build/pawlc

all: $(OUT)

$(OUT): $(SRC) src/front.h src/codegen.h src/link.h
	mkdir -p build
	$(CXX) $(CXXFLAGS) -o $(OUT) $(SRC)

clean:
	rm -rf build

.PHONY: all clean

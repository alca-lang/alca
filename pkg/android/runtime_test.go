// Copyright 2026 Alca Programming Language
// SPDX-License-Identifier: Apache-2.0

package android

import (
	"strings"
	"testing"
)

func TestTranspileOperatorsAndArith(t *testing.T) {
	src := `
a := 10;
b := 3;
div := a / b;
c := (a > 5) and (b < 5);
d := not (a == b);
e := a != b;
`
	js, err := TranspileAlcaToJS(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(js, "Math.trunc((a) / (b))") {
		t.Errorf("expected integer division with Math.trunc, got:\n%s", js)
	}
	if !strings.Contains(js, "&&") {
		t.Errorf("expected 'and' operator to be transpiled to '&&', got:\n%s", js)
	}
	if strings.Contains(js, " and ") {
		t.Errorf("found raw 'and' keyword in JS output:\n%s", js)
	}
	if !strings.Contains(js, "!((") {
		t.Errorf("expected 'not' operator to be transpiled to '!', got:\n%s", js)
	}
	if !strings.Contains(js, "!==") {
		t.Errorf("expected '!=' operator to be transpiled to '!==', got:\n%s", js)
	}
}

func TestTranspileLoopsAndRanges(t *testing.T) {
	src := `
sum := 0;
for i in 0..3:
    sum = sum + i;
for i in 0..=3:
    sum = sum + i;
items := [1, 2, 3];
for x in items:
    sum = sum + x;
while sum < 100:
    sum = sum + 1;
    if sum == 50:
        break;
`
	js, err := TranspileAlcaToJS(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(js, "for (let i = 0; i < 3; i++)") {
		t.Errorf("expected half-open range loop 'i < 3', got:\n%s", js)
	}
	if !strings.Contains(js, "for (let i = 0; i <= 3; i++)") {
		t.Errorf("expected inclusive range loop 'i <= 3', got:\n%s", js)
	}
	if !strings.Contains(js, "for (let x of items)") {
		t.Errorf("expected for-of loop, got:\n%s", js)
	}
	if !strings.Contains(js, "while ((sum < 100))") {
		t.Errorf("expected while loop, got:\n%s", js)
	}
	if !strings.Contains(js, "break;") {
		t.Errorf("expected break statement, got:\n%s", js)
	}
}

func TestTranspileStructsClassesEnums(t *testing.T) {
	src := `
enum Status {
    Pending,
    Active,
    Done
}

struct Point {
    x: int = 0;
    y: int = 0;
}

class Animal {
    pub name: string;
    pub fn speak() -> string {
        return self.name;
    }
}
`
	js, err := TranspileAlcaToJS(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(js, "const Status = Object.freeze") {
		t.Errorf("expected enum Status object, got:\n%s", js)
	}
	if !strings.Contains(js, "function Point(x, y)") {
		t.Errorf("expected struct Point constructor, got:\n%s", js)
	}
	if !strings.Contains(js, "class Animal_Class") || !strings.Contains(js, "function Animal(name)") {
		t.Errorf("expected Animal class and factory, got:\n%s", js)
	}
	if !strings.Contains(js, "this.name") {
		t.Errorf("expected 'self.name' mapped to 'this.name', got:\n%s", js)
	}
}

func TestTranspileMatches(t *testing.T) {
	src := `
val := 2;
res := "";
match val:
    case 1:
        res = "one";
    case 2:
        res = "two";
    else:
        res = "other";
`
	js, err := TranspileAlcaToJS(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(js, "switch (val)") {
		t.Errorf("expected switch (val), got:\n%s", js)
	}
	if !strings.Contains(js, "case 1:") || !strings.Contains(js, "case 2:") || !strings.Contains(js, "default:") {
		t.Errorf("expected case and default clauses, got:\n%s", js)
	}
}

func TestTranspilerRejectsUnsupportedNodesWithLine(t *testing.T) {
	src := `
ba:
    println("background job");
`
	_, err := TranspileAlcaToJS(src)
	if err == nil {
		t.Fatalf("expected error for unsupported BaStatement, got nil")
	}
	if !strings.Contains(err.Error(), "unsupported") || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("expected error message to mention 'unsupported' and line number, got: %v", err)
	}
}

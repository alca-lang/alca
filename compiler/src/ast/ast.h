// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <cstdint>
#include <map>
#include <memory>
#include <string>
#include <vector>

#include "token/token.h"

namespace pawl::ast {

// Base interface for all AST nodes
class Node {
public:
    virtual ~Node() = default;
    virtual std::string tokenLiteral() const = 0;
    virtual std::string toString() const = 0;
};

// Base interface for statement nodes
class Statement : public Node {
public:
    virtual void statementNode() = 0;
};

// Base interface for expression nodes
class Expression : public Node {
public:
    virtual void expressionNode() = 0;
};

using StatementPtr = std::shared_ptr<Statement>;
using ExpressionPtr = std::shared_ptr<Expression>;

// Forward declarations of concrete AST nodes
class Program;
class VarDeclaration;
class FuncDeclaration;
class FuncParam;
class StructDeclaration;
class ClassDeclaration;
class MethodDeclaration;
class StructField;
class IfStatement;
class ElifClause;
class WhileStatement;
class ForStatement;
class ForEachStatement;
class LoopStatement;
class BlockStatement;
class ReturnStatement;
class BreakStatement;
class ContinueStatement;
class ExpressionStatement;
class UseStatement;
class WatchStatement;
class TestStatement;
class MockStatement;
class BaStatement;
class BaForStatement;
class ThreadsStatement;
class TxStatement;
class WaitStatement;
class AssertStatement;
class Decorator;
class MatchStatement;
class MatchCase;

class Identifier;
class IntegerLiteral;
class FloatLiteral;
class StringLiteral;
class BooleanLiteral;
class NullLiteral;
class ListLiteral;
class MapPair;
class MapLiteral;
class PrefixExpression;
class InfixExpression;
class CallExpression;
class IndexExpression;
class DotExpression;
class AssignExpression;
class StreamExpression;
class PipeExpression;
class TimeoutExpression;
class ConsoleLiteral;
class LambdaExpression;
class TypeAnnotation;
class ErrExpression;
class RangeLiteral;
class StructInstantiation;
class EnumVariant;
class EnumDeclaration;
class MethodSignature;
class InterfaceDeclaration;
class AwaitExpression;

// TypeAnnotation represents an explicit type annotation, which may be a union.
class TypeAnnotation : public Expression {
public:
    token::Token token;
    std::vector<std::string> types;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// Decorator represents an annotation (@name or @name(arg))
class Decorator : public Statement {
public:
    token::Token token;
    std::string name;
    ExpressionPtr arg;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// Program is the root node of every Pawl abstract syntax tree.
class Program : public Statement {
public:
    token::Token token;
    std::vector<StatementPtr> statements;

    void statementNode() override {}
    std::string tokenLiteral() const override;
    std::string toString() const override;
};

// Identifier represents a named identifier.
class Identifier : public Expression {
public:
    token::Token token;
    std::string value;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return value; }
};

// VarDeclaration represents a variable declaration and optional assignment.
class VarDeclaration : public Statement {
public:
    token::Token token;
    std::vector<std::shared_ptr<Decorator>> decorators;
    std::shared_ptr<Identifier> name;
    std::shared_ptr<TypeAnnotation> typeAnnotation;
    ExpressionPtr value;
    std::string visibility; // "", "public" or "private"
    bool isConst = false;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// FuncParam represents a parameter in a function declaration.
class FuncParam : public Statement {
public:
    token::Token token;
    std::string name;
    std::shared_ptr<TypeAnnotation> typeAnnotation;
    ExpressionPtr defaultValue;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// BlockStatement represents a block of statements enclosed in braces or indentation.
class BlockStatement : public Statement {
public:
    token::Token token;
    std::vector<StatementPtr> statements;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// FuncDeclaration represents a named function declaration.
class FuncDeclaration : public Statement {
public:
    token::Token token;
    std::vector<std::shared_ptr<Decorator>> decorators;
    bool isPublic = false;
    bool isAsync = false;
    std::string name;
    std::vector<std::shared_ptr<FuncParam>> params;
    std::shared_ptr<TypeAnnotation> returnType;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// StructField represents a field inside a struct or class declaration.
class StructField : public Statement {
public:
    token::Token token;
    std::string name;
    std::shared_ptr<TypeAnnotation> typeAnnotation;
    ExpressionPtr defaultValue;
    std::string visibility;
    bool isConst = false;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// StructDeclaration represents a struct type declaration.
class StructDeclaration : public Statement {
public:
    token::Token token;
    std::vector<std::shared_ptr<Decorator>> decorators;
    bool isPublic = false;
    std::string name;
    std::vector<std::shared_ptr<StructField>> fields;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MethodDeclaration represents a method definition inside a class.
class MethodDeclaration : public Node {
public:
    token::Token token;
    std::string visibility;
    std::shared_ptr<FuncDeclaration> func;

    std::string tokenLiteral() const override { return token.literal; }
    std::string name() const { return func ? func->name : ""; }
    std::string toString() const override;
};

// ClassDeclaration represents an object-oriented class declaration.
class ClassDeclaration : public Statement {
public:
    token::Token token;
    std::vector<std::shared_ptr<Decorator>> decorators;
    bool isPublic = false;
    std::string name;
    std::string parent;
    token::Token parentToken;
    std::vector<std::shared_ptr<StructField>> fields;
    std::vector<std::shared_ptr<MethodDeclaration>> methods;
    std::vector<std::shared_ptr<StructDeclaration>> nestedStructs;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ElifClause represents an elif branch in an if-statement.
class ElifClause : public Statement {
public:
    token::Token token;
    ExpressionPtr condition;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// IfStatement represents a conditional branching statement.
class IfStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr condition;
    std::shared_ptr<BlockStatement> consequence;
    std::vector<std::shared_ptr<ElifClause>> alternatives;
    std::shared_ptr<BlockStatement> elseBlock;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// WhileStatement represents a condition-driven while loop.
class WhileStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr condition;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ForStatement represents a loop iterating over a range or collection.
class ForStatement : public Statement {
public:
    token::Token token;
    std::string variable;
    ExpressionPtr iterable;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ForEachStatement represents a foreach statement.
class ForEachStatement : public Statement {
public:
    token::Token token;
    std::string variable;
    ExpressionPtr iterable;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// LoopStatement represents an unconditional infinite loop statement.
class LoopStatement : public Statement {
public:
    token::Token token;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ReturnStatement represents a return statement.
class ReturnStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr value;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// BreakStatement represents a break statement.
class BreakStatement : public Statement {
public:
    token::Token token;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return "break"; }
};

// ContinueStatement represents a continue statement.
class ContinueStatement : public Statement {
public:
    token::Token token;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return "continue"; }
};

// ExpressionStatement represents an expression evaluated as a statement.
class ExpressionStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr expression;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// UseStatement represents an import statement.
class UseStatement : public Statement {
public:
    token::Token token;
    std::string path;
    std::string alias;
    std::vector<std::string> symbols;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// WatchStatement represents an observable watch declaration.
class WatchStatement : public Statement {
public:
    token::Token token;
    std::shared_ptr<Identifier> target;
    std::shared_ptr<BlockStatement> handler;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// TestStatement represents a testing unit declaration.
class TestStatement : public Statement {
public:
    token::Token token;
    std::string name;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MockStatement represents a test mocking declaration.
class MockStatement : public Statement {
public:
    token::Token token;
    std::shared_ptr<Identifier> target;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// BaStatement represents a business architecture domain declaration.
class BaStatement : public Statement {
public:
    token::Token token;
    std::string role;
    std::string description;
    std::vector<std::shared_ptr<Decorator>> decorators;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// BaForStatement represents a business-architecture iteration construct.
class BaForStatement : public Statement {
public:
    token::Token token;
    std::string variable;
    ExpressionPtr iterable;
    std::vector<std::shared_ptr<Decorator>> decorators;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ThreadsStatement represents a concurrent threads statement.
class ThreadsStatement : public Statement {
public:
    token::Token token;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// TxStatement represents an atomic transaction block.
class TxStatement : public Statement {
public:
    token::Token token;
    std::string name;
    std::shared_ptr<BlockStatement> body;
    std::shared_ptr<BlockStatement> rollback;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// WaitStatement represents a barrier wait instruction.
class WaitStatement : public Statement {
public:
    token::Token token;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return "wait"; }
};

// AssertStatement represents an invariant assertion statement.
class AssertStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr condition;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MatchCase represents a case clause inside a match statement.
class MatchCase : public Statement {
public:
    token::Token token;
    ExpressionPtr pattern;
    std::shared_ptr<BlockStatement> body;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MatchStatement represents pattern matching branching.
class MatchStatement : public Statement {
public:
    token::Token token;
    ExpressionPtr subject;
    std::vector<std::shared_ptr<MatchCase>> cases;
    std::shared_ptr<BlockStatement> defaultCase;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// IntegerLiteral represents an integer constant value.
class IntegerLiteral : public Expression {
public:
    token::Token token;
    int64_t value = 0;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return token.literal; }
};

// FloatLiteral represents a floating-point constant value.
class FloatLiteral : public Expression {
public:
    token::Token token;
    double value = 0.0;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return token.literal; }
};

// StringLiteral represents a string literal value.
class StringLiteral : public Expression {
public:
    token::Token token;
    std::string value;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return token.literal; }
};

// BooleanLiteral represents a boolean true or false literal.
class BooleanLiteral : public Expression {
public:
    token::Token token;
    bool value = false;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return token.literal; }
};

// NullLiteral represents a null literal constant.
class NullLiteral : public Expression {
public:
    token::Token token;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override { return "null"; }
};

// ListLiteral represents an array/list literal expression.
class ListLiteral : public Expression {
public:
    token::Token token;
    std::vector<ExpressionPtr> elements;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MapPair represents a key-value entry inside a map literal.
class MapPair : public Node {
public:
    token::Token token;
    ExpressionPtr key;
    ExpressionPtr value;

    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MapLiteral represents a dictionary/map literal expression.
class MapLiteral : public Expression {
public:
    token::Token token;
    std::vector<std::shared_ptr<MapPair>> pairs;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// PrefixExpression represents a unary prefix operation (e.g. -x, not flag).
class PrefixExpression : public Expression {
public:
    token::Token token;
    std::string op;
    ExpressionPtr right;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// InfixExpression represents a binary infix operation (e.g. a + b, x == y).
class InfixExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr left;
    std::string op;
    ExpressionPtr right;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// CallExpression represents an invocation of a callable target.
class CallExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr function;
    std::vector<ExpressionPtr> arguments;
    std::map<std::string, ExpressionPtr> kwargs;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// IndexExpression represents indexing into a collection or string (left[index]).
class IndexExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr left;
    ExpressionPtr index;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// DotExpression represents member access (left.field).
class DotExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr left;
    std::string field;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// AssignExpression represents an assignment expression (target = value).
class AssignExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr target;
    ExpressionPtr value;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// StreamExpression represents Unix-style stream redirection.
class StreamExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr source;
    std::string op;
    ExpressionPtr target;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// PipeExpression represents pipeline operations (left |> right).
class PipeExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr left;
    ExpressionPtr right;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// TimeoutExpression represents a timeout constraint expression.
class TimeoutExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr expression;
    ExpressionPtr duration;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ConsoleLiteral represents interactive terminal prompt expressions.
class ConsoleLiteral : public Expression {
public:
    token::Token token;
    ExpressionPtr prompt;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// LambdaExpression represents an anonymous inline function.
class LambdaExpression : public Expression {
public:
    token::Token token;
    std::vector<std::shared_ptr<FuncParam>> params;
    ExpressionPtr body;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// ErrExpression represents a first-class error construction expression.
class ErrExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr message;
    ExpressionPtr code;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// RangeLiteral represents range definitions (0..10 or 0..=10).
class RangeLiteral : public Expression {
public:
    token::Token token;
    ExpressionPtr start;
    ExpressionPtr end;
    bool inclusive = false;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// StructInstantiation represents an instance creation expression.
class StructInstantiation : public Expression {
public:
    token::Token token;
    std::string name;
    std::vector<ExpressionPtr> args;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// EnumVariant represents a single variant inside an enum declaration.
class EnumVariant : public Node {
public:
    token::Token token;
    std::string name;
    ExpressionPtr value;

    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// EnumDeclaration represents an enum type declaration.
class EnumDeclaration : public Statement {
public:
    token::Token token;
    std::string name;
    std::vector<std::shared_ptr<EnumVariant>> variants;
    bool isPublic = false;
    std::vector<std::shared_ptr<Decorator>> decorators;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// MethodSignature represents an abstract method inside an interface.
class MethodSignature : public Node {
public:
    token::Token token;
    std::string name;
    std::vector<std::shared_ptr<FuncParam>> params;
    std::shared_ptr<TypeAnnotation> returnType;

    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// InterfaceDeclaration represents an interface/trait declaration.
class InterfaceDeclaration : public Statement {
public:
    token::Token token;
    std::string name;
    std::vector<std::shared_ptr<MethodSignature>> methods;
    bool isPublic = false;
    std::vector<std::shared_ptr<Decorator>> decorators;

    void statementNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

// AwaitExpression represents an await operation on an async value.
class AwaitExpression : public Expression {
public:
    token::Token token;
    ExpressionPtr expression;

    void expressionNode() override {}
    std::string tokenLiteral() const override { return token.literal; }
    std::string toString() const override;
};

} // namespace pawl::ast

// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "ast/ast.h"

#include <algorithm>
#include <sstream>

namespace pawl::ast {

namespace {

std::string join(const std::vector<std::string>& list, const std::string& sep) {
    std::string res;
    for (size_t i = 0; i < list.size(); ++i) {
        if (i > 0) res += sep;
        res += list[i];
    }
    return res;
}

} // namespace

std::string Program::tokenLiteral() const {
    if (!token.literal.empty()) return token.literal;
    if (!statements.empty() && statements[0]) return statements[0]->tokenLiteral();
    return "";
}

std::string Program::toString() const {
    std::ostringstream out;
    for (const auto& s : statements) {
        if (s) {
            out << s->toString() << "\n";
        }
    }
    return out.str();
}

std::string TypeAnnotation::toString() const {
    return join(types, " | ");
}

std::string Decorator::toString() const {
    std::string out = "@" + name;
    if (arg) {
        out += "(" + arg->toString() + ")";
    }
    return out;
}

std::string VarDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (!visibility.empty()) {
        out << visibility << " ";
    }
    if (isConst) {
        out << "const ";
    }
    if (name) {
        out << name->toString();
    }
    if (typeAnnotation) {
        out << ": " << typeAnnotation->toString();
    }
    if (value) {
        out << " = " << value->toString();
    }
    return out.str();
}

std::string FuncParam::toString() const {
    std::string out = name;
    if (typeAnnotation) {
        out += ": " + typeAnnotation->toString();
    }
    if (defaultValue) {
        out += " = " + defaultValue->toString();
    }
    return out;
}

std::string BlockStatement::toString() const {
    std::ostringstream out;
    for (const auto& s : statements) {
        if (s) out << s->toString();
    }
    return out.str();
}

std::string FuncDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (isPublic) out << "pub ";
    if (isAsync) out << "async ";
    out << "func " << name << "(";
    std::vector<std::string> pStrs;
    for (const auto& p : params) {
        if (p) pStrs.push_back(p->toString());
    }
    out << join(pStrs, ", ") << ")";
    if (returnType) {
        out << " -> " << returnType->toString();
    }
    out << " ";
    if (body) {
        out << body->toString();
    }
    return out.str();
}

std::string StructField::toString() const {
    std::ostringstream out;
    if (!visibility.empty()) out << visibility << " ";
    if (isConst) out << "const ";
    out << name;
    if (typeAnnotation) {
        out << ": " << typeAnnotation->toString();
    }
    if (defaultValue) {
        out << " = " << defaultValue->toString();
    }
    return out.str();
}

std::string StructDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (isPublic) out << "pub ";
    out << "struct " << name << " {\n";
    for (const auto& f : fields) {
        if (f) out << "  " << f->toString() << "\n";
    }
    out << "}";
    return out.str();
}

std::string MethodDeclaration::toString() const {
    std::ostringstream out;
    if (!visibility.empty()) out << visibility << " ";
    if (func) out << func->toString();
    return out.str();
}

std::string ClassDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (isPublic) out << "pub ";
    out << "class " << name;
    if (!parent.empty()) out << " extends " << parent;
    out << " {\n";
    for (const auto& f : fields) {
        if (f) out << "  " << f->toString() << "\n";
    }
    for (const auto& m : methods) {
        if (m) out << "  " << m->toString() << "\n";
    }
    out << "}";
    return out.str();
}

std::string ElifClause::toString() const {
    std::ostringstream out;
    out << "elif ";
    if (condition) out << condition->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string IfStatement::toString() const {
    std::ostringstream out;
    out << "if ";
    if (condition) out << condition->toString();
    out << " ";
    if (consequence) out << consequence->toString();
    for (const auto& alt : alternatives) {
        if (alt) out << " " << alt->toString();
    }
    if (elseBlock) {
        out << " else " << elseBlock->toString();
    }
    return out.str();
}

std::string WhileStatement::toString() const {
    std::ostringstream out;
    out << "while ";
    if (condition) out << condition->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string ForStatement::toString() const {
    std::ostringstream out;
    out << "for " << variable << " in ";
    if (iterable) out << iterable->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string ForEachStatement::toString() const {
    std::ostringstream out;
    out << "foreach " << variable << " in ";
    if (iterable) out << iterable->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string LoopStatement::toString() const {
    std::ostringstream out;
    out << "loop ";
    if (body) out << body->toString();
    return out.str();
}

std::string ReturnStatement::toString() const {
    std::string out = "return";
    if (value) out += " " + value->toString();
    return out;
}

std::string ExpressionStatement::toString() const {
    if (expression) return expression->toString();
    return "";
}

std::string UseStatement::toString() const {
    std::ostringstream out;
    out << "use " << path;
    if (!alias.empty()) out << " as " << alias;
    if (!symbols.empty()) {
        out << " { " << join(symbols, ", ") << " }";
    }
    return out.str();
}

std::string WatchStatement::toString() const {
    std::ostringstream out;
    out << "watch ";
    if (target) out << target->toString();
    out << " ";
    if (handler) out << handler->toString();
    return out.str();
}

std::string TestStatement::toString() const {
    std::ostringstream out;
    out << "test \"" << name << "\" ";
    if (body) out << body->toString();
    return out.str();
}

std::string MockStatement::toString() const {
    std::ostringstream out;
    out << "mock ";
    if (target) out << target->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string BaStatement::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    out << "ba " << role;
    if (!description.empty()) out << " \"" << description << "\"";
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string BaForStatement::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    out << "ba for " << variable << " in ";
    if (iterable) out << iterable->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string ThreadsStatement::toString() const {
    std::ostringstream out;
    out << "threads ";
    if (body) out << body->toString();
    return out.str();
}

std::string TxStatement::toString() const {
    std::ostringstream out;
    out << "tx";
    if (!name.empty()) out << " " << name;
    out << " ";
    if (body) out << body->toString();
    if (rollback) out << " rollback " + rollback->toString();
    return out.str();
}

std::string AssertStatement::toString() const {
    std::string out = "assert ";
    if (condition) out += condition->toString();
    return out;
}

std::string MatchCase::toString() const {
    std::ostringstream out;
    out << "case ";
    if (pattern) out << pattern->toString();
    out << " ";
    if (body) out << body->toString();
    return out.str();
}

std::string MatchStatement::toString() const {
    std::ostringstream out;
    out << "match ";
    if (subject) out << subject->toString();
    out << " {\n";
    for (const auto& c : cases) {
        if (c) out << "  " << c->toString() << "\n";
    }
    if (defaultCase) {
        out << "  default " << defaultCase->toString() << "\n";
    }
    out << "}";
    return out.str();
}

std::string ListLiteral::toString() const {
    std::vector<std::string> elems;
    for (const auto& e : elements) {
        if (e) elems.push_back(e->toString());
    }
    return "[" + join(elems, ", ") + "]";
}

std::string MapPair::toString() const {
    std::string k = key ? key->toString() : "";
    std::string v = value ? value->toString() : "";
    return k + ": " + v;
}

std::string MapLiteral::toString() const {
    std::vector<std::string> pStrs;
    for (const auto& p : pairs) {
        if (p) pStrs.push_back(p->toString());
    }
    return "{" + join(pStrs, ", ") + "}";
}

std::string PrefixExpression::toString() const {
    std::string out = "(" + op;
    if (right) out += right->toString();
    out += ")";
    return out;
}

std::string InfixExpression::toString() const {
    std::string l = left ? left->toString() : "";
    std::string r = right ? right->toString() : "";
    return "(" + l + " " + op + " " + r + ")";
}

std::string CallExpression::toString() const {
    std::ostringstream out;
    if (function) out << function->toString();
    out << "(";
    std::vector<std::string> args;
    for (const auto& a : arguments) {
        if (a) args.push_back(a->toString());
    }
    for (const auto& pair : kwargs) {
        std::string valStr = pair.second ? pair.second->toString() : "";
        args.push_back(pair.first + "=" + valStr);
    }
    out << join(args, ", ") << ")";
    return out.str();
}

std::string IndexExpression::toString() const {
    std::string l = left ? left->toString() : "";
    std::string idx = index ? index->toString() : "";
    return "(" + l + "[" + idx + "])";
}

std::string DotExpression::toString() const {
    std::string l = left ? left->toString() : "";
    return l + "." + field;
}

std::string AssignExpression::toString() const {
    std::string t = target ? target->toString() : "";
    std::string v = value ? value->toString() : "";
    return t + " = " + v;
}

std::string StreamExpression::toString() const {
    std::string s = source ? source->toString() : "";
    std::string t = target ? target->toString() : "";
    return "(" + s + " " + op + " " + t + ")";
}

std::string PipeExpression::toString() const {
    std::string l = left ? left->toString() : "";
    std::string r = right ? right->toString() : "";
    return "(" + l + " |> " + r + ")";
}

std::string TimeoutExpression::toString() const {
    std::string e = expression ? expression->toString() : "";
    std::string d = duration ? duration->toString() : "";
    return "(" + e + " ~ " + d + ")";
}

std::string ConsoleLiteral::toString() const {
    if (prompt) return "(console " + prompt->toString() + ")";
    return "console";
}

std::string LambdaExpression::toString() const {
    std::vector<std::string> pStrs;
    for (const auto& p : params) {
        if (p) pStrs.push_back(p->toString());
    }
    std::string b = body ? body->toString() : "";
    return "|" + join(pStrs, ", ") + "| -> " + b;
}

std::string ErrExpression::toString() const {
    std::string m = message ? message->toString() : "";
    if (code) {
        return "err(" + m + ", " + code->toString() + ")";
    }
    return "err(" + m + ")";
}

std::string RangeLiteral::toString() const {
    std::string s = start ? start->toString() : "";
    std::string e = end ? end->toString() : "";
    std::string op = inclusive ? "..=" : "..";
    return "(" + s + op + e + ")";
}

std::string StructInstantiation::toString() const {
    std::vector<std::string> aStrs;
    for (const auto& a : args) {
        if (a) aStrs.push_back(a->toString());
    }
    return name + "{" + join(aStrs, ", ") + "}";
}

std::string EnumVariant::toString() const {
    if (value) return name + " = " + value->toString();
    return name;
}

std::string EnumDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (isPublic) out << "pub ";
    out << "enum " << name << " {\n";
    for (const auto& v : variants) {
        if (v) out << "  " << v->toString() << "\n";
    }
    out << "}";
    return out.str();
}

std::string MethodSignature::toString() const {
    std::vector<std::string> pStrs;
    for (const auto& p : params) {
        if (p) pStrs.push_back(p->toString());
    }
    std::string out = "func " + name + "(" + join(pStrs, ", ") + ")";
    if (returnType) {
        out += " -> " + returnType->toString();
    }
    return out;
}

std::string InterfaceDeclaration::toString() const {
    std::ostringstream out;
    for (const auto& d : decorators) {
        if (d) out << d->toString() << "\n";
    }
    if (isPublic) out << "pub ";
    out << "interface " << name << " {\n";
    for (const auto& m : methods) {
        if (m) out << "  " << m->toString() << "\n";
    }
    out << "}";
    return out.str();
}

std::string AwaitExpression::toString() const {
    std::string e = expression ? expression->toString() : "";
    return "(await " + e + ")";
}

} // namespace pawl::ast

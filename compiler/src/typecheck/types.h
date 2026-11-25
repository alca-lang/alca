// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#pragma once

#include <memory>
#include <optional>
#include <string>
#include <vector>
#include <unordered_map>

namespace ulang::typecheck {

class Type {
public:
    virtual ~Type() = default;
    virtual std::string toString() const = 0;
};

using TypePtr = std::shared_ptr<Type>;

class Basic : public Type {
public:
    explicit Basic(std::string name) : name_(std::move(name)) {}
    std::string toString() const override { return name_; }
    const std::string& name() const { return name_; }

private:
    std::string name_;
};

// Singleton primitive types
TypePtr getInt();
TypePtr getFloat();
TypePtr getString();
TypePtr getBool();
TypePtr getNull();
TypePtr getVoid();
TypePtr getBlob();
TypePtr getSecret();
TypePtr getJSON();
TypePtr getAny();
TypePtr getNever();
TypePtr getInvalid();

class ListType : public Type {
public:
    explicit ListType(TypePtr elem) : elem_(std::move(elem)) {}
    std::string toString() const override { return "list[" + (elem_ ? elem_->toString() : "any") + "]"; }
    const TypePtr& elem() const { return elem_; }

private:
    TypePtr elem_;
};

class MapType : public Type {
public:
    MapType(TypePtr key, TypePtr val) : key_(std::move(key)), val_(std::move(val)) {}
    std::string toString() const override {
        return "map[" + (key_ ? key_->toString() : "any") + ", " + (val_ ? val_->toString() : "any") + "]";
    }
    const TypePtr& key() const { return key_; }
    const TypePtr& val() const { return val_; }

private:
    TypePtr key_;
    TypePtr val_;
};

struct Param {
    std::string name;
    TypePtr type;
    bool optional = false;
};

class FuncType : public Type {
public:
    std::vector<Param> params;
    TypePtr ret;
    bool variadic = false;
    bool dynamic = false;

    std::string toString() const override;
};

class Overloads : public Type {
public:
    std::vector<std::shared_ptr<FuncType>> sigs;
    std::string toString() const override;
};

class StructType;

struct Field {
    std::string name;
    TypePtr type;
    bool hasDefault = false;
    std::string visibility; // "", "public" or "private"
    bool isConst = false;
    std::shared_ptr<StructType> owner;
};

struct Method {
    std::string name;
    std::shared_ptr<FuncType> signature;
    std::string visibility;
    std::shared_ptr<StructType> owner;
};

class StructType : public Type, public std::enable_shared_from_this<StructType> {
public:
    std::string name;
    std::vector<Field> fields;
    std::unordered_map<std::string, std::shared_ptr<Method>> methods;
    std::shared_ptr<StructType> parent;
    bool isClass = false;

    std::string toString() const override { return name; }

    std::optional<Field> lookupField(const std::string& fieldName) const;
    std::shared_ptr<Method> lookupMethod(const std::string& methodName) const;
    std::vector<Field> allFields() const;
    bool inherits(const std::shared_ptr<StructType>& other) const;
};

class EnumType : public Type {
public:
    std::string name;
    std::unordered_map<std::string, TypePtr> variants;

    std::string toString() const override { return name; }
};

class InterfaceType : public Type {
public:
    std::string name;
    std::unordered_map<std::string, std::shared_ptr<FuncType>> methods;

    std::string toString() const override { return name; }
};

class SuperType : public Type {
public:
    std::shared_ptr<StructType> parent;
    std::string toString() const override {
        return parent ? "super " + parent->name : "super";
    }
};

class UnionType : public Type {
public:
    std::vector<TypePtr> types;
    std::string toString() const override;
};

class ModuleType : public Type {
public:
    std::string name;
    std::unordered_map<std::string, TypePtr> members;
    bool open = false;

    std::string toString() const override { return "module " + name; }
};

// Builtin error type
std::shared_ptr<StructType> getErrorType();

// Type utilities
TypePtr newUnion(const std::vector<TypePtr>& ts);
bool equal(const TypePtr& a, const TypePtr& b);
bool assignable(const TypePtr& dst, const TypePtr& src);
bool comparable(const TypePtr& a, const TypePtr& b);
bool isNumeric(const TypePtr& t);
bool isInteger(const TypePtr& t);
bool nullable(const TypePtr& t);

TypePtr parseType(const std::string& src, const std::unordered_map<std::string, TypePtr>* localTypes = nullptr);
TypePtr parseSig(const std::string& sig);

std::shared_ptr<ModuleType> moduleByName(const std::string& name);
std::shared_ptr<ModuleType> stdNamespace();
const std::unordered_map<std::string, TypePtr>& globalBuiltins();

} // namespace ulang::typecheck

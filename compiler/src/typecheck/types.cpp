// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "typecheck/types.h"
#include "typecheck/builtins_table.h"

#include <algorithm>
#include <mutex>
#include <optional>
#include <sstream>

namespace pawl::typecheck {

TypePtr getInt() { static auto t = std::make_shared<Basic>("int"); return t; }
TypePtr getFloat() { static auto t = std::make_shared<Basic>("float"); return t; }
TypePtr getString() { static auto t = std::make_shared<Basic>("string"); return t; }
TypePtr getBool() { static auto t = std::make_shared<Basic>("bool"); return t; }
TypePtr getNull() { static auto t = std::make_shared<Basic>("null"); return t; }
TypePtr getVoid() { static auto t = std::make_shared<Basic>("void"); return t; }
TypePtr getBlob() { static auto t = std::make_shared<Basic>("blob"); return t; }
TypePtr getSecret() { static auto t = std::make_shared<Basic>("secret"); return t; }
TypePtr getJSON() { static auto t = std::make_shared<Basic>("json"); return t; }
TypePtr getAny() { static auto t = std::make_shared<Basic>("any"); return t; }
TypePtr getNever() { static auto t = std::make_shared<Basic>("never"); return t; }
TypePtr getInvalid() { static auto t = std::make_shared<Basic>("<invalid>"); return t; }

std::shared_ptr<StructType> getErrorType() {
    static auto err = [] {
        auto s = std::make_shared<StructType>();
        s->name = "error";
        s->fields = {
            {"msg", getString(), false, "public", false, s},
            {"message", getString(), false, "public", false, s},
            {"code", getInt(), false, "public", false, s},
            {"time", getString(), false, "public", false, s}
        };
        return s;
    }();
    return err;
}

std::string FuncType::toString() const {
    if (dynamic) return "func";
    std::string res = "func(";
    for (size_t i = 0; i < params.size(); ++i) {
        if (i > 0) res += ", ";
        res += params[i].type ? params[i].type->toString() : "any";
        if (params[i].optional) res += "?";
    }
    res += ") -> ";
    res += ret ? ret->toString() : "void";
    return res;
}

std::string Overloads::toString() const {
    std::string res;
    for (size_t i = 0; i < sigs.size(); ++i) {
        if (i > 0) res += " | ";
        if (sigs[i]) res += sigs[i]->toString();
    }
    return res;
}

std::optional<Field> StructType::lookupField(const std::string& fieldName) const {
    for (const auto& f : fields) {
        if (f.name == fieldName) return f;
    }
    if (parent) return parent->lookupField(fieldName);
    return std::nullopt;
}

std::shared_ptr<Method> StructType::lookupMethod(const std::string& methodName) const {
    auto it = methods.find(methodName);
    if (it != methods.end()) return it->second;
    if (parent) return parent->lookupMethod(methodName);
    return nullptr;
}

std::vector<Field> StructType::allFields() const {
    std::vector<Field> out;
    if (parent) {
        out = parent->allFields();
    }
    for (const auto& f : fields) {
        out.push_back(f);
    }
    return out;
}

bool StructType::inherits(const std::shared_ptr<StructType>& other) const {
    if (!other) return false;
    for (const StructType* cur = this; cur != nullptr; cur = cur->parent.get()) {
        if (cur == other.get() || cur->name == other->name) return true;
    }
    return false;
}

std::string UnionType::toString() const {
    std::string res;
    for (size_t i = 0; i < types.size(); ++i) {
        if (i > 0) res += " | ";
        if (types[i]) res += types[i]->toString();
    }
    return res;
}

TypePtr newUnion(const std::vector<TypePtr>& ts) {
    std::vector<TypePtr> flat;
    auto add = [&](auto& self, const TypePtr& t) -> void {
        if (!t) return;
        if (auto u = std::dynamic_pointer_cast<UnionType>(t)) {
            for (const auto& m : u->types) self(self, m);
            return;
        }
        for (const auto& e : flat) {
            if (equal(e, t)) return;
        }
        flat.push_back(t);
    };
    for (const auto& t : ts) add(add, t);
    if (flat.empty()) return getInvalid();
    if (flat.size() == 1) return flat[0];
    auto u = std::make_shared<UnionType>();
    u->types = flat;
    return u;
}

bool equal(const TypePtr& a, const TypePtr& b) {
    if (a == b) return true;
    if (!a || !b) return false;

    if (auto la = std::dynamic_pointer_cast<ListType>(a)) {
        if (auto lb = std::dynamic_pointer_cast<ListType>(b)) {
            return equal(la->elem(), lb->elem());
        }
        return false;
    }
    if (auto ma = std::dynamic_pointer_cast<MapType>(a)) {
        if (auto mb = std::dynamic_pointer_cast<MapType>(b)) {
            return equal(ma->key(), mb->key()) && equal(ma->val(), mb->val());
        }
        return false;
    }
    if (auto sa = std::dynamic_pointer_cast<StructType>(a)) {
        if (auto sb = std::dynamic_pointer_cast<StructType>(b)) {
            return sa->name == sb->name;
        }
        return false;
    }
    if (auto fa = std::dynamic_pointer_cast<FuncType>(a)) {
        auto fb = std::dynamic_pointer_cast<FuncType>(b);
        if (!fb || fa->dynamic != fb->dynamic || fa->params.size() != fb->params.size() || !equal(fa->ret, fb->ret)) {
            return false;
        }
        for (size_t i = 0; i < fa->params.size(); ++i) {
            if (!equal(fa->params[i].type, fb->params[i].type)) return false;
        }
        return true;
    }
    if (auto ua = std::dynamic_pointer_cast<UnionType>(a)) {
        auto ub = std::dynamic_pointer_cast<UnionType>(b);
        if (!ub || ua->types.size() != ub->types.size()) return false;
        for (const auto& m : ua->types) {
            bool found = false;
            for (const auto& n : ub->types) {
                if (equal(m, n)) { found = true; break; }
            }
            if (!found) return false;
        }
        return true;
    }
    if (auto ea = std::dynamic_pointer_cast<EnumType>(a)) {
        if (auto eb = std::dynamic_pointer_cast<EnumType>(b)) {
            return ea->name == eb->name;
        }
        return false;
    }
    if (auto ia = std::dynamic_pointer_cast<InterfaceType>(a)) {
        if (auto ib = std::dynamic_pointer_cast<InterfaceType>(b)) {
            return ia->name == ib->name;
        }
        return false;
    }
    return false;
}

bool isNumeric(const TypePtr& t) {
    return t == getInt() || t == getFloat();
}

bool isInteger(const TypePtr& t) {
    return t == getInt();
}

bool nullable(const TypePtr& t) {
    if (!t) return false;
    if (t == getNull()) return true;
    if (auto u = std::dynamic_pointer_cast<UnionType>(t)) {
        for (const auto& m : u->types) {
            if (m == getNull()) return true;
        }
    }
    return false;
}

bool assignable(const TypePtr& dst, const TypePtr& src) {
    if (dst == getInvalid() || src == getInvalid() || equal(dst, src)) return true;
    if (auto sl = std::dynamic_pointer_cast<ListType>(src)) {
        if (sl->elem() == getNever()) {
            if (std::dynamic_pointer_cast<ListType>(dst)) return true;
        }
    }
    if (auto sm = std::dynamic_pointer_cast<MapType>(src)) {
        if (sm->key() == getNever()) {
            if (std::dynamic_pointer_cast<MapType>(dst)) return true;
        }
    }
    if (dst == getAny() || src == getNever()) return true;
    if (auto dl = std::dynamic_pointer_cast<ListType>(dst)) {
        if (auto sl = std::dynamic_pointer_cast<ListType>(src)) {
            if (dl->elem() == getAny() || dl->elem() == getJSON()) return true;
            return assignable(dl->elem(), sl->elem());
        }
    }
    if (auto dm = std::dynamic_pointer_cast<MapType>(dst)) {
        if (auto sm = std::dynamic_pointer_cast<MapType>(src)) {
            if ((dm->key() == getAny() || dm->key() == getJSON()) &&
                (dm->val() == getAny() || dm->val() == getJSON())) return true;
            return assignable(dm->key(), sm->key()) && assignable(dm->val(), sm->val());
        }
    }
    if (dst == getJSON()) return true;

    if (auto su = std::dynamic_pointer_cast<UnionType>(src)) {
        for (const auto& m : su->types) {
            if (!assignable(dst, m)) return false;
        }
        return true;
    }
    if (auto du = std::dynamic_pointer_cast<UnionType>(dst)) {
        for (const auto& m : du->types) {
            if (assignable(m, src)) return true;
        }
        return false;
    }
    if (auto f = std::dynamic_pointer_cast<FuncType>(dst)) {
        if (f->dynamic && std::dynamic_pointer_cast<FuncType>(src)) return true;
    }
    if (auto d = std::dynamic_pointer_cast<StructType>(dst)) {
        if (auto s = std::dynamic_pointer_cast<StructType>(src)) {
            if (d->isClass && s->isClass) return s->inherits(d);
        }
    }
    if (auto dstIface = std::dynamic_pointer_cast<InterfaceType>(dst)) {
        if (auto srcStruct = std::dynamic_pointer_cast<StructType>(src)) {
            for (const auto& pair : dstIface->methods) {
                auto m = srcStruct->lookupMethod(pair.first);
                if (!m || !equal(m->signature, pair.second)) return false;
            }
            return true;
        }
    }
    return false;
}

bool comparable(const TypePtr& a, const TypePtr& b) {
    if (a == getInvalid() || b == getInvalid() || a == getJSON() || b == getJSON() ||
        a == getAny() || b == getAny() || a == getNull() || b == getNull()) {
        return true;
    }
    if (assignable(a, b) || assignable(b, a)) return true;
    if (isNumeric(a) && isNumeric(b)) return true;
    if (auto al = std::dynamic_pointer_cast<ListType>(a)) {
        if (auto bl = std::dynamic_pointer_cast<ListType>(b)) {
            return comparable(al->elem(), bl->elem());
        }
    }
    if (auto am = std::dynamic_pointer_cast<MapType>(a)) {
        if (auto bm = std::dynamic_pointer_cast<MapType>(b)) {
            return comparable(am->key(), bm->key()) && comparable(am->val(), bm->val());
        }
    }
    return false;
}

namespace {

std::string trim(std::string_view s) {
    size_t start = 0;
    while (start < s.size() && std::isspace(static_cast<unsigned char>(s[start]))) ++start;
    size_t end = s.size();
    while (end > start && std::isspace(static_cast<unsigned char>(s[end - 1]))) --end;
    return std::string(s.substr(start, end - start));
}

std::vector<std::string> splitTop(const std::string& s, char sep) {
    std::vector<std::string> parts;
    int depth = 0;
    size_t start = 0;
    for (size_t i = 0; i < s.size(); ++i) {
        char c = s[i];
        if (c == '[' || c == '(') depth++;
        else if (c == ']' || c == ')') depth--;
        else if (c == sep && depth == 0) {
            parts.push_back(s.substr(start, i - start));
            start = i + 1;
        }
    }
    parts.push_back(s.substr(start));
    return parts;
}

int matchParen(const std::string& s) {
    int depth = 0;
    for (size_t i = 0; i < s.size(); ++i) {
        if (s[i] == '(') depth++;
        else if (s[i] == ')') {
            depth--;
            if (depth == 0) return static_cast<int>(i);
        }
    }
    return -1;
}

} // namespace

TypePtr parseType(const std::string& raw, const std::unordered_map<std::string, TypePtr>* localTypes) {
    std::string s = trim(raw);
    if (s.empty()) return getInvalid();

    auto unionParts = splitTop(s, '|');
    if (unionParts.size() > 1) {
        std::vector<TypePtr> ts;
        for (const auto& part : unionParts) {
            ts.push_back(parseType(part, localTypes));
        }
        return newUnion(ts);
    }

    if (s == "int") return getInt();
    if (s == "float") return getFloat();
    if (s == "string") return getString();
    if (s == "bool") return getBool();
    if (s == "null") return getNull();
    if (s == "void") return getVoid();
    if (s == "blob") return getBlob();
    if (s == "secret") return getSecret();
    if (s == "json") return getJSON();
    if (s == "any") return getAny();
    if (s == "never") return getNever();

    if (localTypes) {
        auto it = localTypes->find(s);
        if (it != localTypes->end()) return it->second;
    }

    if (s.size() > 5 && s.rfind("list[", 0) == 0 && s.back() == ']') {
        auto inner = s.substr(5, s.size() - 6);
        return std::make_shared<ListType>(parseType(inner, localTypes));
    }
    if (s.size() > 4 && s.rfind("map[", 0) == 0 && s.back() == ']') {
        auto inner = s.substr(4, s.size() - 5);
        auto commaParts = splitTop(inner, ',');
        if (commaParts.size() == 2) {
            return std::make_shared<MapType>(parseType(commaParts[0], localTypes),
                                             parseType(commaParts[1], localTypes));
        }
    }

    return std::make_shared<Basic>(s);
}

TypePtr parseSig(const std::string& raw) {
    std::string sig = trim(raw);
    if (sig.empty()) {
        auto f = std::make_shared<FuncType>();
        f->dynamic = true;
        f->ret = getJSON();
        return f;
    }
    if (sig[0] != '(') {
        return parseType(sig);
    }
    std::vector<std::shared_ptr<FuncType>> sigs;
    for (const auto& altRaw : splitTop(sig, ';')) {
        std::string alt = trim(altRaw);
        int closeIdx = matchParen(alt);
        if (closeIdx < 0) continue;

        std::string paramSrc = alt.substr(1, closeIdx - 1);
        std::string rest = trim(alt.substr(closeIdx + 1));
        if (rest.rfind("->", 0) == 0) rest = trim(rest.substr(2));

        TypePtr ret = parseType(rest);
        auto f = std::make_shared<FuncType>();
        f->ret = ret;

        if (!trim(paramSrc).empty()) {
            for (const auto& psRaw : splitTop(paramSrc, ',')) {
                std::string ps = trim(psRaw);
                Param p;
                if (ps.rfind("...", 0) == 0) {
                    f->variadic = true;
                    ps = trim(ps.substr(3));
                } else if (ps.size() >= 3 && ps.compare(ps.size() - 3, 3, "...") == 0) {
                    f->variadic = true;
                    ps = trim(ps.substr(0, ps.size() - 3));
                }
                if (!ps.empty() && ps.back() == '?') {
                    p.optional = true;
                    ps = trim(ps.substr(0, ps.size() - 1));
                }
                p.type = parseType(ps);
                f->params.push_back(p);
            }
        }
        sigs.push_back(f);
    }

    if (sigs.empty()) {
        auto f = std::make_shared<FuncType>();
        f->dynamic = true;
        f->ret = getJSON();
        return f;
    }
    if (sigs.size() == 1) return sigs[0];

    auto ov = std::make_shared<Overloads>();
    ov->sigs = sigs;
    return ov;
}

namespace {
std::mutex gModMutex;
std::unordered_map<std::string, std::shared_ptr<ModuleType>> gModuleCache;
}

std::shared_ptr<ModuleType> moduleByName(const std::string& rawName) {
    std::string name = rawName;
    const auto& aliases = getModuleAliases();
    auto itAlias = aliases.find(name);
    if (itAlias != aliases.end()) name = itAlias->second;

    std::lock_guard<std::mutex> lock(gModMutex);
    auto itCached = gModuleCache.find(name);
    if (itCached != gModuleCache.end()) return itCached->second;

    const auto& allMods = getModuleMembers();
    auto itMod = allMods.find(name);
    if (itMod == allMods.end()) return nullptr;

    auto m = std::make_shared<ModuleType>();
    m->name = name;
    for (const auto& pair : itMod->second) {
        m->members[pair.first] = parseSig(pair.second);
    }
    gModuleCache[name] = m;
    return m;
}

std::shared_ptr<ModuleType> stdNamespace() {
    auto ns = std::make_shared<ModuleType>();
    ns->name = "std";
    for (const auto& pair : getModuleMembers()) {
        auto m = moduleByName(pair.first);
        if (m) ns->members[pair.first] = m;
    }
    for (const auto& pair : getModuleAliases()) {
        auto m = moduleByName(pair.first);
        if (m) ns->members[pair.first] = m;
    }
    return ns;
}

const std::unordered_map<std::string, TypePtr>& globalBuiltins() {
    static const auto g = [] {
        std::unordered_map<std::string, TypePtr> map;
        for (const auto& pair : getGlobalSigs()) {
            map[pair.first] = parseSig(pair.second);
        }
        return map;
    }();
    return g;
}

} // namespace pawl::typecheck

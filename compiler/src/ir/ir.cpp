// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "ir.h"
#include <sstream>
#include <iomanip>

namespace pawl::ir {

std::string_view typeToString(Type t) {
    switch (t) {
    case Type::Void: return "void";
    case Type::I8:   return "i8";
    case Type::I64:  return "i64";
    case Type::F64:  return "f64";
    case Type::Ptr:  return "ptr";
    }
    return "unknown";
}

std::string VReg::toString() const {
    std::ostringstream ss;
    ss << "v" << id << ":" << typeToString(type);
    return ss.str();
}

std::string_view opToString(Op op) {
    switch (op) {
    case Op::Nop:        return "nop";
    case Op::ConstInt:   return "const_int";
    case Op::ConstFloat: return "const_float";
    case Op::ConstBool:  return "const_bool";
    case Op::ConstStr:   return "const_str";
    case Op::Print:      return "print";
    case Op::Println:    return "println";
    case Op::ConcatStr:  return "concat_str";
    case Op::IntToStr:   return "int_to_str";
    case Op::IntToFloat: return "int_to_float";
    case Op::FloatToInt: return "float_to_int";
    case Op::Alloc:      return "alloc";
    case Op::Load:       return "load";
    case Op::Store:      return "store";
    case Op::CallPtr:    return "call_ptr";
    case Op::SetErr:     return "set_err";
    case Op::GetErr:     return "get_err";
    case Op::Copy:       return "copy";
    case Op::Add:        return "add";
    case Op::Sub:        return "sub";
    case Op::Mul:        return "mul";
    case Op::Div:        return "div";
    case Op::Mod:        return "mod";
    case Op::Neg:        return "neg";
    case Op::Not:        return "not";
    case Op::Eq:         return "eq";
    case Op::Ne:         return "ne";
    case Op::Lt:         return "lt";
    case Op::Le:         return "le";
    case Op::Gt:         return "gt";
    case Op::Ge:         return "ge";
    case Op::Jmp:        return "jmp";
    case Op::Branch:     return "branch";
    case Op::Call:       return "call";
    case Op::Ret:        return "ret";
    }
    return "unknown_op";
}

static std::string escapeString(const std::string& s) {
    std::ostringstream ss;
    ss << '"';
    for (char c : s) {
        if (c == '"') ss << "\\\"";
        else if (c == '\\') ss << "\\\\";
        else if (c == '\n') ss << "\\n";
        else if (c == '\t') ss << "\\t";
        else if (c == '\r') ss << "\\r";
        else ss << c;
    }
    ss << '"';
    return ss.str();
}

std::string Instruction::toString() const {
    std::ostringstream ss;
    switch (op) {
    case Op::ConstInt:
        ss << "  " << dst.toString() << " = const_int " << intVal;
        break;
    case Op::ConstFloat:
        ss << "  " << dst.toString() << " = const_float " << floatVal;
        break;
    case Op::ConstBool:
        ss << "  " << dst.toString() << " = const_bool " << (intVal != 0 ? 1 : 0);
        break;
    case Op::ConstStr:
        ss << "  " << dst.toString() << " = const_str " << escapeString(strVal);
        break;
    case Op::Print:
        ss << "  print " << src1.toString();
        break;
    case Op::Println:
        ss << "  println " << src1.toString();
        break;
    case Op::ConcatStr:
        ss << "  " << dst.toString() << " = concat_str " << src1.toString() << ", " << src2.toString();
        break;
    case Op::IntToStr:
        ss << "  " << dst.toString() << " = int_to_str " << src1.toString();
        break;
    case Op::IntToFloat:
    case Op::FloatToInt:
        ss << "  " << dst.toString() << " = " << opToString(op) << " " << src1.toString();
        break;
    case Op::Copy:
        ss << "  " << dst.toString() << " = copy " << src1.toString();
        break;
    case Op::Add:
    case Op::Sub:
    case Op::Mul:
    case Op::Div:
    case Op::Mod:
    case Op::Eq:
    case Op::Ne:
    case Op::Lt:
    case Op::Le:
    case Op::Gt:
    case Op::Ge:
        ss << "  " << dst.toString() << " = " << opToString(op) << " " << src1.toString() << ", " << src2.toString();
        break;
    case Op::Neg:
    case Op::Not:
        ss << "  " << dst.toString() << " = " << opToString(op) << " " << src1.toString();
        break;
    case Op::Jmp:
        ss << "  jmp " << (target ? target->name : "<?null>");
        break;
    case Op::Branch:
        ss << "  branch " << src1.toString()
           << ", then " << (target ? target->name : "<?null>")
           << ", else " << (elseTgt ? elseTgt->name : "<?null>");
        break;
    case Op::Call: {
        ss << "  ";
        if (dst.type != Type::Void) {
            ss << dst.toString() << " = ";
        }
        ss << "call @" << callName << "(";
        for (size_t i = 0; i < args.size(); ++i) {
            if (i > 0) ss << ", ";
            ss << args[i].toString();
        }
        ss << ")";
        break;
    }
    case Op::Alloc:
        ss << "  " << dst.toString() << " = alloc " << src1.toString() << " bytes";
        break;
    case Op::Load:
        ss << "  " << dst.toString() << " = load " << src1.toString() << "[" << intVal << "]";
        break;
    case Op::Store:
        ss << "  store " << src1.toString() << "[" << intVal << "] = " << src2.toString();
        break;
    case Op::CallPtr: {
        ss << "  ";
        if (dst.type != Type::Void) {
            ss << dst.toString() << " = ";
        }
        ss << "call_ptr " << src1.toString() << "(";
        for (size_t i = 0; i < args.size(); ++i) {
            if (i > 0) ss << ", ";
            ss << args[i].toString();
        }
        ss << ")";
        break;
    }
    case Op::SetErr:
        ss << "  set_err " << src1.toString();
        break;
    case Op::GetErr:
        ss << "  " << dst.toString() << " = get_err";
        break;
    case Op::Ret:
        if (src1.type != Type::Void && src1.id >= 0) {
            ss << "  ret " << src1.toString();
        } else {
            ss << "  ret";
        }
        break;
    default:
        ss << "  nop";
        break;
    }
    return ss.str();
}

std::string Function::toString() const {
    std::ostringstream ss;
    ss << "fn @" << name << "(";
    for (size_t i = 0; i < params.size(); ++i) {
        if (i > 0) ss << ", ";
        ss << params[i].toString();
    }
    ss << ") -> " << typeToString(returnType) << " {\n";
    for (const auto& b : blocks) {
        ss << b->name << ":\n";
        for (const auto& inst : b->instructions) {
            ss << inst->toString() << "\n";
        }
    }
    ss << "}\n";
    return ss.str();
}

std::string Program::toString() const {
    std::ostringstream ss;
    for (const auto& f : functions) {
        ss << f->toString() << "\n";
    }
    return ss.str();
}

} // namespace pawl::ir

// Copyright 2026 Pawl Programming Language
// SPDX-License-Identifier: Apache-2.0

#include "ir.h"
#include <sstream>
#include <iomanip>

namespace ulang::ir {

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
    case Op::MathUnary:  return "math_unary";
    case Op::MathBinary: return "math_binary";
    case Op::Alloc:      return "alloc";
    case Op::Load:       return "load";
    case Op::Store:      return "store";
    case Op::CallPtr:    return "call_ptr";
    case Op::SetErr:     return "set_err";
    case Op::GetErr:     return "get_err";
    case Op::CpuCli:     return "cpu_cli";
    case Op::CpuSti:     return "cpu_sti";
    case Op::CpuHlt:     return "cpu_hlt";
    case Op::CpuPause:   return "cpu_pause";
    case Op::CpuIn:      return "cpu_in";
    case Op::CpuOut:     return "cpu_out";
    case Op::CpuReadCr:  return "cpu_read_cr";
    case Op::CpuWriteCr: return "cpu_write_cr";
    case Op::CpuInvlpg:  return "cpu_invlpg";
    case Op::CpuRdmsr:   return "cpu_rdmsr";
    case Op::CpuWrmsr:   return "cpu_wrmsr";
    case Op::CpuCpuid:   return "cpu_cpuid";
    case Op::CpuLidt:    return "cpu_lidt";
    case Op::CpuIret:    return "cpu_iret";
    case Op::RawLoad:    return "raw_load";
    case Op::RawStore:   return "raw_store";
    case Op::LoadGlobal: return "load_global";
    case Op::StoreGlobal: return "store_global";
    case Op::FuncAddr:   return "func_addr";
    case Op::ClearErr:   return "clear_err";
    case Op::GetErrValue: return "get_err_value";
    case Op::SetErrType: return "set_err_type";
    case Op::GetErrType: return "get_err_type";
    case Op::Copy:       return "copy";
    case Op::Add:        return "add";
    case Op::Sub:        return "sub";
    case Op::Mul:        return "mul";
    case Op::Div:        return "div";
    case Op::Mod:        return "mod";
    case Op::Shl:        return "shl";
    case Op::Shr:        return "shr";
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
    case Op::MathUnary:
        ss << "  " << dst.toString() << " = " << callName << " " << src1.toString();
        break;
    case Op::MathBinary:
        ss << "  " << dst.toString() << " = " << callName << " " << src1.toString() << ", " << src2.toString();
        break;
    case Op::Copy:
        ss << "  " << dst.toString() << " = copy " << src1.toString();
        break;
    case Op::Add:
    case Op::Sub:
    case Op::Mul:
    case Op::Div:
    case Op::Mod:
    case Op::Shl:
    case Op::Shr:
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
    case Op::CpuCli:
    case Op::CpuSti:
    case Op::CpuHlt:
    case Op::CpuPause:
        ss << "  " << opToString(op);
        break;
    case Op::CpuIn:
        ss << "  " << dst.toString() << " = in" << intVal << " " << src1.toString();
        break;
    case Op::CpuOut:
        ss << "  out" << intVal << " " << src1.toString() << ", " << src2.toString();
        break;
    case Op::CpuReadCr:
        ss << "  " << dst.toString() << " = read_cr" << intVal;
        break;
    case Op::CpuWriteCr:
        ss << "  write_cr" << intVal << " " << src1.toString();
        break;
    case Op::CpuInvlpg:
        ss << "  invlpg " << src1.toString();
        break;
    case Op::CpuRdmsr:
        ss << "  " << dst.toString() << " = rdmsr " << src1.toString();
        break;
    case Op::CpuWrmsr:
        ss << "  wrmsr " << src1.toString() << ", " << src2.toString();
        break;
    case Op::CpuCpuid:
        ss << "  " << dst.toString() << " = cpuid" << intVal << " " << src1.toString() << ", " << src2.toString();
        break;
    case Op::CpuLidt:
        ss << "  lidt [" << src1.toString() << "]";
        break;
    case Op::CpuIret:
        ss << "  iret";
        break;
    case Op::RawLoad:
        ss << "  " << dst.toString() << " = raw_load" << intVal << " [" << src1.toString() << "]";
        break;
    case Op::RawStore:
        ss << "  raw_store" << intVal << " [" << src1.toString() << "] = " << src2.toString();
        break;
    case Op::LoadGlobal:
        ss << "  " << dst.toString() << " = load_global @" << callName;
        break;
    case Op::StoreGlobal:
        ss << "  store_global @" << callName << " = " << src1.toString();
        break;
    case Op::FuncAddr:
        ss << "  " << dst.toString() << " = func_addr @" << callName;
        break;
    case Op::ClearErr:
        ss << "  clear_err";
        break;
    case Op::GetErrValue:
        ss << "  " << dst.toString() << " = get_err_value";
        break;
    case Op::SetErrType:
        ss << "  set_err_type " << intVal;
        break;
    case Op::GetErrType:
        ss << "  " << dst.toString() << " = get_err_type";
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
    for (const auto& g : globals) {
        ss << "global @" << g.name << " = " << g.initValue << "\n";
    }
    for (const auto& f : functions) {
        ss << f->toString() << "\n";
    }
    return ss.str();
}

} // namespace ulang::ir

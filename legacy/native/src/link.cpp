// Pawl bootstrap compiler - built-in PE32+ / ELF64 linkers.
// SPDX-License-Identifier: Apache-2.0
#include "link.h"

#include <cstring>
#include <fstream>
#include <map>
#include <vector>

uint32_t alignUp(uint32_t n, uint32_t a) { return (n + a - 1) / a * a; }

void put16(std::vector<uint8_t>& b, uint32_t v) { for (int i = 0; i < 2; i++) b.push_back(uint8_t(v >> (8 * i))); }
void put32(std::vector<uint8_t>& b, uint32_t v) { for (int i = 0; i < 4; i++) b.push_back(uint8_t(v >> (8 * i))); }
void put64(std::vector<uint8_t>& b, uint64_t v) { for (int i = 0; i < 8; i++) b.push_back(uint8_t(v >> (8 * i))); }

void writeFile(const std::string& path, const std::vector<uint8_t>& data, bool exec) {
    std::ofstream f(path, std::ios::binary);
    if (!f) fail("cannot open output file '" + path + "'");
    f.write(reinterpret_cast<const char*>(data.data()), std::streamsize(data.size()));
    if (!f) fail("cannot write output file '" + path + "'");
    (void)exec;
}

void patch32(Module& m, size_t at, int32_t v) {
    if (at + 4 > m.text.size()) fail("internal: patch out of range");
    uint32_t u = uint32_t(v);
    for (int i = 0; i < 4; i++) m.text[at + i] = uint8_t(u >> (8 * i));
}

uint32_t sym(const Module& m, const std::string& name) {
    auto it = m.symbols.find(name);
    if (it == m.symbols.end()) fail("linker: unresolved symbol '" + name + "'");
    return it->second;
}

// Resolve all rel32 call sites (text-relative).
void patchCalls(Module& m) {
    for (auto& c : m.calls) {
        int32_t rel = int32_t(int64_t(sym(m, c.target)) - int64_t(c.at + 4));
        patch32(m, c.at, rel);
    }
}

// ---------------------------------------------------------------------------
// PE32+
// ---------------------------------------------------------------------------
const uint32_t PE_FILE_ALIGN = 0x200;
const uint32_t PE_SECT_ALIGN = 0x1000;
const uint64_t PE_IMAGE_BASE_LOW = 0x140000000ull;

struct PELayout {
    uint32_t textRVA = 0x1000;
    uint32_t virtTextLen = 0;
    uint32_t rawTextLen = 0;
    uint32_t rdataRVA = 0;
    uint32_t rdataLen = 0;
    uint32_t rawRDataLen = 0;
    uint32_t virtRDataLen = 0;
    uint32_t constRVA = 0;
    uint32_t bssRVA = 0;
    uint32_t bssSize = 0x200000;
    std::map<std::string, uint32_t> iat;
};

uint32_t peHeaderArea(const std::vector<std::string>& imports) {
    uint32_t n = uint32_t(imports.size());
    uint32_t iatSize = (n + 1) * 8;
    uint32_t idtOffset = iatSize;
    uint32_t idtSize = 40;
    uint32_t iltOffset = (idtOffset + idtSize + 7) & ~7u;
    uint32_t iltSize = (n + 1) * 8;
    uint32_t dllNameOffset = iltOffset + iltSize;
    uint32_t dllNameSize = 16;
    uint32_t hintOffset = dllNameOffset + dllNameSize;
    uint32_t hintLen = 0;
    for (auto& fn : imports) {
        hintLen += 2 + uint32_t(fn.size()) + 1;
        if (hintLen % 2) hintLen++;
    }
    if (hintLen % 16) hintLen += 16 - (hintLen % 16);
    return hintOffset + hintLen;
}

PELayout layoutPE(uint32_t textLen, uint32_t rodataLen, const std::vector<std::string>& imports) {
    PELayout L;
    L.rawTextLen = alignUp(textLen, PE_FILE_ALIGN);
    L.virtTextLen = alignUp(textLen, PE_SECT_ALIGN);
    if (!L.virtTextLen) L.virtTextLen = PE_SECT_ALIGN;
    L.rdataRVA = L.textRVA + L.virtTextLen;

    uint32_t hdr = peHeaderArea(imports);
    for (size_t i = 0; i < imports.size(); i++) L.iat[imports[i]] = L.rdataRVA + uint32_t(i * 8);

    L.constRVA = L.rdataRVA + hdr;
    L.rdataLen = hdr + 8 + rodataLen; // 8-byte newline block precedes the pool
    L.rawRDataLen = alignUp(L.rdataLen, PE_FILE_ALIGN);
    L.virtRDataLen = alignUp(L.rdataLen, PE_SECT_ALIGN);
    if (!L.virtRDataLen) L.virtRDataLen = PE_SECT_ALIGN;
    L.bssRVA = L.rdataRVA + L.virtRDataLen;
    return L;
}

void writePE(Module& m, const std::string& path) {
    if (!m.windows) fail("internal: writePE called for a non-Windows module");
    std::vector<std::string> imports = {"ExitProcess", "GetStdHandle", "WriteFile"};
    PELayout L = layoutPE(uint32_t(m.text.size()), uint32_t(m.rodata.size()), imports);

    // Patch PE runtime references.
    for (auto& c : m.iatCalls) {
        auto it = L.iat.find(c.second);
        if (it == L.iat.end()) fail("linker: unknown import '" + c.second + "'");
        int32_t disp = int32_t(int64_t(it->second) - int64_t(L.textRVA + c.first + 4));
        patch32(m, c.first, disp);
    }
    for (size_t at : m.stdoutRefs) {
        patch32(m, at, int32_t(int64_t(L.bssRVA) - int64_t(L.textRVA + at + 4)));
    }
    for (size_t at : m.allocRefs) {
        patch32(m, at, int32_t(int64_t(L.bssRVA + 8) - int64_t(L.textRVA + at + 4)));
    }
    for (size_t at : m.newlineRefs) {
        patch32(m, at, int32_t(int64_t(L.constRVA) - int64_t(L.textRVA + at + 4)));
    }
    for (size_t at : m.allocInitImm) {
        uint64_t v = PE_IMAGE_BASE_LOW + L.bssRVA + 0x10000;
        for (int i = 0; i < 8; i++) m.text[at + i] = uint8_t(v >> (8 * i));
    }
    for (auto& s : m.strings) {
        int32_t disp = int32_t(int64_t(L.constRVA + 8 + s.off) - int64_t(L.textRVA + s.at + 4));
        patch32(m, s.at, disp);
    }
    patchCalls(m);

    // Build .rdata.
    uint32_t hdr = peHeaderArea(imports);
    std::vector<uint8_t> rdata(hdr, 0);
    uint32_t iatSize = uint32_t((imports.size() + 1) * 8);
    uint32_t idtOffset = iatSize;
    uint32_t iltOffset = (idtOffset + 40 + 7) & ~7u;
    uint32_t dllNameOffset = iltOffset + iatSize;
    uint32_t hintOffset = dllNameOffset + 16;

    uint32_t cur = hintOffset;
    std::vector<uint32_t> hintRVAs;
    for (auto& fn : imports) {
        hintRVAs.push_back(L.rdataRVA + cur);
        cur += 2;
        for (char ch : fn) rdata[cur++] = uint8_t(ch);
        rdata[cur++] = 0;
        if (cur % 2) rdata[cur++] = 0;
    }

    auto wr32 = [&](uint32_t at, uint32_t v) {
        for (int i = 0; i < 4; i++) rdata[at + i] = uint8_t(v >> (8 * i));
    };
    auto wr64 = [&](uint32_t at, uint64_t v) {
        for (int i = 0; i < 8; i++) rdata[at + i] = uint8_t(v >> (8 * i));
    };

    // IAT
    for (size_t i = 0; i < imports.size(); i++) wr64(uint32_t(i * 8), hintRVAs[i]);
    wr64(uint32_t(imports.size() * 8), 0);
    // Import directory table
    wr32(idtOffset + 0, L.rdataRVA + iltOffset); // OriginalFirstThunk
    wr32(idtOffset + 4, 0);
    wr32(idtOffset + 8, 0);
    wr32(idtOffset + 12, L.rdataRVA + dllNameOffset);
    wr32(idtOffset + 16, L.rdataRVA);            // FirstThunk
    // ILT
    for (size_t i = 0; i < imports.size(); i++) wr64(iltOffset + uint32_t(i * 8), hintRVAs[i]);
    wr64(iltOffset + uint32_t(imports.size() * 8), 0);
    // DLL name
    const char* dll = "KERNEL32.dll";
    for (size_t i = 0; i < std::strlen(dll); i++) rdata[dllNameOffset + i] = uint8_t(dll[i]);
    rdata[dllNameOffset + std::strlen(dll)] = 0;

    // Newline block + string pool.
    if (rdata.size() != hdr) fail("internal: PE rdata header size mismatch");
    std::vector<uint8_t> constants = rdata;
    constants.resize(hdr + 8, 0);
    constants[hdr + 0] = '\r';
    constants[hdr + 1] = '\n';
    constants.insert(constants.end(), m.rodata.begin(), m.rodata.end());
    rdata = constants;

    rdata.resize(L.rawRDataLen, 0);
    std::vector<uint8_t> text = m.text;
    text.resize(L.rawTextLen, 0);

    // Headers: DOS header (0x00..0x3F), DOS stub (0x40..0x7F), PE at 0x80.
    std::vector<uint8_t> out(0x80, 0);
    out[0] = 'M'; out[1] = 'Z';
    {
        uint32_t v = 0x80; // e_lfanew
        for (int i = 0; i < 4; i++) out[0x3C + i] = uint8_t(v >> (8 * i));
    }
    const char* stub = "This program cannot be run in DOS mode.\r\r\n$";
    for (size_t i = 0; i < std::strlen(stub) && 0x40 + i < 0x80; i++) out[0x40 + i] = uint8_t(stub[i]);

    put32(out, 0x00004550); // "PE\0\0"
    put16(out, 0x8664);     // machine AMD64
    put16(out, 3);          // number of sections
    put32(out, 0);          // timestamp
    put32(out, 0);          // symbol table
    put32(out, 0);          // symbols
    put16(out, 0xF0);       // optional header size
    put16(out, 0x0222);     // executable | large address aware

    put16(out, 0x020B);     // PE32+
    out.push_back(1); out.push_back(0);
    put32(out, L.rawTextLen);
    put32(out, L.rawRDataLen);
    put32(out, 0);          // uninitialized data size
    put32(out, L.textRVA);  // entry point
    put32(out, L.textRVA);  // base of code
    put64(out, PE_IMAGE_BASE_LOW);
    put32(out, PE_SECT_ALIGN);
    put32(out, PE_FILE_ALIGN);
    put16(out, 6); put16(out, 0);
    put16(out, 0); put16(out, 0);
    put16(out, 6); put16(out, 0);
    put32(out, 0);          // win32 version
    put32(out, L.bssRVA + alignUp(L.bssSize, PE_SECT_ALIGN)); // SizeOfImage
    put32(out, 0x400);      // SizeOfHeaders
    put32(out, 0);          // checksum
    put16(out, 3);          // console subsystem
    put16(out, 0x8160);     // DLL characteristics
    put64(out, 0x100000); put64(out, 0x1000);
    put64(out, 0x100000); put64(out, 0x1000);
    put32(out, 0);          // loader flags
    put32(out, 16);         // number of RVA/sizes

    for (int i = 0; i < 16; i++) {
        if (i == 1) {
            put32(out, L.rdataRVA + idtOffset); put32(out, 40);
        } else if (i == 12) {
            put32(out, L.rdataRVA); put32(out, iatSize);
        } else {
            put32(out, 0); put32(out, 0);
        }
    }

    auto section = [&](const char* name, uint32_t vsize, uint32_t rva, uint32_t rawsize,
                       uint32_t rawptr, uint32_t chars) {
        char buf[8] = {0};
        std::strncpy(buf, name, 8);
        for (int i = 0; i < 8; i++) out.push_back(uint8_t(buf[i]));
        put32(out, vsize);
        put32(out, rva);
        put32(out, rawsize);
        put32(out, rawptr);
        put32(out, 0); put32(out, 0);
        put16(out, 0); put16(out, 0);
        put32(out, chars);
    };
    section(".text", L.virtTextLen, L.textRVA, L.rawTextLen, 0x400, 0x60000020);
    section(".rdata", L.virtRDataLen, L.rdataRVA, L.rawRDataLen, 0x400 + L.rawTextLen, 0x40000040);
    section(".bss", alignUp(L.bssSize, PE_SECT_ALIGN), L.bssRVA, 0, 0, 0xC0000080);

    out.resize(0x400, 0);
    out.insert(out.end(), text.begin(), text.end());
    out.insert(out.end(), rdata.begin(), rdata.end());
    writeFile(path, out, true);
}

// ---------------------------------------------------------------------------
// ELF64 (Linux x86-64)
// ---------------------------------------------------------------------------
const uint64_t ELF_BASE = 0x400000;
const uint64_t ELF_TEXT = 0x401000;
const uint64_t ELF_RODATA = 0x410000;
const uint64_t ELF_RODATA_OFF = 0x10000;

void writeELF(Module& m, const std::string& path) {
    if (m.windows) fail("internal: writeELF called for a Windows module");
    if (m.text.size() > 0xF000) fail("ELF text section too large for the bootstrap layout");

    patchCalls(m);
    for (auto& s : m.strings) {
        int32_t disp = int32_t(int64_t(ELF_RODATA + 2 + s.off) - int64_t(ELF_TEXT + s.at + 4));
        patch32(m, s.at, disp);
    }

    std::vector<uint8_t> rodata;
    rodata.push_back('\n');
    rodata.push_back(0);
    rodata.insert(rodata.end(), m.rodata.begin(), m.rodata.end());

    std::vector<uint8_t> out;
    // ELF header
    const uint8_t ident[16] = {0x7F, 'E', 'L', 'F', 2, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0};
    out.insert(out.end(), ident, ident + 16);
    put16(out, 2);              // ET_EXEC
    put16(out, 0x3E);           // AMD64
    put32(out, 1);
    put64(out, ELF_TEXT);       // entry
    put64(out, 64);             // phoff
    put64(out, 0);              // shoff
    put32(out, 0);
    put16(out, 64);             // ehsize
    put16(out, 56);             // phentsize
    put16(out, 2);              // phnum
    put16(out, 64); put16(out, 0); put16(out, 0);

    uint64_t textSegSize = 0x1000 + m.text.size();
    // PT_LOAD text (RX)
    put32(out, 1); put32(out, 5);
    put64(out, 0);
    put64(out, ELF_BASE);
    put64(out, ELF_BASE);
    put64(out, textSegSize);
    put64(out, textSegSize);
    put64(out, 0x1000);

    // PT_LOAD rodata (R)
    put32(out, 1); put32(out, 4);
    put64(out, ELF_RODATA_OFF);
    put64(out, ELF_RODATA);
    put64(out, ELF_RODATA);
    put64(out, rodata.size());
    put64(out, rodata.size());
    put64(out, 0x1000);

    out.resize(0x1000, 0);
    out.insert(out.end(), m.text.begin(), m.text.end());
    out.resize(ELF_RODATA_OFF, 0);
    out.insert(out.end(), rodata.begin(), rodata.end());
    writeFile(path, out, true);
}

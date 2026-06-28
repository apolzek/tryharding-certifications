---
title: RISC-V Foundational Associate (RVFA)
markmap:
  colorFreezeLevel: 2
---

<!--
  ACRONYM: RVFA = "RISC-V Foundational Associate", a Linux Foundation
  certification validating entry-level RISC-V hardware and software knowledge.
  Domains and weights below are the official exam blueprint.
  Source: training.linuxfoundation.org/certification/linux-foundation-risc-v-foundational-associate/
-->

# RVFA

## RISC-V Overview (10%)
### Background and ecosystem
- History of RISC-V: the free and open ISA
  - Origin: UC Berkeley (Krste Asanović, Patterson, 2010)
  - License: BSD open ISA (royalty-free, no per-core fee)
  - Frozen base + optional ratified extensions model
- RISC-V International
  - Swiss non-profit governing body (riscv.org)
  - Technical Steering Committee (TSC) ratifies specs
  - Membership tiers: Premier / Strategic / Community
- RISC-V documentation
  - Volume I: Unprivileged ISA spec
  - Volume II: Privileged Architecture spec
  - ABI spec (psABI: lp64/ilp32)
- Contribute to RISC-V
  - GitHub: riscv/riscv-isa-manual
  - Task Groups (TG) and Special Interest Groups (SIG)
  - Public review / ratification process

## RISC-V Instruction Set Architecture (35%)
### The RISC-V ISA in depth
- RV32I and RV64I base integer ISAs
  - RV32I: 32-bit XLEN, 47 base instructions
  - RV64I: 64-bit XLEN, adds *.w/*.d (addw, ld, sd)
  - RV32E: 16-register embedded subset
  - 32 general-purpose registers x0-x31
    - x0 (zero): hardwired to 0
    - x1 (ra): return address
    - x2 (sp): stack pointer
    - x8 (s0/fp): frame pointer
    - x10-x17 (a0-a7): args / return values
  - Program counter: pc register
- Instruction formats
  - R-type: add, sub, and, or, xor, sll, slt
  - I-type: addi, andi, ori, slti, jalr
  - S-type: sw, sh, sb (store)
  - B-type: beq, bne, blt, bge (branch)
  - U-type: lui, auipc
  - J-type: jal
  - Branching
    - beq rs1, rs2, label
    - bne / blt / bge / bltu / bgeu
    - jal rd, offset  /  jalr rd, rs1, imm
  - Accessing memory
    - Load: lw / lh / lb / lhu / lbu
    - Store: sw / sh / sb
    - Little-endian byte order
  - Accessing data structures
    - Base+offset addressing: lw rd, imm(rs1)
    - la pseudo-instruction for symbol address
    - auipc + addi for PC-relative
- Modular ISA structure
  - Core ratified extensions (M, C, F, D, A)
    - M: mul, mulh, div, rem
    - C: 16-bit compressed (c.addi, c.lw)
    - F: single-precision float (flw, fadd.s)
    - D: double-precision float (fld, fadd.d)
    - A: atomics (lr.w, sc.w, amoadd.w)
  - Other extensions
    - Zicsr: CSR instructions (csrrw, csrrs)
    - Zifencei: fence.i instruction
    - V: vector extension (RVV 1.0)
    - B: bit-manipulation (Zba/Zbb/Zbc/Zbs)
    - G = IMAFD_Zicsr_Zifencei (general-purpose)
- Privilege modes, system calls, CSRs
  - Machine mode (M): mode 0b11, highest privilege
  - Supervisor mode (S): mode 0b01, OS kernel
  - User mode (U): mode 0b00, applications
  - System calls: ecall, ebreak
  - CSR registers
    - mstatus: global status / MIE / MPP bits
    - mtvec: trap-handler base address
    - mepc: machine exception PC
    - mcause: trap cause code
    - mie / mip: interrupt enable / pending
    - mhartid: hardware thread ID
    - satp: supervisor address translation (S-mode)
  - CSR access: csrrw / csrrs / csrrc
  - Exceptions and interrupt handling
    - Trap delegation: medeleg / mideleg
    - mret / sret to return from trap
    - Synchronous (exception) vs asynchronous (interrupt)
- Memory model, cache management, virtual memory management
  - RVWMO: RISC-V Weak Memory Ordering
  - fence instruction for ordering
  - Virtual memory: Sv32 / Sv39 / Sv48 page tables
  - 4 KiB pages; PTE (Page Table Entry) bits R/W/X/V
  - PMP: Physical Memory Protection (pmpcfg/pmpaddr)

## Assembly Language for RISC-V (25%)
### Writing and tuning RISC-V assembly
- RISC-V assembly syntax and features
  - Directives: .text, .data, .globl, .word, .byte
  - Labels and pseudo-instructions: li, mv, j, ret, nop, call
  - CSR access
    - csrr rd, csr  /  csrw csr, rs1
    - csrrs / csrrc for set/clear bits
  - GNU assembler (as) syntax via riscv64-unknown-elf-as
- Writing and debugging assembly code
  - GDB: riscv64-unknown-elf-gdb, breakpoints, stepi
  - objdump -d disassembly
  - QEMU + GDB remote (target remote :1234)
- Performance assessment of assembly implementations
  - Instruction count / cycles per instruction (CPI)
  - Pipeline hazards and branch prediction
  - perf counters via mcycle / minstret CSRs
- Converting higher-level code to assembly language
  - gcc -S to emit .s assembly
  - Mapping if/loops to beq/bne + jal
  - Function prologue/epilogue (sp adjust, ra save)

## High Level Languages for RISC-V: C Programming (15%)
### From C to RISC-V
- RISC-V tools
  - Compilers, debuggers, simulators
    - GCC: riscv64-unknown-elf-gcc / riscv-gnu-toolchain
    - LLVM/Clang --target=riscv64
    - GDB, QEMU (qemu-system-riscv64), Spike ISA sim
  - Performance tools, OSes, SDKs
    - perf, gprof
    - Linux, FreeRTOS, Zephyr RTOS
    - SiFive Freedom SDK, ESP-IDF (ESP32-C3)
- Calling conventions (ABIs), the stack, and disassembly
  - ILP32 / LP64 (and *f/*d hard-float variants)
  - Arg registers a0-a7, return a0/a1
  - Caller-saved t0-t6 vs callee-saved s0-s11
  - Stack grows downward; 16-byte aligned sp
  - Disassembly: objdump -d / -S
- Inline assembly techniques
  - GCC __asm__ volatile ("..." : outputs : inputs : clobbers)
  - Reading a CSR via inline asm (csrr)

## RISC-V Operating Systems & Tools (15%)
### Systems software on RISC-V
- Operating system fundamentals
  - Basic functionality implemented in assembly
    - Boot/reset vector at 0x1000 / 0x80000000
    - Trap vector setup (write mtvec)
    - Context switch: save/restore x1-x31
  - Timer interrupts via mtime / mtimecmp (CLINT)
- Firmware basics for RISC-V platforms
  - OpenSBI (Supervisor Binary Interface) M-mode firmware
  - SBI ecall interface to S-mode
  - U-Boot bootloader; device tree (.dts/.dtb)
- Microcontrollers versus application processors
  - MCU: SiFive E-series, no MMU, M+U modes
  - App processor: SiFive U-series, MMU, M+S+U
- Running applications in general purpose operating systems
  - Linux on RISC-V (riscv64 port, Debian/Fedora)
  - glibc / musl C libraries
  - ELF binaries via riscv64 toolchain

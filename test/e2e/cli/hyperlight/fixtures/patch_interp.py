#!/usr/bin/env python3
import struct
import sys

if len(sys.argv) != 2:
    print(f"Usage: {sys.argv[0]} <elf-binary>", file=sys.stderr)
    sys.exit(1)

path = sys.argv[1]
with open(path, "rb") as f:
    data = bytearray(f.read())

e_phoff,     = struct.unpack_from("<Q", data, 32)
e_phentsize, = struct.unpack_from("<H", data, 54)
e_phnum,     = struct.unpack_from("<H", data, 56)

for i in range(e_phnum):
    off = e_phoff + i * e_phentsize
    p_type, = struct.unpack_from("<I", data, off)
    if p_type == 3:
        struct.pack_into("<I", data, off, 0)
        print(f"Patched PT_INTERP -> PT_NULL at program header index {i}")

with open(path, "wb") as f:
    f.write(data)

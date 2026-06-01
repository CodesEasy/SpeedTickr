package icon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
)

// Windows resource type IDs.
const (
	rtIcon      = 3  // RT_ICON: one image's DIB
	rtGroupIcon = 14 // RT_GROUP_ICON: the directory that ties the images together
	langEnUS    = 0x0409
)

// Supported COFF machine types and their RVA (no-base) relocation kinds.
const (
	MachineAMD64 = 0x8664
	MachineARM64 = 0xaa64
	MachineI386  = 0x14c
)

func relocType(machine uint16) (uint16, error) {
	switch machine {
	case MachineAMD64:
		return 0x0003, nil // IMAGE_REL_AMD64_ADDR32NB
	case MachineARM64:
		return 0x0002, nil // IMAGE_REL_ARM64_ADDR32NB
	case MachineI386:
		return 0x0007, nil // IMAGE_REL_I386_DIR32NB
	default:
		return 0, fmt.Errorf("icon: unsupported COFF machine 0x%x", machine)
	}
}

// SYSO encodes imgs into a COFF object (.syso) carrying a single application icon
// resource. Dropped into the main package's directory, the Go linker merges its
// .rsrc section into the Windows executable, so Explorer and the taskbar show the
// icon for the file itself. The lowest-ID RT_GROUP_ICON (1) becomes the app icon.
//
// machine must match the build's GOARCH (see the Machine* constants); name the
// output file accordingly (e.g. rsrc_windows_amd64.syso) so it links only there.
func SYSO(machine uint16, imgs []*image.RGBA) ([]byte, error) {
	if len(imgs) == 0 {
		return nil, fmt.Errorf("icon: SYSO needs at least one image")
	}
	rt, err := relocType(machine)
	if err != nil {
		return nil, err
	}
	le := binary.LittleEndian
	n := len(imgs)

	// .rsrc$02 — raw resource bytes: each icon's DIB, then the group directory.
	// Offsets recorded here are filled into the data entries and fixed up by a
	// relocation against the .rsrc$02 section base.
	var data bytes.Buffer
	align4 := func(b *bytes.Buffer) {
		for b.Len()%4 != 0 {
			b.WriteByte(0)
		}
	}
	dibs := make([][]byte, n)
	iconOff := make([]uint32, n)
	for i, im := range imgs {
		align4(&data)
		dibs[i] = DIB(im)
		iconOff[i] = uint32(data.Len())
		data.Write(dibs[i])
	}
	align4(&data)
	grpOff := uint32(data.Len())
	data.Write(groupDir(imgs, dibs))

	// Layout of the directory tree within .rsrc$01 (offsets relative to its start,
	// which the linker keeps as the start of the merged .rsrc).
	const dirHdr, dirEnt, dataEnt = 16, 8, 16
	rootOff := 0
	iconDirOff := rootOff + dirHdr + 2*dirEnt
	grpDirOff := iconDirOff + dirHdr + n*dirEnt
	langOff := grpDirOff + dirHdr + 1*dirEnt // n icon lang dirs, then 1 group lang dir
	dataEntOff := langOff + (n+1)*(dirHdr+dirEnt)

	var dir bytes.Buffer
	writeDirHeader := func(named, id int) {
		_ = binary.Write(&dir, le, uint32(0)) // characteristics
		_ = binary.Write(&dir, le, uint32(0)) // timestamp
		_ = binary.Write(&dir, le, uint16(0)) // major
		_ = binary.Write(&dir, le, uint16(0)) // minor
		_ = binary.Write(&dir, le, uint16(named))
		_ = binary.Write(&dir, le, uint16(id))
	}
	writeDirEntry := func(idOrName uint32, offset int, isDir bool) {
		v := uint32(offset)
		if isDir {
			v |= 0x80000000
		}
		_ = binary.Write(&dir, le, idOrName)
		_ = binary.Write(&dir, le, v)
	}

	// Root: type RT_ICON then RT_GROUP_ICON (entries must be sorted by ID).
	writeDirHeader(0, 2)
	writeDirEntry(rtIcon, iconDirOff, true)
	writeDirEntry(rtGroupIcon, grpDirOff, true)

	// RT_ICON: one named-by-ID entry per image (IDs 1..n), each → its language dir.
	writeDirHeader(0, n)
	for i := 0; i < n; i++ {
		writeDirEntry(uint32(i+1), langOff+i*(dirHdr+dirEnt), true)
	}

	// RT_GROUP_ICON: a single group, ID 1 → its language dir.
	writeDirHeader(0, 1)
	writeDirEntry(1, langOff+n*(dirHdr+dirEnt), true)

	// Language dirs: each holds one entry (en-US) → a data entry (leaf, no high bit).
	for i := 0; i < n; i++ {
		writeDirHeader(0, 1)
		writeDirEntry(langEnUS, dataEntOff+i*dataEnt, false)
	}
	writeDirHeader(0, 1)
	writeDirEntry(langEnUS, dataEntOff+n*dataEnt, false)

	// Data entries. Each OffsetToData is an offset into .rsrc$02 that a relocation
	// turns into a final RVA; record where each one sits for the relocation table.
	relocVA := make([]uint32, 0, n+1)
	writeDataEntry := func(off, size uint32) {
		relocVA = append(relocVA, uint32(dir.Len())) // OffsetToData is the first field
		_ = binary.Write(&dir, le, off)
		_ = binary.Write(&dir, le, size)
		_ = binary.Write(&dir, le, uint32(0)) // code page
		_ = binary.Write(&dir, le, uint32(0)) // reserved
	}
	for i := 0; i < n; i++ {
		writeDataEntry(iconOff[i], uint32(len(dibs[i])))
	}
	writeDataEntry(grpOff, uint32(data.Len())-grpOff)

	rsrc01 := dir.Bytes()
	rsrc02 := data.Bytes()

	// Relocations for .rsrc$01: one per data entry, all against symbol 0 (the
	// .rsrc$02 section base).
	var reloc bytes.Buffer
	for _, va := range relocVA {
		_ = binary.Write(&reloc, le, va)        // VirtualAddress
		_ = binary.Write(&reloc, le, uint32(0)) // SymbolTableIndex → symbol 0
		_ = binary.Write(&reloc, le, rt)        // Type
	}

	// File layout: header, 2 section headers, then section data/relocs, symbols.
	const fileHdr, secHdr = 20, 40
	off01 := fileHdr + 2*secHdr
	offReloc := off01 + len(rsrc01)
	off02 := offReloc + reloc.Len()
	symPtr := off02 + len(rsrc02)

	var buf bytes.Buffer
	// IMAGE_FILE_HEADER.
	_ = binary.Write(&buf, le, machine)
	_ = binary.Write(&buf, le, uint16(2))      // NumberOfSections
	_ = binary.Write(&buf, le, uint32(0))      // TimeDateStamp
	_ = binary.Write(&buf, le, uint32(symPtr)) // PointerToSymbolTable
	_ = binary.Write(&buf, le, uint32(2))      // NumberOfSymbols (section symbol + aux)
	_ = binary.Write(&buf, le, uint16(0))      // SizeOfOptionalHeader
	_ = binary.Write(&buf, le, uint16(0))      // Characteristics

	const scnInitRead = 0x40000040 // CNT_INITIALIZED_DATA | MEM_READ
	writeSection := func(name string, rawSize, rawPtr, relPtr, relCount int) {
		var nm [8]byte
		copy(nm[:], name)
		buf.Write(nm[:])
		_ = binary.Write(&buf, le, uint32(0))        // VirtualSize
		_ = binary.Write(&buf, le, uint32(0))        // VirtualAddress
		_ = binary.Write(&buf, le, uint32(rawSize))  // SizeOfRawData
		_ = binary.Write(&buf, le, uint32(rawPtr))   // PointerToRawData
		_ = binary.Write(&buf, le, uint32(relPtr))   // PointerToRelocations
		_ = binary.Write(&buf, le, uint32(0))        // PointerToLinenumbers
		_ = binary.Write(&buf, le, uint16(relCount)) // NumberOfRelocations
		_ = binary.Write(&buf, le, uint16(0))        // NumberOfLinenumbers
		_ = binary.Write(&buf, le, uint32(scnInitRead))
	}
	writeSection(".rsrc$01", len(rsrc01), off01, offReloc, len(relocVA))
	writeSection(".rsrc$02", len(rsrc02), off02, 0, 0)

	buf.Write(rsrc01)
	buf.Write(reloc.Bytes())
	buf.Write(rsrc02)

	// Symbol table: a static section symbol for .rsrc$02 plus its aux record, so the
	// relocations resolve to that section's base.
	var sym [8]byte
	copy(sym[:], ".rsrc$02")
	buf.Write(sym[:])
	_ = binary.Write(&buf, le, uint32(0))           // Value
	_ = binary.Write(&buf, le, int16(2))            // SectionNumber (.rsrc$02)
	_ = binary.Write(&buf, le, uint16(0))           // Type
	buf.WriteByte(3)                                // StorageClass = STATIC
	buf.WriteByte(1)                                // NumberOfAuxSymbols
	_ = binary.Write(&buf, le, uint32(len(rsrc02))) // aux: section length
	_ = binary.Write(&buf, le, uint16(0))           // aux: NumberOfRelocations
	_ = binary.Write(&buf, le, uint16(0))           // aux: NumberOfLinenumbers
	_ = binary.Write(&buf, le, uint32(0))           // aux: CheckSum
	_ = binary.Write(&buf, le, uint16(2))           // aux: Number (.rsrc$02)
	buf.WriteByte(0)                                // aux: Selection
	buf.Write([]byte{0, 0, 0})                      // aux: padding to 18 bytes

	_ = binary.Write(&buf, le, uint32(4)) // string table: size only (empty)

	return buf.Bytes(), nil
}

// groupDir builds the GRPICONDIR that an RT_GROUP_ICON resource holds: a header
// plus one 14-byte entry per image, each pointing at RT_ICON resource ID i+1.
func groupDir(imgs []*image.RGBA, dibs [][]byte) []byte {
	le := binary.LittleEndian
	var b bytes.Buffer
	_ = binary.Write(&b, le, uint16(0))         // idReserved
	_ = binary.Write(&b, le, uint16(1))         // idType: icon
	_ = binary.Write(&b, le, uint16(len(imgs))) // idCount
	for i, im := range imgs {
		bb := im.Bounds()
		b.WriteByte(byte(bb.Dx() % 256))               // bWidth (0 means 256)
		b.WriteByte(byte(bb.Dy() % 256))               // bHeight
		b.WriteByte(0)                                 // bColorCount
		b.WriteByte(0)                                 // bReserved
		_ = binary.Write(&b, le, uint16(1))            // wPlanes
		_ = binary.Write(&b, le, uint16(32))           // wBitCount
		_ = binary.Write(&b, le, uint32(len(dibs[i]))) // dwBytesInRes
		_ = binary.Write(&b, le, uint16(i+1))          // nID: RT_ICON resource ID
	}
	return b.Bytes()
}

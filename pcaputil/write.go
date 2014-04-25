// Copyright 2012 Google, Inc. All rights reserved.
//
// Use of this source code is governed by a BSD-style license
// that can be found in the LICENSE file in the root of the source
// tree.

// Package pcaputil allows users to write PCAP data to files.
//
// We keep PCAP writing outside of the pcap/ package, since that package
// requires libpcap installation to be compiled, while this does not.
package pcaputil

import (
	"code.google.com/p/gopacket"
	"code.google.com/p/gopacket/layers"
	"encoding/binary"
	"fmt"
	"io"
)

// Writer wraps an underlying io.Writer to write packet data in PCAP
// format.  See http://wiki.wireshark.org/Development/LibpcapFileFormat
// for information on the file format.
//
// For those that care, we currently write v2.4 files with nanosecond
// timestamp resolution and little-endian encoding.
type Writer struct {
	w io.Writer
}

const magicNanoseconds = 0xA1B23C4D
const versionMajor = 2
const versionMinor = 4

// NewWriter returns a new writer object, for writing packet data out
// to the given writer.  If this is a new empty writer (as opposed to
// an append), you must call WriteFileHeader before WritePacket.
//
//  // Write a new file:
//  f, _ := os.Create("/tmp/file.pcap")
//  w := pcaputil.NewWriter(f)
//  w.WriteFileHeader(65536, layers.LinkTypeEthernet)  // new file, must do this.
//  w.WritePacket(gopacket.CaptureInfo{...}, data1)
//  f.Close()
//  // Append to existing file (must have same snaplen and linktype)
//  f2, _ := os.OpenFile("/tmp/file.pcap", os.O_APPEND, 0700)
//  w2 := pcaputil.NewWriter(f2)
//  // no need for file header, it's already written.
//  w2.WritePacket(gopacket.CaptureInfo{...}, data2)
//  f2.Close()
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// WriteFileHeader writes a file header out to the writer.
// This must be called exactly once per output.
func (w *Writer) WriteFileHeader(snaplen uint32, linktype layers.LinkType) error {
	var buf [24]byte
	binary.LittleEndian.PutUint32(buf[0:4], magicNanoseconds)
	binary.LittleEndian.PutUint16(buf[4:6], versionMajor)
	binary.LittleEndian.PutUint16(buf[6:8], versionMinor)
	// bytes 8:12 stay 0 (timezone = UTC)
	// bytes 12:16 stay 0 (sigfigs is always set to zero, according to
	//   http://wiki.wireshark.org/Development/LibpcapFileFormat
	binary.LittleEndian.PutUint32(buf[16:20], snaplen)
	binary.LittleEndian.PutUint32(buf[20:24], uint32(linktype))
	_, err := w.w.Write(buf[:])
	return err
}

const nanosPerSecond = 1000000000

func (w *Writer) writePacketHeader(ci gopacket.CaptureInfo) error {
	var buf [16]byte
	nanos := ci.Timestamp.UnixNano()
	secs, nsecs := uint32(nanos/nanosPerSecond), uint32(nanos%nanosPerSecond)
	binary.LittleEndian.PutUint32(buf[0:4], secs)
	binary.LittleEndian.PutUint32(buf[4:8], nsecs)
	binary.LittleEndian.PutUint32(buf[8:12], uint32(ci.CaptureLength))
	binary.LittleEndian.PutUint32(buf[12:16], uint32(ci.Length))
	_, err := w.w.Write(buf[:])
	return err
}

// WritePacket writes the given packet data out to the file.
func (w *Writer) WritePacket(ci gopacket.CaptureInfo, data []byte) error {
	if ci.CaptureLength != len(data) {
		return fmt.Errorf("capture length %d does not match data length %d", ci.CaptureLength, len(data))
	}
	if ci.CaptureLength > ci.Length {
		return fmt.Errorf("invalid capture info %+v:  capture length > length", ci)
	}
	if err := w.writePacketHeader(ci); err != nil {
		return fmt.Errorf("error writing packet header: %v", err)
	}
	_, err := w.w.Write(data)
	return err
}
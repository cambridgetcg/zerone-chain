//go:build ignore

package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	// Parse the existing rawDesc to compare
	existing := "\n" +
		"\x1fzerone/partnerships/v1/tx.proto\x12\x16zerone.partnerships.v1\x1a\x17cosmos/msg/v1/msg.proto\x1a\"zerone/partnerships/v1/types.proto\x1a$zerone/partnerships/v1/genesis.proto\"\xaa\x01\n" +
		"\x15MsgProposePartnership\x12\x1a\n" +
		"\bproposer\x18\x01 \x01(\tR\bproposer\x12\x18\n" +
		"\apartner\x18\x02 \x01(\tR\apartner\x12'\n" +
		"\x0finitial_deposit\x18\x03 \x01(\tR\x0einitialDeposit\x12#\n" +
		"\rproposed_tier\x18\x04 \x01(\rR\fproposedTier:\r\x82\xe7\xb0*\bproposer"

	var fd descriptorpb.FileDescriptorProto
	if err := proto.Unmarshal([]byte(existing), &fd); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	msg := fd.MessageType[0]
	fmt.Printf("Msg: %s\n", msg.GetName())
	fmt.Printf("  Options raw unknown: %x\n", msg.Options.ProtoReflect().GetUnknown())

	// Now re-serialize and check
	bz, _ := proto.Marshal(&fd)
	
	var fd2 descriptorpb.FileDescriptorProto
	proto.Unmarshal(bz, &fd2)
	msg2 := fd2.MessageType[0]
	fmt.Printf("After round-trip:\n")
	fmt.Printf("  Options raw unknown: %x\n", msg2.Options.ProtoReflect().GetUnknown())

	// Check if round-trip preserves the bytes
	if string(bz) == existing {
		fmt.Println("PERFECT MATCH!")
	} else {
		fmt.Printf("Round-trip differs (%d vs %d bytes)\n", len(bz), len(existing))
		// Show diffs
		for i := 0; i < len(bz) && i < len(existing); i++ {
			if bz[i] != existing[i] {
				fmt.Printf("  First diff at byte %d: got %02x want %02x\n", i, bz[i], existing[i])
				break
			}
		}
	}
}

//go:build ignore

package main

import (
	"fmt"
	"os"

	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

func main() {
	// Parse the existing rawDesc from tx.pb.go to see how it encodes the signer option
	existing := "" +
		"\n" +
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

	fmt.Printf("File: %s\n", fd.GetName())
	fmt.Printf("Messages: %d\n", len(fd.MessageType))
	for i, msg := range fd.MessageType {
		fmt.Printf("  %d: %s\n", i, msg.GetName())
		if msg.Options != nil {
			fmt.Printf("    Options: %v\n", msg.Options)
			// Check for unknown fields (extension data)
			raw := msg.Options.ProtoReflect().GetUnknown()
			if len(raw) > 0 {
				fmt.Printf("    Unknown fields (raw bytes): %x\n", raw)
			}
		}
	}
}

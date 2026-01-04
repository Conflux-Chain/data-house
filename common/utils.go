package common

import (
	"fmt"
	"strings"
)

func GetMethodID(input []byte) string {
	if len(input) < 4 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("0x")

	for _, b := range input[:4] {
		builder.WriteString(fmt.Sprintf("%02x", b))
	}

	return builder.String()
}

package testutil

import "strings"

func StripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			i += 2
			for i < len(s) && s[i] >= 0x20 && s[i] < 0x40 {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

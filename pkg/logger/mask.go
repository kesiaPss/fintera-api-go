package logger

import "strings"

// MaskEmail hides the local part of an address for logs, keeping the first
// character and the domain: "josej@example.com" becomes "j***@example.com".
// Inputs without an "@" are fully masked.
func MaskEmail(address string) string {
	address = strings.TrimSpace(address)
	at := strings.LastIndex(address, "@")
	if at <= 0 {
		if address == "" {
			return ""
		}
		return "***"
	}
	return address[:1] + "***" + address[at:]
}

// MaskEmails masks each address in a list, for logging recipient lists.
func MaskEmails(addresses []string) string {
	masked := make([]string, len(addresses))
	for i, address := range addresses {
		masked[i] = MaskEmail(address)
	}
	return strings.Join(masked, ", ")
}

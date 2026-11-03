package delivery_log

import "strings"

// dsnMeanings maps exact enhanced status codes (RFC 3463) to a plain-language reason.
var dsnMeanings = map[string]string{
	"5.1.0":  "The recipient address was rejected",
	"5.1.1":  "The recipient address does not exist (check the spelling)",
	"5.1.2":  "The recipient's domain does not exist",
	"5.1.10": "The recipient address does not exist (check the spelling)",
	"5.2.1":  "The recipient's mailbox is disabled",
	"5.2.2":  "The recipient's mailbox is full",
	"4.2.2":  "The recipient's mailbox is full, retrying later",
	"5.3.4":  "The message is too large for the recipient's server",
	"5.4.1":  "The recipient's server refused the message",
	"5.4.4":  "The mail server could not be found (DNS or network problem)",
	"4.4.1":  "The recipient's server did not answer, retrying later",
	"4.4.2":  "The connection to the recipient's server dropped, retrying later",
	"4.4.3":  "The recipient's server could not be found right now, retrying later",
	"5.7.1":  "The recipient's server blocked the message (spam or policy)",
	"5.7.23": "The sender failed SPF checks (fix the domain's SPF record)",
	"5.7.25": "The sending server has no reverse DNS (PTR) record",
	"5.7.26": "The message failed DMARC/DKIM/SPF authentication",
	"4.7.0":  "The recipient's server is temporarily limiting messages, retrying later",
	"4.7.1":  "The recipient's server temporarily refused the message, retrying later",
}

// ExplainDSN turns a delivery status code (e.g. "5.1.1") into a short, plain-language reason.
// Unknown codes fall back to a description of their class.
func ExplainDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if meaning, ok := dsnMeanings[dsn]; ok {
		return meaning
	}

	switch {
	case strings.HasPrefix(dsn, "2."):
		return "Delivered"
	case strings.HasPrefix(dsn, "4."):
		return "Temporary problem, the server will retry automatically"
	case strings.HasPrefix(dsn, "5."):
		return "Permanent failure, the message was not delivered"
	}
	return ""
}

package dtls

import (
	"regexp"
)

var pemSplit *regexp.Regexp = regexp.MustCompile(`(?sm)` +
	`(^-----[\s-]*?BEGIN.*?-----$` +
	`.*?` +
	`^-----[\s-]*?END.*?-----$)`)

func SplitPEM(data []byte) [][]byte {
	var results [][]byte
	for _, block := range pemSplit.FindAll(data, -1) {
		results = append(results, block)
	}
	return results
}

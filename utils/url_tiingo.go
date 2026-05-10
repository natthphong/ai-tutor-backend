package utils

import (
	"fmt"
)

func CallTiingoHistoryEod(url, ticker, startDate, token string) string {
	return fmt.Sprintf(url, ticker, startDate, token)
}
func CallTiingoHistory(url, ticker, startDate, freq, token string) string {
	return fmt.Sprintf(url, ticker, startDate, token, freq)
}

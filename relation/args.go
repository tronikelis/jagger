package relation

import (
	"strconv"
	"strings"
	"unicode"
)

func toIncrementedArgsQuery(query string, by int) (string, error) {
	counts := struct {
		quote  int
		quotes int
	}{}
	runes := []rune(query)

	builder := strings.Builder{}
	acc := strings.Builder{}

	for i := 0; i < len(runes); i++ {
		builder.WriteRune(runes[i])

		if runes[i] == '$' && counts.quote%2 == 0 && counts.quotes%2 == 0 {
			acc.WriteString(builder.String())
			builder.Reset()

			for i+1 < len(runes) && unicode.IsDigit(runes[i+1]) {
				i++
				builder.WriteRune(runes[i])
			}
			if builder.Len() == 0 {
				continue
			}

			num, err := strconv.Atoi(builder.String())
			if err != nil {
				return "", err
			}
			builder.Reset()

			num += by

			acc.WriteString(strconv.Itoa(num))
			continue
		}

		switch runes[i] {
		case '"':
			counts.quotes++
		case '\'':
			counts.quote++
		}
	}

	acc.WriteString(builder.String())

	return acc.String(), nil
}

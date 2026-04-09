package rules

import (
	"math"
	"strings"
	"unicode"
)

// ValidateIDCard validates a Chinese ID card number using GB 11643-1999 Mod11-2.
func ValidateIDCard(s string) bool {
	if len(s) != 18 {
		return false
	}
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkChars := []byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

	sum := 0
	for i := 0; i < 17; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
		sum += int(s[i]-'0') * weights[i]
	}
	remainder := sum % 11
	expected := checkChars[remainder]
	last := byte(unicode.ToUpper(rune(s[17])))
	return last == expected
}

// ValidateLuhn validates a number string using the Luhn algorithm (Mod-10).
func ValidateLuhn(s string) bool {
	// Strip non-digit characters
	clean := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)

	if len(clean) < 13 || len(clean) > 19 {
		return false
	}

	sum := 0
	nDigits := len(clean)
	parity := nDigits % 2
	for i := 0; i < nDigits; i++ {
		d := int(clean[i] - '0')
		if i%2 == parity {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
	}
	return sum%10 == 0
}

// ValidateIPv4 checks each octet is <= 255.
func ValidateIPv4(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, p := range parts {
		n := 0
		for _, c := range p {
			if c < '0' || c > '9' {
				return false
			}
			n = n*10 + int(c-'0')
		}
		if n > 255 {
			return false
		}
	}
	return true
}

// ValidateUSCC validates a Unified Social Credit Code using GB 32100-2015 Mod31.
func ValidateUSCC(s string) bool {
	if len(s) != 18 {
		return false
	}

	// Character to value mapping per GB 32100-2015
	charMap := map[byte]int{
		'0': 0, '1': 1, '2': 2, '3': 3, '4': 4, '5': 5, '6': 6, '7': 7, '8': 8, '9': 9,
		'A': 10, 'B': 11, 'C': 12, 'D': 13, 'E': 14, 'F': 15, 'G': 16, 'H': 17,
		'J': 18, 'K': 19, 'L': 20, 'M': 21, 'N': 22, 'P': 23, 'Q': 24, 'R': 25,
		'T': 26, 'U': 27, 'W': 28, 'X': 29, 'Y': 30,
	}

	weights := []int{1, 3, 9, 27, 19, 81, 57, 9, 27, 19, 81, 57, 9, 27, 19, 81, 57}

	sum := 0
	for i := 0; i < 17; i++ {
		v, ok := charMap[s[i]]
		if !ok {
			return false
		}
		sum += v * weights[i]
	}

	remainder := sum % 31
	var checkVal int
	if remainder == 0 {
		checkVal = 0
	} else {
		checkVal = 31 - remainder
	}

	// Map checkVal back to character
	checkChars := "0123456789ABCDEFGHJKLMNPQRTUWXY"
	if checkVal >= len(checkChars) {
		return false
	}

	return s[17] == checkChars[checkVal]
}

// ValidateHighEntropy checks if a string has Shannon entropy > 4.5.
func ValidateHighEntropy(s string) bool {
	return shannonEntropy(s) > 4.5
}

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}
	var entropy float64
	length := float64(len(s))
	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}
	return entropy
}

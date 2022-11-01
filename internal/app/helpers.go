package app

// Valid check number is valid or not based on Luhn algorithm

func ValidateLuhn(number int) bool {

	checksum := func(number int) int {
		var luhn int

		for i := 0; number > 0; i++ {
			cur := number % 10

			if i%2 == 0 { // even
				cur = cur * 2
				if cur > 9 {
					cur = cur%10 + cur/10
				}
			}
			luhn += cur
			number = number / 10
		}
		return luhn % 10
	}
	return (number%10+checksum(number/10))%10 == 0
}

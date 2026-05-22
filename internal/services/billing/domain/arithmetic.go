package domain

import (
	"fmt"
	"math/big"
)

// SubtractAmount вычитает b из a, возвращая результат с фиксированной точностью 6 знаков.
// Используется везде, где требуется единообразная арифметика для NUMERIC(20,6) значений.
// Формат вывода совпадает с BillingService.subtract (big.Float.Text('f', 6)).
func SubtractAmount(a, b string) (string, error) {
	aBig := new(big.Float)
	bBig := new(big.Float)

	if _, ok := aBig.SetString(a); !ok {
		return "", fmt.Errorf("invalid number: %s", a)
	}
	if _, ok := bBig.SetString(b); !ok {
		return "", fmt.Errorf("invalid number: %s", b)
	}

	result := new(big.Float).Sub(aBig, bBig)
	return result.Text('f', 6), nil
}

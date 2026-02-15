package common

import (
	"fmt"
)

func Must(err error) {
	if err != nil {
		fmt.Errorf("\nerr %w", err)
		panic(err)
	}
}
